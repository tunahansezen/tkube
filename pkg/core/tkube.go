package core

import (
	cfg "com.github.tunahansezen/tkube/pkg/config"
	"com.github.tunahansezen/tkube/pkg/config/model"
	"com.github.tunahansezen/tkube/pkg/config/templates"
	conn "com.github.tunahansezen/tkube/pkg/connection"
	"com.github.tunahansezen/tkube/pkg/constant"
	"com.github.tunahansezen/tkube/pkg/kube"
	"com.github.tunahansezen/tkube/pkg/os"
	"com.github.tunahansezen/tkube/pkg/path"
	"com.github.tunahansezen/tkube/pkg/util"
	"embed"
	"fmt"
	"github.com/cloudflare/cfssl/csr"
	"github.com/cloudflare/cfssl/helpers"
	cfssl "github.com/cloudflare/cfssl/initca"
	"github.com/cloudflare/cfssl/signer"
	signerl "github.com/cloudflare/cfssl/signer/local"
	"github.com/guumaster/logsymbols"
	"github.com/hashicorp/go-version"
	log "github.com/sirupsen/logrus"
	"net"
	"slices"
	"strings"
	"time"
)

const (
	DefaultDockerVersion     = "20.10.24"
	DefaultContainerdVersion = "auto"
	DefaultEtcdVersion       = "3.5.14"
	DefaultKubeVersion       = "1.30.1"
	DefaultCalicoVersion     = "auto"
	DefaultHelmVersion       = "3.15.1"
	DefaultDockerPrune       = false
)

var (
	//go:embed resources
	f                     embed.FS
	AuthMapStr            string // node1IP:node1SshUser:node1SshPass,node2IP:node2SshUser:node2SshPass...
	DockerVersion         string
	ContainerdVersion     string
	EtcdVersion           string
	KubeVersion           string
	CalicoVersion         string
	HelmVersion           string
	IsoPath               string
	etcdCompressedFile    string
	etcdUrl               string
	helmCompressedFile    string
	helmUrl               string
	DockerPrune           bool
	multiMasterDeployment bool
)

func Install(nodes model.KubeNodes) {
	if nodes.IncludeMaster() {
		switch len(nodes.GetMasterKubeNodes()) {
		case 1:
			fmt.Println("Single-master deployment started")
			multiMasterDeployment = false
		default:
			fmt.Println("Multi-master deployment started")
			multiMasterDeployment = true
		}
	}
	handleAuthMap()
	addToEtcHosts(cfg.DeploymentCfg.GetKubeNodes())
	if IsoPath != "" {
		var firstMasterNode model.KubeNode
		for i, node := range nodes.Nodes {
			if i == 0 {
				firstMasterNode = node
				os.UmountISO(constant.IsoMountDir, node.IP)
				os.MountISO(constant.IsoMountDir, IsoPath, node.IP)
				var repoAddress string
				if os.InstallerType == os.Apt {
					repoAddress = fmt.Sprintf("file://%s/repo ./", constant.IsoMountDir)
				} else if os.InstallerType == os.Yum || os.InstallerType == os.Dnf {
					repoAddress = fmt.Sprintf("file://%s/repo", constant.IsoMountDir)
				}
				os.AddRepository("tkube", "tkube", "tkube", repoAddress, "", node.IP)
				os.UpdateRepos(node.IP)
				os.InstallPackage("sshpass", node.IP)
				continue
			}
			isoFile := IsoPath[strings.LastIndex(IsoPath, "/")+1:]
			if !os.IsFileExistsOn(os.GetMd5On(IsoPath, firstMasterNode.IP), IsoPath, node.IP) {
				util.StartSpinner(fmt.Sprintf("Transferring \"%s\" file to \"%s\"", isoFile, node.IP))
				err := os.TransferFile(IsoPath, IsoPath, firstMasterNode.IP, node.IP)
				if err != nil {
					os.Exit(err.Error(), 1)
				}
				util.StopSpinner("", logsymbols.Success)
			} else {
				fmt.Printf("\"%s\" file exist on \"%s\"\n", isoFile, node.IP.String())
			}
			os.UmountISO(constant.IsoMountDir, node.IP)
			os.MountISO(constant.IsoMountDir, IsoPath, node.IP)
			var repoAddress string
			if os.InstallerType == os.Apt {
				repoAddress = fmt.Sprintf("file://%s/repo ./", constant.IsoMountDir)
			} else if os.InstallerType == os.Yum || os.InstallerType == os.Dnf {
				repoAddress = fmt.Sprintf("file://%s/repo", constant.IsoMountDir)
			}
			os.AddRepository("tkube", "tkube", "tkube", repoAddress, "", node.IP)
			os.UpdateRepos(node.IP)
		}
	}
	addCustomRepos(nodes)
	installPackages(nodes)
	kubeInstReqMap := removeKubePackagesIfNecessary(nodes)
	prepareNodes(nodes)
	installDocker(nodes)
	installContainerd(nodes)
	installKubePackages(nodes, kubeInstReqMap)
	if multiMasterDeployment {
		generateAndDistributeKubeAndEtcdCerts(nodes)
		installEtcd(nodes)
	} else {
		for _, kubeNode := range nodes.GetMasterKubeNodes() {
			os.RunCommandOn("sudo service etcd stop || true", kubeNode.IP, true)
			os.RunCommandOn("sudo rm -rf /var/lib/etcd", kubeNode.IP, true)
		}
	}
	installHelm(nodes)
	if cfg.DeploymentCfg.Keepalived.Enabled {
		installKeepAliveD(nodes)
	}
	if IsoPath != "" {
		kubeSemVer, _ := version.NewVersion(KubeVersion)
		kube124Ver, _ := version.NewVersion("1.24")
		for _, node := range nodes.Nodes {
			util.StartSpinner(fmt.Sprintf("Loading images on \"%s\"", node.Hostname))
			if kubeSemVer.GreaterThanOrEqual(kube124Ver) {
				os.RunCommandOn(fmt.Sprintf("ls -1 %s/kubernetes/images/*.tar | "+
					"xargs --no-run-if-empty -L 1 sudo ctr -n=k8s.io images import", constant.IsoMountDir),
					node.IP, true)
				os.RunCommandOn(fmt.Sprintf("ls -1 %s/calico/images/*.tar | "+
					"xargs --no-run-if-empty -L 1 sudo ctr -n=k8s.io images import", constant.IsoMountDir),
					node.IP, true)
			} else {
				os.RunCommandOn(fmt.Sprintf("ls -1 %s/kubernetes/images/*.tar | "+
					"xargs --no-run-if-empty -L 1 sudo docker load -i", constant.IsoMountDir),
					node.IP, true)
				os.RunCommandOn(fmt.Sprintf("ls -1 %s/calico/images/*.tar | "+
					"xargs --no-run-if-empty -L 1 sudo docker load -i", constant.IsoMountDir),
					node.IP, true)
			}
			util.StopSpinner("", logsymbols.Success)
		}
	}
	initKubernetes(nodes)
}

func handleAuthMap() {
	if AuthMapStr == "" {
		return
	}
	for _, authInfo := range strings.Split(AuthMapStr, ",") {
		authInfoSlice := strings.Split(authInfo, ":")
		if len(authInfoSlice) != 3 {
			os.Exit("Auth data invalid", 1)
		} else {
			err := conn.WriteSSHData(authInfoSlice[0], authInfoSlice[1], authInfoSlice[2])
			if err != nil {
				os.Exit(err.Error(), 1)
			}
		}
	}
}

func addToEtcHosts(nodes []model.KubeNode) {
	for _, kubeNode := range nodes {
		for _, h := range nodes {
			os.AppendLineOn(fmt.Sprintf("%s %s", h.IP, h.Hostname), "/etc/hosts", true, kubeNode.IP)
		}
	}
}

func prepareNodes(nodes model.KubeNodes) {
	for _, kubeNode := range nodes.Nodes {
		os.RunCommandOn("sudo sysctl fs.inotify.max_user_watches=1048576", kubeNode.IP, true)
		os.RunCommandOn("sudo swapoff -a", kubeNode.IP, true)
		//os.RunCommandOn("sudo mkdir -p /sys/fs/cgroup/cpu,cpuacct", kubeNode.IP, true)
		//os.RunCommandOn("sudo mount -t cgroup -o cpu,cpuacct none /sys/fs/cgroup/cpu,cpuacct || true", kubeNode.IP, true)
		//os.RunCommandOn("sudo mkdir -p /sys/fs/cgroup/systemd", kubeNode.IP, true)
		//os.RunCommandOn("sudo mount -t cgroup -o none,name=systemd cgroup /sys/fs/cgroup/systemd || true", kubeNode.IP, true)
		if (os.OS == os.CentOS || os.OS == os.Rocky) && cfg.DeploymentCfg.CentOS.SetSelinuxPermissive {
			if os.IsSelinuxEnabled(kubeNode.IP) {
				util.StartSpinner("Setting SELinux to permissive mode")
				os.RunCommandOn("sudo setenforce 0", kubeNode.IP, true)
				os.RunCommandOn("sudo sed -i 's/^SELINUX=enforcing$/SELINUX=permissive/' /etc/selinux/config", kubeNode.IP, true)
				util.StopSpinner("", logsymbols.Success)
			} else {
				fmt.Printf("SELinux already disabled on \"%s\"\n", kubeNode.IP.String())
			}
		}
	}
}

func addCustomRepos(nodes model.KubeNodes) {
	if IsoPath != "" {
		log.Debugf("Skipping adding custom repo, because iso repo defined.")
		return
	}
	for _, kubeNode := range nodes.Nodes {
		for _, repo := range cfg.DeploymentCfg.CustomRepos {
			if !repo.Enabled {
				continue
			}
			util.StartSpinner(fmt.Sprintf("Adding custom repo on \"%s\"", kubeNode.IP))
			repoFields := strings.Fields(repo.Address)
			repoUrl := ""
			for _, field := range repoFields {
				if strings.HasPrefix(field, "http") {
					repoUrl = field
				}
			}
			if repoUrl != "" {
				if !conn.IsReachableURL(repoUrl) {
					os.Exit(fmt.Sprintf("Custom repo \"%s\" is not reachable!", repoUrl), 1)
				}
			}
			keyPath := os.AddGpgKey(repo.Key, repo.ShortName(), kubeNode.IP)
			os.AddRepository(repo.Name, repo.ShortName(), repo.ShortName(), repo.Address, keyPath, kubeNode.IP)
			util.StopSpinner("", logsymbols.Success)
		}
		os.RemoveRelatedRepoFiles("docker", kubeNode.IP)
		os.RemoveRelatedRepoFiles("kubernetes", kubeNode.IP)
		os.UpdateRepos(kubeNode.IP)
	}
}

func installPackages(nodes model.KubeNodes) {
	for _, packageName := range cfg.DeploymentCfg.Packages {
		for _, node := range nodes.Nodes {
			os.InstallPackage(packageName, node.IP)
		}
	}
}

func installDocker(nodes model.KubeNodes) {
	kubeSemVer, _ := version.NewVersion(KubeVersion)
	kube124Ver, _ := version.NewVersion("1.24")
	if kubeSemVer.GreaterThanOrEqual(kube124Ver) && !cfg.DeploymentCfg.Docker.Enabled {
		return
	}
	repo := cfg.DeploymentCfg.Docker.Repo
	for _, kubeNode := range nodes.Nodes {
		isoPathDefined := IsoPath != ""
		if isoPathDefined {
			log.Debugf("Skipping to add docker repo on %s. Because iso repo defined.", kubeNode.IP.String())
		}
		if repo.Enabled && !isoPathDefined {
			util.StartSpinner("Adding docker repo")
			keyPath := os.AddGpgKey(repo.Key, repo.ShortName(), kubeNode.IP)
			os.AddRepository(repo.Name, repo.ShortName(), repo.ShortName(), repo.Address, keyPath, kubeNode.IP)
			util.StopSpinner("", logsymbols.Success)
			os.UpdateRepos(kubeNode.IP)
		}
		isDockerInstalled, installedDockerVer := os.PackageInstalledOn("docker-ce", kubeNode.IP)
		isDockerCliInstalled, installedDockerCliVer := os.PackageInstalledOn("docker-ce-cli", kubeNode.IP)
		installationNeeded := false
		differentDockerVer := false
		if (isDockerInstalled && !strings.Contains(installedDockerVer, DockerVersion)) ||
			(isDockerCliInstalled && !strings.Contains(installedDockerCliVer, DockerVersion)) {
			differentDockerVer = true
			util.PrintWarning(fmt.Sprintf("Different version (\"%s\") docker packages found on \"%s\". "+
				"Desired version: \"%s\"", installedDockerVer, kubeNode.Hostname, DockerVersion))
		}
		if DockerPrune || differentDockerVer {
			os.RemovePackage("docker-ce-cli", kubeNode.IP)
			os.RemovePackage("docker-ce", kubeNode.IP)
			// unofficial docker packages
			os.RemovePackage("docker.io", kubeNode.IP)
			os.RemovePackage("docker-doc", kubeNode.IP)
			os.RemovePackage("docker-compose", kubeNode.IP)
			os.RemovePackage("podman-docker", kubeNode.IP)
			// already bundled packages
			os.RemovePackage("containerd", kubeNode.IP)
			os.RemovePackage("containerd.io", kubeNode.IP)
			os.RemovePackage("runc", kubeNode.IP)
			// extras
			os.RemovePackage("docker-ce-rootless-extras", kubeNode.IP)
			os.RemovePackage("docker-buildx-plugin", kubeNode.IP)
			os.RemovePackage("docker-compose-plugin", kubeNode.IP)
			os.RemovePackage("docker-scan-plugin", kubeNode.IP)
			installationNeeded = true
		} else {
			dockerRunning := os.RunCommandOn("systemctl show --property ActiveState docker | cut -d= -f2 | xargs",
				kubeNode.IP, true)
			if dockerRunning != "active" {
				installationNeeded = true
			} else {
				os.RunCommandOn("sudo service docker stop", kubeNode.IP, true)
				createDockerDaemonCfgOn(kubeNode.IP)
				os.ChangeServiceStatus("docker", "restart", 10, kubeNode.IP)
			}
		}
		if installationNeeded || !isDockerInstalled || !isDockerCliInstalled {
			if DockerPrune {
				util.StartSpinner(fmt.Sprintf("Removing \"/var/lib/docker\" folder on \"%s\"", kubeNode.Hostname))
				os.RunCommandOn("sudo rm -rf /var/lib/docker", kubeNode.IP, true)
				util.StopSpinner("", logsymbols.Success)
				util.StartSpinner(fmt.Sprintf("Removing \"/var/lib/containerd\" folder on \"%s\"", kubeNode.Hostname))
				os.RunCommandOn("sudo rm -rf /var/lib/containerd", kubeNode.IP, true)
				util.StopSpinner("", logsymbols.Success)
			}
			os.RunCommandOn("sudo rm -rf /etc/docker/*", kubeNode.IP, true)
			// todo install from packages https://docs.docker.com/engine/install/ubuntu/
			dockerPackages := fmt.Sprintf("docker-ce=%s docker-ce-cli=%s", DockerVersion, DockerVersion)
			if ContainerdVersion != DefaultContainerdVersion {
				// fixme downgrade check
				dockerPackages = fmt.Sprintf("containerd.io=%s %s", ContainerdVersion, dockerPackages)
			}
			os.InstallPackageNoStart(dockerPackages, kubeNode.IP)
			createDockerDaemonCfgOn(kubeNode.IP)
			os.ChangeServiceStatus("docker", "restart", 10, kubeNode.IP)
		} else {
			fmt.Printf("Required docker packages with version \"%s\" already installed on \"%s\"\n",
				DockerVersion, kubeNode.Hostname)
		}
		os.RunCommandOn("sudo groupadd docker -f", kubeNode.IP, true)
		os.RunCommandOn("sudo gpasswd -a $USER docker", kubeNode.IP, true)
		os.RunCommandOn("newgrp docker", kubeNode.IP, true)
		os.RunCommandOn("sudo chown $(id -u):$(id -g) $HOME/.docker/config.json || true", kubeNode.IP, true)
	}
}
func installContainerd(nodes model.KubeNodes) {
	for _, kubeNode := range nodes.Nodes {
		os.InstallPackage("containerd.io", kubeNode.IP)
		kubeSemVer, _ := version.NewVersion(KubeVersion)
		kube124Ver, _ := version.NewVersion("1.24")
		if kubeSemVer.GreaterThanOrEqual(kube124Ver) {
			os.RunCommandOn("sudo rm -rf /etc/containerd_bak", kubeNode.IP, true)
			os.RunCommandOn("sudo mkdir -p /etc/containerd && sudo mv /etc/containerd /etc/containerd_bak", kubeNode.IP, true)
			os.RunCommandOn("sudo mkdir -p /etc/containerd", kubeNode.IP, true)
			os.RunCommandOn("sudo containerd config default | sudo tee /etc/containerd/config.toml",
				kubeNode.IP, true)
			os.RunCommandOn("sudo sed -i 's/            SystemdCgroup = false/            SystemdCgroup = true/' "+
				"/etc/containerd/config.toml",
				kubeNode.IP, true)
			os.RunCommandOn(fmt.Sprintf("sudo sed -i 's/    sandbox_image = .*/    sandbox_image = \"%s\\/%s\"/g' "+
				"/etc/containerd/config.toml", cfg.DeploymentCfg.Kubernetes.ImageRegistry,
				cfg.DeploymentCfg.Containerd.Cri.SandboxImage), kubeNode.IP, true)
			if len(cfg.DeploymentCfg.Docker.Daemon.InsecureRegistries) > 0 {
				os.RunCommandOn("sudo sed -i 's/config_path = \"\"/config_path = \"\\/etc\\/containerd\\/certs.d\"/g' "+
					"/etc/containerd/config.toml", kubeNode.IP, true)
			}
			for _, inReg := range cfg.DeploymentCfg.Docker.Daemon.InsecureRegistries {
				dir := fmt.Sprintf("/etc/containerd/certs.d/%s", inReg)
				os.RunCommandOn(fmt.Sprintf("sudo mkdir -p %s", dir), kubeNode.IP, true)
				os.RunCommandOn(fmt.Sprintf("echo '[host.\"http://%s\"]' | "+
					"sudo tee %s/hosts.toml", inReg, dir), kubeNode.IP, true)
				os.RunCommandOn(fmt.Sprintf("echo '  capabilities = [\"pull\", \"resolve\", \"push\"]' | "+
					"sudo tee -a %s/hosts.toml", dir), kubeNode.IP, true)
				os.RunCommandOn(fmt.Sprintf("echo '  skip_verify = true' | "+
					"sudo tee -a %s/hosts.toml", dir), kubeNode.IP, true)
			}
			os.RunCommandOn("sudo systemctl restart containerd", kubeNode.IP, true)
			time.Sleep(5 * time.Second)
		}
	}
}

func createDockerDaemonCfgOn(ip net.IP) {
	os.CreateFile(cfg.DeploymentCfg.Docker.Daemon.MarshallJson(),
		fmt.Sprintf("%s/daemon.json", path.GetTKubeTmpDir(ip)), ip)
	os.RunCommandOn(fmt.Sprintf("sudo mkdir -p /etc/docker && sudo mv %s/daemon.json /etc/docker/",
		path.GetTKubeTmpDir(ip)), ip, true)
}

func removeKubePackagesIfNecessary(nodes model.KubeNodes) map[string]bool {
	repo := cfg.DeploymentCfg.Kubernetes.Repo
	installationRequired := make(map[string]bool)
	for _, kubeNode := range nodes.Nodes {
		isoPathDefined := IsoPath != ""
		if isoPathDefined {
			log.Debugf("Skipping to add kube repo on %s. Because iso repo defined.", kubeNode.IP.String())
		}
		if repo.Enabled && !isoPathDefined {
			util.StartSpinner("Adding kubernetes repo")
			var repoName string
			if strings.Contains(repo.Address, "{version}") {
				repoName = fmt.Sprintf("%s-%s", repo.ShortName(), util.GetMajorVersion(KubeVersion))
			} else {
				repoName = repo.ShortName()
			}
			keyPath := os.AddGpgKey(strings.ReplaceAll(repo.Key, "{version}", util.GetMajorVersion(KubeVersion)),
				repoName, kubeNode.IP)
			os.AddRepository(repo.Name, repo.ShortName(), repoName,
				strings.ReplaceAll(repo.Address, "{version}", util.GetMajorVersion(KubeVersion)), keyPath, kubeNode.IP)
			util.StopSpinner("", logsymbols.Success)
			os.UpdateRepos(kubeNode.IP)
		}
		isKubeletInstalled, installedKubeletVer := os.PackageInstalledOn("kubelet", kubeNode.IP)
		isKubectlInstalled, installedKubectlVer := os.PackageInstalledOn("kubectl", kubeNode.IP)
		isKubeadmInstalled, installedKubeadmVer := os.PackageInstalledOn("kubeadm", kubeNode.IP)
		if isKubeadmInstalled {
			util.StartSpinner(fmt.Sprintf("Resetting kubernetes on \"%s\"", kubeNode.Hostname))
			os.RunCommandOn("sudo kubeadm reset -f", kubeNode.IP, true)
			util.StopSpinner("", logsymbols.Success)
		}
		os.RunCommandOn("sudo rm -rf /etc/kubernetes", kubeNode.IP, true)
		os.RunCommandOn("sudo rm -rf /etc/cni/net.d", kubeNode.IP, true)
		os.RunCommandOn("sudo rm -rf /var/lib/cni", kubeNode.IP, true)
		// todo check ipvsadm exist, run "ipvsadm --clear"
		os.RunCommandOn("sudo rm -rf $HOME/.kube", kubeNode.IP, true)
		if isKubeletInstalled {
			os.RunCommandOn("sudo service kubelet stop || true", kubeNode.IP, true)
		}
		os.RunCommandOn("sudo rm -rf /var/lib/kubelet", kubeNode.IP, true)
		removeKubePackages := false
		if (isKubeletInstalled && !strings.HasPrefix(installedKubeletVer, KubeVersion)) ||
			(isKubectlInstalled && !strings.HasPrefix(installedKubectlVer, KubeVersion)) ||
			(isKubeadmInstalled && !strings.HasPrefix(installedKubeadmVer, KubeVersion)) {
			util.PrintWarning(fmt.Sprintf("Different version (\"%s\") kube packages found on \"%s\". "+
				"Desired version: \"%s\"", installedKubeadmVer, kubeNode.Hostname, KubeVersion))
			os.RemovePackage("kubeadm", kubeNode.IP)
			os.RemovePackage("kubectl", kubeNode.IP)
			os.RemovePackage("kubelet", kubeNode.IP)
			removeKubePackages = true
		}
		if removeKubePackages || !isKubeletInstalled || !isKubectlInstalled || !isKubeadmInstalled {
			installationRequired[kubeNode.IP.String()] = true
		} else {
			installationRequired[kubeNode.IP.String()] = false
		}
	}
	return installationRequired
}

func installKubePackages(nodes model.KubeNodes, installationRequired map[string]bool) {
	for _, kubeNode := range nodes.Nodes {
		if installationRequired[kubeNode.IP.String()] {
			os.InstallPackage(fmt.Sprintf("kubelet=%s kubectl=%s kubeadm=%s", KubeVersion, KubeVersion, KubeVersion),
				kubeNode.IP)
			for _, pkg := range []string{"kubectl", "kubelet", "kubeadm"} {
				os.LockPackageVersion(pkg, kubeNode.IP)
			}
			if cfg.DeploymentCfg.Kubernetes.BashCompletion {
				os.RunCommandOn("kubectl completion bash | sudo tee /etc/bash_completion.d/kubectl > /dev/null",
					kubeNode.IP, true)
				os.RunCommandOn("sudo chmod a+r /etc/bash_completion.d/kubectl",
					kubeNode.IP, true)
			}
		} else {
			fmt.Printf("Required kubernetes packages with version \"%s-00\" already installed on \"%s\"\n",
				KubeVersion, kubeNode.Hostname)
		}
	}
}

func generateAndDistributeKubeAndEtcdCerts(nodes model.KubeNodes) {
	if !nodes.IncludeMaster() {
		return
	}
	util.StartSpinner("Generating kube and etcd certs")
	caCert, _, caKey, err := cfssl.New(model.DefaultKubernetesCSR())
	if err != nil {
		os.Exit(err.Error(), 1)
	}
	etcdCert, _, etcdKey, err := createEtcdCerts(caCert, caKey)
	if err != nil {
		os.Exit(err.Error(), 1)
	}

	// config sh
	for _, kubeNode := range nodes.GetMasterKubeNodes() {
		os.RunCommandOn(fmt.Sprintf("sudo mkdir -p %s", constant.EtcdPkiFolder), kubeNode.IP, true)
		os.CreateFile(caCert, constant.EtcdCaCertPath, kubeNode.IP)
		os.CreateFile(caKey, constant.EtcdCaKeyPath, kubeNode.IP)
		os.CreateFile(etcdKey, constant.EtcdClientKeyPath, kubeNode.IP)
		os.CreateFile(etcdCert, constant.EtcdClientCertPath, kubeNode.IP)
	}
	util.StopSpinner("", logsymbols.Success)
}

func Validator(req *csr.CertificateRequest) error {
	return nil
}

func createEtcdCerts(caCert []byte, caKey []byte) (cert, csrPEM, key []byte, err error) {
	etcdCsr := model.DefaultEtcdCSR(cfg.DeploymentCfg.GetMasterKubeNodeIPs())
	var csrBytes []byte
	g := &csr.Generator{Validator: Validator}
	csrBytes, key, err = g.ProcessRequest(etcdCsr)
	if err != nil {
		return nil, nil, nil, err
	}
	parsedCa, err := helpers.ParseCertificatePEM(caCert)
	if err != nil {
		return nil, nil, nil, err
	}
	parsedPK, err := helpers.ParsePrivateKeyPEM(caKey)
	if err != nil {
		return nil, nil, nil, err
	}
	s, err := signerl.NewSigner(parsedPK, parsedCa, signer.DefaultSigAlgo(parsedPK), model.DefaultKubernetesCA())
	if err != nil {
		return nil, nil, nil, err
	}
	signReq := signer.SignRequest{
		Request: string(csrBytes),
		Profile: "kubernetes",
	}
	cert, err = s.Sign(signReq)
	if err != nil {
		return nil, nil, nil, err
	}
	return cert, csrBytes, key, err
}

func installEtcd(nodes model.KubeNodes) {
	if !nodes.IncludeMaster() {
		return
	}
	// download and distribute compressed etcd file
	var err error
	etcdUrl = cfg.DeploymentCfg.GetEtcdExactUrl(EtcdVersion)
	etcdCompressedFile = etcdUrl[strings.LastIndex(etcdUrl, "/")+1:]
	var filePath string
	var firstNode model.KubeNode
	var etcdFileExists bool
	var skipInstallEtcd []string
	for i, kubeNode := range nodes.GetMasterKubeNodes() {
		etcdExists := os.CommandExists("etcd")
		etcdCtlExists := os.CommandExists("etcdctl")
		if etcdExists && etcdCtlExists {
			installedEtcdVersion := os.RunCommand("etcd --version | head -1 | cut -d: -f2 | xargs", true)
			if installedEtcdVersion == EtcdVersion {
				skipInstallEtcd = append(skipInstallEtcd, kubeNode.IP.String())
				fmt.Printf("etcd with \"%s\" version already installed on \"%s\"\n", EtcdVersion, kubeNode.Hostname)
				continue
			}
		}
		if err != nil {
			os.Exit(err.Error(), 1)
		}
		if i == 0 {
			firstNode = kubeNode
			etcdFolder := path.GetTKubeTmpDir(kubeNode.IP)
			if IsoPath != "" {
				etcdFolder = fmt.Sprintf("%s/etcd", constant.IsoMountDir)
			}
			filePath = fmt.Sprintf("%s/%s", etcdFolder, etcdCompressedFile)
			etcdFileExists = os.IsFileExistsOn("", filePath, firstNode.IP)
		}
		if etcdFileExists {
			util.StartSpinner(fmt.Sprintf("Transferring \"%s\" for %s to %s",
				etcdCompressedFile, firstNode.IP, kubeNode.IP))
			err = os.TransferFile(filePath, fmt.Sprintf("%s/%s", path.GetTKubeTmpDir(kubeNode.IP), etcdCompressedFile),
				firstNode.IP, kubeNode.IP)
			if err != nil {
				os.Exit(err.Error(), 1)
			}
			util.StopSpinner("", logsymbols.Success)
		} else {
			util.StartSpinner(fmt.Sprintf("Downloading \"%s\" to %s", etcdCompressedFile, kubeNode.IP))
			os.RunCommandOn(fmt.Sprintf("wget -nc -P %s %s", path.GetTKubeTmpDir(kubeNode.IP), etcdUrl), kubeNode.IP,
				true)
			if err != nil {
				os.Exit(err.Error(), 1)
			}
			if i == 0 {
				os.RunCommandOn(fmt.Sprintf("sudo cp %s/%s %s/", path.GetTKubeTmpDir(kubeNode.IP), etcdCompressedFile,
					path.GetTKubeResourcesDir()), kubeNode.IP, true)
				if err != nil {
					os.Exit(err.Error(), 1)
				}
			}
			util.StopSpinner("", logsymbols.Success)
		}
	}

	// install etcd
	extractedEtcdFolder := strings.ReplaceAll(etcdCompressedFile, ".tar.gz", "")
	for _, kubeNode := range nodes.GetMasterKubeNodes() {
		os.AppendLineOn("ETCDCTL_API=3", "/etc/environment", true, kubeNode.IP)
		os.RunCommandOn("sudo service etcd stop || true", kubeNode.IP, true)
		if !slices.Contains(skipInstallEtcd, kubeNode.IP.String()) {
			prevEtcdPath := os.RunCommandOn("which etcd || true", kubeNode.IP, true)
			if prevEtcdPath != "" {
				os.RunCommandOn(fmt.Sprintf("sudo rm -rf %s", prevEtcdPath), kubeNode.IP, true)
			}
			prevEtcdCtlPath := os.RunCommandOn("which etcdctl || true", kubeNode.IP, true)
			if prevEtcdCtlPath != "" {
				os.RunCommandOn(fmt.Sprintf("sudo rm -rf %s", prevEtcdCtlPath), kubeNode.IP, true)
			}
		}
		os.RunCommandOn("sudo rm -rf /var/lib/etcd", kubeNode.IP, true)
	}
	for _, kubeNode := range nodes.GetMasterKubeNodes() {
		if slices.Contains(skipInstallEtcd, kubeNode.IP.String()) {
			continue
		}
		util.StartSpinner(fmt.Sprintf("Installing etcd-%s on \"%s\"", EtcdVersion, kubeNode.Hostname))
		tmpDir := path.GetTKubeTmpDir(kubeNode.IP)
		os.RunCommandOn(fmt.Sprintf("tar -zxvf %s/%s -C %s", tmpDir, etcdCompressedFile, tmpDir), kubeNode.IP, true)
		os.RunCommandOn(fmt.Sprintf("sudo cp %s/%s/etcd* /usr/bin/", tmpDir, extractedEtcdFolder),
			kubeNode.IP, true)
		os.RunCommandOn("sudo chmod +x /usr/bin/etcd*", kubeNode.IP, true)
		util.StopSpinner("", logsymbols.Success)
	}

	// start etcd
	var etcdSvcCfgBytes []byte
	etcdSvcCfgBytes, err = f.ReadFile("resources/etcd.service")
	if err != nil {
		os.Exit(err.Error(), 1)
	}
	var clusterAddresses []string
	for _, k := range nodes.GetMasterKubeNodes() {
		clusterAddresses = append(clusterAddresses, fmt.Sprintf("%s=https://%s:2380", k.Hostname, k.IP))
	}
	for _, kubeNode := range nodes.GetMasterKubeNodes() {
		util.StartSpinner(fmt.Sprintf("Starting etcd service on \"%s\"", kubeNode.Hostname))
		os.RunCommandOn("sudo mkdir -p /var/lib/etcd", kubeNode.IP, true)
		etcdSvcCfg := string(etcdSvcCfgBytes)
		etcdSvcCfg = strings.ReplaceAll(etcdSvcCfg, "${HOST_IP}", kubeNode.IP.String())
		etcdSvcCfg = strings.ReplaceAll(etcdSvcCfg, "${HOSTNAME}", kubeNode.Hostname)
		etcdSvcCfg = strings.ReplaceAll(etcdSvcCfg, "\\\n", "")
		etcdSvcCfg = strings.ReplaceAll(etcdSvcCfg, "${CLUSTER_ADDRESSES}", strings.Join(clusterAddresses, ","))
		os.CreateFile([]byte(etcdSvcCfg), fmt.Sprintf("%s/etcd.service", path.GetTKubeTmpDir(kubeNode.IP)), kubeNode.IP)
		os.RunCommandOn(fmt.Sprintf("sudo mv %s/etcd.service /etc/systemd/system", path.GetTKubeTmpDir(kubeNode.IP)),
			kubeNode.IP, true)
		os.RunCommandOn("sudo systemctl daemon-reload", kubeNode.IP, true)
		os.RunCommandOn("sudo systemctl enable etcd", kubeNode.IP, true)
		os.RunCommandOn("sudo systemctl start --no-block etcd", kubeNode.IP, true)
		util.StopSpinner("", logsymbols.Success)
	}

	// check etcd running well
	util.StartSpinner("Checking etcd service running well on master nodes")
	time.Sleep(5 * time.Second)
	var endpoints []string
	for _, k := range nodes.GetMasterKubeNodes() {
		endpoints = append(endpoints, fmt.Sprintf("https://%s:2379", k.IP))
	}
	allEtcdStarted := false
	retry := 5
	for !allEtcdStarted && retry > 0 {
		output := os.RunCommandOn(fmt.Sprintf("sudo etcdctl --endpoints=%s --cacert=%s --cert=%s --key=%s member list",
			strings.Join(endpoints, ","), constant.EtcdCaCertPath, constant.EtcdClientCertPath,
			constant.EtcdClientKeyPath), nodes.Nodes[0].IP, true)
		var notStarted []string
		for _, line := range strings.Split(strings.TrimSuffix(output, "\n"), "\n") {
			fields := strings.Split(line, ", ")
			name := fields[2]
			if fields[1] != "started" {
				notStarted = append(notStarted, name)
			}
		}
		if len(notStarted) == 0 {
			allEtcdStarted = true
		} else {
			retry--
			if retry > 0 {
				util.UpdateSpinner(fmt.Sprintf("Waiting etcd to be \"started\" on %s", strings.Join(notStarted, ", ")))
				time.Sleep(5 * time.Second)
			} else {
				os.Exit("", 1)
			}
		}
	}
	util.StopSpinner("", logsymbols.Success)
}

func installHelm(nodes model.KubeNodes) {
	if !nodes.IncludeMaster() {
		return
	}
	// download and distribute compressed helm file
	var err error
	helmUrl = cfg.DeploymentCfg.GetHelmExactUrl(HelmVersion)
	helmCompressedFile = helmUrl[strings.LastIndex(helmUrl, "/")+1:]
	var filePath string
	var firstNode model.KubeNode
	var helmFileExists bool
	var skipInstallHelm []string
	for i, kubeNode := range nodes.GetMasterKubeNodes() {
		helmExists := os.CommandExists("helm")
		if helmExists {
			installedHelmVersion := os.RunCommand("helm version --short | cut -d+ -f1 | cut -dv -f2 | xargs", true)
			if installedHelmVersion == HelmVersion {
				skipInstallHelm = append(skipInstallHelm, kubeNode.IP.String())
				fmt.Printf("helm with \"%s\" version already installed on \"%s\"\n", HelmVersion, kubeNode.Hostname)
				continue
			}
		}
		if err != nil {
			os.Exit(err.Error(), 1)
		}
		if i == 0 {
			firstNode = kubeNode
			helmFolder := path.GetTKubeTmpDir(kubeNode.IP)
			if IsoPath != "" {
				helmFolder = fmt.Sprintf("%s/helm", constant.IsoMountDir)
			}
			filePath = fmt.Sprintf("%s/%s", helmFolder, helmCompressedFile)
			helmFileExists = os.IsFileExistsOn("", filePath, firstNode.IP)
		}
		if helmFileExists {
			util.StartSpinner(fmt.Sprintf("Transferring \"%s\" for %s to %s",
				helmCompressedFile, firstNode.IP, kubeNode.IP))
			err = os.TransferFile(filePath, fmt.Sprintf("%s/%s", path.GetTKubeTmpDir(kubeNode.IP), helmCompressedFile),
				firstNode.IP, kubeNode.IP)
			if err != nil {
				os.Exit(err.Error(), 1)
			}
			util.StopSpinner("", logsymbols.Success)
		} else {
			util.StartSpinner(fmt.Sprintf("Downloading \"%s\" to %s", helmCompressedFile, kubeNode.IP))
			os.RunCommandOn(fmt.Sprintf("wget -nc -P %s %s", path.GetTKubeTmpDir(kubeNode.IP), helmUrl), kubeNode.IP,
				true)
			if err != nil {
				os.Exit(err.Error(), 1)
			}
			if i == 0 {
				os.RunCommandOn(fmt.Sprintf("sudo cp %s/%s %s/", path.GetTKubeTmpDir(kubeNode.IP), helmCompressedFile,
					path.GetTKubeResourcesDir()), kubeNode.IP, true)
				if err != nil {
					os.Exit(err.Error(), 1)
				}
			}
			util.StopSpinner("", logsymbols.Success)
		}
	}

	// install helm
	for _, kubeNode := range nodes.GetMasterKubeNodes() {
		if slices.Contains(skipInstallHelm, kubeNode.IP.String()) {
			continue
		}
		util.StartSpinner(fmt.Sprintf("Installing helm-%s on \"%s\"", HelmVersion, kubeNode.Hostname))
		prevHelmPath := os.RunCommandOn("which helm || true", kubeNode.IP, true)
		if prevHelmPath != "" {
			os.RunCommandOn(fmt.Sprintf("sudo rm -rf %s", prevHelmPath), kubeNode.IP, true)
		}
		tmpDir := path.GetTKubeTmpDir(kubeNode.IP)
		os.RunCommandOn(fmt.Sprintf("tar -zxvf %s/%s -C %s", tmpDir, helmCompressedFile, tmpDir), kubeNode.IP, true)
		os.RunCommandOn(fmt.Sprintf("sudo cp %s/linux-amd64/helm /usr/bin/", tmpDir), kubeNode.IP, true)
		os.RunCommandOn("sudo chmod +x /usr/bin/helm", kubeNode.IP, true)
		util.StopSpinner("", logsymbols.Success)
	}
}

func installKeepAliveD(nodes model.KubeNodes) {
	for _, masterNode := range nodes.GetMasterKubeNodes() {
		os.RunCommandOn("sudo mkdir -p /etc/keepalived", masterNode.IP, true)
		os.RunCommandOn("sudo chmod -R 777 /etc/keepalived", masterNode.IP, true)
		os.InstallPackage("keepalived", masterNode.IP)
		keepalivedConfVars := util.TemplateVars{
			"Interface":       masterNode.Interface,
			"VirtualRouterID": cfg.DeploymentCfg.Keepalived.VirtualRouterId,
			"VirtualIP":       cfg.DeploymentCfg.Keepalived.VirtualIP,
			"Priority":        100,     // todo add to cfg
			"AuthPass":        "tkube", // todo add to cfg
		}
		rendered, err := util.RenderTemplate(templates.KeepalivedConf, keepalivedConfVars)
		os.ThrowIfError(err, 1)
		os.CreateFile([]byte(rendered), fmt.Sprintf("/etc/keepalived/%s", templates.KeepalivedConf.Name()),
			masterNode.IP)
		checkApiserverShVars := util.TemplateVars{
			"VirtualIP": cfg.DeploymentCfg.Keepalived.VirtualIP,
		}
		rendered, err = util.RenderTemplate(templates.CheckApiserverSh, checkApiserverShVars)
		os.ThrowIfError(err, 1)
		os.CreateFile([]byte(rendered), fmt.Sprintf("/etc/keepalived/%s", templates.CheckApiserverSh.Name()),
			masterNode.IP)
		os.RunCommandOn(fmt.Sprintf("sudo chmod -R 644 %s", "/etc/keepalived"), masterNode.IP, true)
		os.RunCommandOn(fmt.Sprintf("sudo chmod +x /etc/keepalived/%s", templates.CheckApiserverSh.Name()),
			masterNode.IP, true)
		os.RunCommandOn("sudo service keepalived restart", masterNode.IP, true)
	}
}

func initKubernetes(nodes model.KubeNodes) {
	var firstMasterNode *model.KubeNode
	var certKey string
	if nodes.IncludeMaster() {
		firstMasterNode = &nodes.GetMasterKubeNodes()[0]
		util.StartSpinner(fmt.Sprintf("Initializing kubernetes on \"%s\"", firstMasterNode.Hostname))
		certKey = kube.CreateCertKey(KubeVersion, firstMasterNode.IP)
		controlPlaneIP := firstMasterNode.IP
		if cfg.DeploymentCfg.Keepalived.Enabled {
			controlPlaneIP = cfg.DeploymentCfg.Keepalived.VirtualIP
		}
		os.CreateFile(kube.CreateCombinedKubeadmCfg(KubeVersion, controlPlaneIP, firstMasterNode.IP, certKey,
			cfg.DeploymentCfg, multiMasterDeployment),
			fmt.Sprintf("%s/kubeadm-config.yaml", path.GetTKubeCfgDir()), firstMasterNode.IP)
		util.StopSpinner("", logsymbols.Success)
		kubeConf := "net.bridge.bridge-nf-call-ip6tables = 1\nnet.bridge.bridge-nf-call-iptables = 1\nnet.ipv4.ip_forward = 1\n"
		os.CreateFile([]byte(kubeConf), "/etc/sysctl.d/kubernetes.conf", firstMasterNode.IP)
		os.RunCommandOn("sudo sysctl --system", firstMasterNode.IP, true)
		os.RunCommandOn(fmt.Sprintf("sudo kubeadm init --config %s/kubeadm-config.yaml --upload-certs",
			path.GetTKubeCfgDir()), firstMasterNode.IP, false)
		os.RunCommandOn("mkdir -p $HOME/.kube", firstMasterNode.IP, true)
		os.RunCommandOn("sudo cp /etc/kubernetes/admin.conf $HOME/.kube/config", firstMasterNode.IP, true)
		os.RunCommandOn("sudo chown $(id -u):$(id -g) $HOME/.kube/config", firstMasterNode.IP, true)
		os.RunCommandOn("sudo mkdir -p /root/.kube", firstMasterNode.IP, true)
		os.RunCommandOn("sudo cp /etc/kubernetes/admin.conf /root/.kube/config", firstMasterNode.IP, true)
		os.RunCommandOn("sudo chown $(id -u):$(id -g) /root/.kube/config", firstMasterNode.IP, true)
		util.StartSpinner(fmt.Sprintf("Applying calico config \"%s\" with version", getCalicoVersion()))
		os.RunCommandOn(fmt.Sprintf("mkdir -p %s", path.GetTKubeTmpDir(firstMasterNode.IP)), firstMasterNode.IP, true)
		var calicoUrl string
		if IsoPath != "" {
			calicoUrl = fmt.Sprintf("%s/calico/calico-%s.yaml", constant.IsoMountDir, CalicoVersion)
		}
		if strings.HasPrefix(calicoUrl, "/") { // check local file
			os.RunCommandOn(fmt.Sprintf("mkdir -p %s && sudo cp %s %s/calico.yaml",
				path.GetTKubeTmpDir(firstMasterNode.IP), calicoUrl,
				path.GetTKubeTmpDir(firstMasterNode.IP)), firstMasterNode.IP, true)
		} else {
			os.RunCommandOn(fmt.Sprintf("rm -f %s/calico.yaml && wget -nc -qO %s/calico.yaml %s --no-check-certificate",
				path.GetTKubeTmpDir(firstMasterNode.IP), path.GetTKubeTmpDir(firstMasterNode.IP),
				cfg.DeploymentCfg.GetCalicoExactUrl(getCalicoVersion())), firstMasterNode.IP, true)
		}
		if cfg.DeploymentCfg.Kubernetes.ImageRegistry != constant.DefaultKubeImageRegistry {
			os.RunCommandOn(fmt.Sprintf("sed -i 's/docker.io/%s/' %s/calico.yaml",
				cfg.DeploymentCfg.Kubernetes.ImageRegistry, path.GetTKubeTmpDir(firstMasterNode.IP)),
				firstMasterNode.IP, true)
		}
		os.RunCommandOn(fmt.Sprintf("kubectl apply -f %s/calico.yaml", path.GetTKubeTmpDir(firstMasterNode.IP)),
			firstMasterNode.IP, true)
		util.StopSpinner("", logsymbols.Success)
	}

	if firstMasterNode == nil {
		firstMasterNode = &cfg.DeploymentCfg.GetMasterKubeNodes()[0]
	}

	for _, workerNode := range nodes.GetWorkerKubeNodes() {
		os.RunCommandOn("mkdir -p $HOME/.kube", workerNode.IP, true)
		err := os.TransferFile("$HOME/.kube/config", "$HOME/.kube/config", firstMasterNode.IP, workerNode.IP)
		if err != nil {
			os.Exit(err.Error(), 1)
		}
	}

	for _, env := range cfg.DeploymentCfg.Kubernetes.Calico.EnvVars {
		kube.SetEnv("daemonset/calico-node", "kube-system", env)
	}
	kube.WaitUntilPodsRunningWithName(kubeSystemPodNames(firstMasterNode.Hostname), "kube-system")
	joinAsMasterCmd := ""
	if len(nodes.GetMasterKubeNodes()) > 1 {
		if certKey == "" {
			certKey = os.RunCommandOn(
				fmt.Sprintf("cat %s/kubeadm-config.yaml | grep certificateKey | awk -F ':' '{print $2}' | xargs",
					path.GetTKubeCfgDir()), firstMasterNode.IP, true)
		}
		joinAsMasterCmd = os.RunCommandOn(
			fmt.Sprintf("sudo kubeadm token create --print-join-command --certificate-key %s", certKey),
			firstMasterNode.IP, true)
	}
	for _, masterNode := range nodes.GetMasterKubeNodes() {
		if masterNode.IP.Equal(firstMasterNode.IP) {
			continue
		}
		kubeConf := "net.bridge.bridge-nf-call-ip6tables = 1\nnet.bridge.bridge-nf-call-iptables = 1\nnet.ipv4.ip_forward = 1\n"
		os.CreateFile([]byte(kubeConf), "/etc/sysctl.d/kubernetes.conf", masterNode.IP)
		os.RunCommandOn("sudo sysctl --system", firstMasterNode.IP, true)
		util.StartSpinner(fmt.Sprintf("Master node \"%s\" joining to cluster", masterNode.Hostname))
		joinCmd := fmt.Sprintf("%s --apiserver-advertise-address=%s", joinAsMasterCmd, masterNode.IP)
		os.RunCommandOn(fmt.Sprintf("sudo %s", joinCmd), masterNode.IP, true)
		util.StopSpinner(fmt.Sprintf("Master node \"%s\" has joined to cluster", masterNode.Hostname),
			logsymbols.Success)
		kube.WaitUntilPodsRunningWithName(kubeSystemPodNames(masterNode.Hostname), "kube-system")
		kube.UpdateServerInfoOnKubeAdminConf(masterNode.IP)
		kube.UpdateServerInfoOnKubeletConf(masterNode.IP)
		os.RunCommandOn("sudo service kubelet restart", masterNode.IP, true)
		os.RunCommandOn("mkdir -p $HOME/.kube", masterNode.IP, true)
		os.RunCommandOn("sudo cp /etc/kubernetes/admin.conf $HOME/.kube/config", masterNode.IP, true)
		os.RunCommandOn("sudo chown $(id -u):$(id -g) $HOME/.kube/config", masterNode.IP, true)
	}
	var joinAsWorkerCmd string
	for _, workerNode := range nodes.GetWorkerKubeNodes() {
		kubeConf := "net.bridge.bridge-nf-call-ip6tables = 1\nnet.bridge.bridge-nf-call-iptables = 1\nnet.ipv4.ip_forward = 1\n"
		os.CreateFile([]byte(kubeConf), "/etc/sysctl.d/kubernetes.conf", workerNode.IP)
		os.RunCommandOn("sudo sysctl --system", firstMasterNode.IP, true)
		if joinAsWorkerCmd == "" {
			joinAsWorkerCmd = os.RunCommandOn("sudo kubeadm token create --print-join-command",
				firstMasterNode.IP, true)
		}
		util.StartSpinner(fmt.Sprintf("Worker node \"%s\" joining to cluster", workerNode.Hostname))
		os.RunCommandOn(fmt.Sprintf("sudo %s", joinAsWorkerCmd), workerNode.IP, true)
		util.StopSpinner(fmt.Sprintf("Worker node \"%s\" has joined to cluster", workerNode.Hostname),
			logsymbols.Success)
	}
	if cfg.DeploymentCfg.Kubernetes.SchedulePodsOnMasters {
		os.RunCommandOn("kubectl taint nodes --all node-role.kubernetes.io/master- || true",
			firstMasterNode.IP, false)
		os.RunCommandOn("kubectl taint nodes --all node-role.kubernetes.io/control-plane- || true",
			firstMasterNode.IP, false)
	}
	time.Sleep(10 * time.Second)
	for _, node := range nodes.Nodes {
		os.RunCommandOn("sudo systemctl restart containerd", node.IP, true)
	}
	time.Sleep(10 * time.Second)
	kube.WaitUntilPodsRunning([]string{"kube-system"})
}

func kubeSystemPodNames(nodeName string) []string {
	var podNames []string
	podNames = append(podNames, fmt.Sprintf("kube-controller-manager-%s", nodeName))
	podNames = append(podNames, fmt.Sprintf("kube-scheduler-%s", nodeName))
	podNames = append(podNames, fmt.Sprintf("kube-apiserver-%s", nodeName))
	return podNames
}

func getCalicoVersion() string {
	if CalicoVersion == "auto" {
		kubeSemVer, _ := version.NewVersion(KubeVersion)
		kube127Ver, _ := version.NewVersion("1.27")
		kube124Ver, _ := version.NewVersion("1.24")
		kube123Ver, _ := version.NewVersion("1.23")
		kube122Ver, _ := version.NewVersion("1.22")
		kube121Ver, _ := version.NewVersion("1.21")
		kube120Ver, _ := version.NewVersion("1.20")
		kube119Ver, _ := version.NewVersion("1.19")
		kube118Ver, _ := version.NewVersion("1.18")
		if kubeSemVer.GreaterThanOrEqual(kube127Ver) {
			return "3.27.2"
		} else if kubeSemVer.GreaterThanOrEqual(kube124Ver) {
			return "3.26.4"
		} else if kubeSemVer.GreaterThanOrEqual(kube123Ver) {
			return "3.25.2"
		} else if kubeSemVer.GreaterThanOrEqual(kube122Ver) {
			return "3.24.6"
		} else if kubeSemVer.GreaterThanOrEqual(kube121Ver) {
			return "3.23"
		} else if kubeSemVer.GreaterThanOrEqual(kube120Ver) {
			return "3.21"
		} else if kubeSemVer.GreaterThanOrEqual(kube119Ver) {
			return "3.20"
		} else if kubeSemVer.GreaterThanOrEqual(kube118Ver) {
			return "3.18"
		} else { // kube version between 1.17 and 1.18
			return "3.17"
		}
	} else {
		return CalicoVersion
	}
}
