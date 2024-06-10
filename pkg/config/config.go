package config

import (
	"bytes"
	"com.github.tunahansezen/tkube/pkg/config/model"
	conn "com.github.tunahansezen/tkube/pkg/connection"
	"com.github.tunahansezen/tkube/pkg/constant"
	"com.github.tunahansezen/tkube/pkg/os"
	"com.github.tunahansezen/tkube/pkg/path"
	"com.github.tunahansezen/tkube/pkg/util"
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
	"net"
	"strconv"
	"strings"
)

var (
	DeploymentCfg     model.DeploymentConfig
	deploymentCfgFile string
)

func ReadConfig() error {
	var err error
	// deployment config
	err = readDeploymentConfig()
	if err != nil {
		os.Exit(err.Error(), 1)
	}
	return nil
}

func readDeploymentConfig() error {
	deploymentCfgFile = fmt.Sprintf("%s/%s.%s", path.GetTKubeCfgDir(), constant.DeploymentCfgName, constant.DefaultCfgType)
	var err error
	if os.IsFileExists(deploymentCfgFile) {
		viper.SetConfigName(constant.DeploymentCfgName)
		viper.SetConfigType(constant.DefaultCfgType)
		if os.RemoteNode == nil {
			viper.AddConfigPath(path.GetTKubeCfgDir())
			err = viper.ReadInConfig()
		} else {
			var output string
			output = os.RunCommand(fmt.Sprintf("cat %s", deploymentCfgFile), true)
			err = viper.ReadConfig(strings.NewReader(output))
		}
		if err != nil {
			return err
		}
		log.Debugf("Using deployment config file: %s", deploymentCfgFile)
		err = viper.Unmarshal(&DeploymentCfg, model.DeploymentCfgViperDecodeHook())
		if err != nil {
			os.Exit(fmt.Sprintf("Error occurred while reading deployment config file\n%s", err.Error()), 1)
		}
	}
	prevBytes, _ := yaml.Marshal(DeploymentCfg)
	err = askDeploymentConfig()
	if err != nil {
		return err
	}
	currentBytes, _ := yaml.Marshal(DeploymentCfg)
	if bytes.Compare(prevBytes, currentBytes) != 0 { // config changed
		var b bytes.Buffer
		yamlEncoder := yaml.NewEncoder(&b)
		yamlEncoder.SetIndent(2)
		err = yamlEncoder.Encode(&DeploymentCfg)
		os.CreateFile(b.Bytes(), deploymentCfgFile, os.RemoteNode.IP)
		var confirmed bool
		confirmed, err = util.UserConfirmation("Config created or updated. Do you want to continue?")
		if err != nil {
			os.Exit(err.Error(), 1)
		}
		if !confirmed {
			os.Exit("", 0)
		}
	}

	return nil
}

func askDeploymentConfig() (err error) {
	var nodes []model.KubeNode
	addNode := true
	index := 1
	firstAsk := false
	if len(DeploymentCfg.Nodes) == 0 {
		firstAsk = true
	}
	for firstAsk && addNode {
		fmt.Printf("%s node information:\n", util.GetOrdinalNumber(index))
		var node model.KubeNode
		node.Hostname, err = util.AskString("hostname", false, util.CommonValidator)
		if err != nil {
			return err
		}
		node.IP, err = util.AskIP("IP")
		if err != nil {
			return err
		}
		node.Interface, err = util.AskString("interface", false, util.CommonValidator)
		if err != nil {
			return err
		}
		var ktStr string
		ktStr, err = util.AskChoice("kubeType", []string{"master", "worker"})
		if err != nil {
			return err
		}
		node.KubeType = ktStr
		if err != nil {
			return err
		}
		nodes = append(nodes, node)
		addNode, err = util.UserConfirmation("Do you want to add another node?")
		if err != nil {
			return err
		}
		index++
	}
	// todo check at least one master
	// todo check --multi-master
	if len(nodes) != 0 {
		DeploymentCfg.Nodes = nodes
	}
	for _, node := range DeploymentCfg.Nodes {
		if node.SshUser != "" && node.SshPass != "" {
			err = conn.WriteSSHData(node.IP.String(), node.SshUser, node.SshPass)
			if err != nil {
				return err
			}
		}
	}

	if os.OS == os.CentOS {
		if firstAsk {
			DeploymentCfg.CentOS = model.CentOS{SetSelinuxPermissive: true}
		}
	}

	if len(DeploymentCfg.Packages) == 0 {
		DeploymentCfg.Packages = []string{"sshpass", "ca-certificates", "curl", "wget", "bash-completion", "net-tools"}
		if os.OS == os.Ubuntu {
			DeploymentCfg.Packages = append(DeploymentCfg.Packages, "gnupg", "apt-transport-https")
		} else if os.OS == os.CentOS {
			DeploymentCfg.Packages = append(DeploymentCfg.Packages, "gnupg2", "yum-utils", "yum-plugin-versionlock")
		}
	}

	// keepalived
	if firstAsk {
		DeploymentCfg.Keepalived.Enabled, _ = util.UserConfirmation("Do you want to use keepalived?")
	}
	if DeploymentCfg.Keepalived.Enabled {
		if DeploymentCfg.Keepalived.VirtualIP == nil {
			DeploymentCfg.Keepalived.VirtualIP, err = util.AskIP("keepalived virtual IP")
			if err != nil {
				return err
			}
		}
	} else {
		if DeploymentCfg.Keepalived.VirtualIP == nil {
			DeploymentCfg.Keepalived.VirtualIP = net.ParseIP("127.0.0.1")
		}
	}
	if DeploymentCfg.Keepalived.Enabled {
		if DeploymentCfg.Keepalived.VirtualRouterId == 0 {
			var virtualRouterIdStr string
			virtualRouterIdStr, err = util.AskString("keepalived virtual router id", false,
				util.ZeroTo255Validator)
			DeploymentCfg.Keepalived.VirtualRouterId, _ = strconv.Atoi(virtualRouterIdStr)
		}
	} else {
		DeploymentCfg.Keepalived.VirtualRouterId = 1
	}

	// etcd
	if DeploymentCfg.Etcd.DownloadUrl == "" {
		DeploymentCfg.Etcd.DownloadUrl = constant.DefaultEtcdUrl
	}

	// containerd
	if DeploymentCfg.Containerd.Cri.SandboxImage == "" {
		DeploymentCfg.Containerd = model.ContainerD{
			Cri: model.CRI{SandboxImage: constant.DefaultContainerdSandboxImage},
		}
	}

	// docker
	if DeploymentCfg.Docker.Repo.Address == "" {
		DeploymentCfg.Docker.Repo.Enabled = true
		DeploymentCfg.Docker.Repo.Name = "Docker"
		if os.OS == os.Ubuntu {
			DeploymentCfg.Docker.Repo.Address = constant.DefaultDockerAptRepoAddress
			DeploymentCfg.Docker.Repo.Key = constant.DefaultDockerAptRepoKey
		} else if os.OS == os.CentOS {
			DeploymentCfg.Docker.Repo.Address = constant.DefaultDockerYumRepoAddress
			DeploymentCfg.Docker.Repo.Key = constant.DefaultDockerYumRepoKey
		}
	}
	defDockerDaemonCfg := model.DefaultDockerDaemonCfg()
	ddcChanged := false
	if firstAsk {
		var insecureRegistriesToAdd []string
		var registryMirrorsToAdd []string
		addInsecureRegistry := true
		index = 1
		for addInsecureRegistry {
			addInsecureRegistry, err = util.UserConfirmation("Do you want to add an insecure registry?")
			if err != nil {
				return err
			}
			if !addInsecureRegistry {
				continue
			}
			var insecureRegistry string
			// todo check valid url
			insecureRegistry, err = util.AskString(fmt.Sprintf("%s insecure registry", util.GetOrdinalNumber(index)),
				false, util.CommonValidator)
			if err != nil {
				return err
			}
			insecureRegistriesToAdd = append(insecureRegistriesToAdd, insecureRegistry)
			var addRegistryMirrorsAsWell bool
			addRegistryMirrorsAsWell, err = util.UserConfirmation(
				fmt.Sprintf("Do you want to add \"%s\" to the registry mirrors as well?", insecureRegistry))
			if err != nil {
				return err
			}
			if addRegistryMirrorsAsWell {
				registryMirrorsToAdd = append(registryMirrorsToAdd, insecureRegistry)
			}
		}
		defDockerDaemonCfg.InsecureRegistries = insecureRegistriesToAdd

		addRegistryMirror := true
		index = 1
		for addRegistryMirror {
			addRegistryMirror, err = util.UserConfirmation("Do you want to add an registry mirror?")
			if err != nil {
				return err
			}
			if !addRegistryMirror {
				continue
			}
			var registryMirror string
			// todo check valid url
			registryMirror, err = util.AskString(fmt.Sprintf("%s registry mirror", util.GetOrdinalNumber(index)),
				false, util.CommonValidator)
			if err != nil {
				return err
			}
			insecureRegistriesToAdd = append(insecureRegistriesToAdd, registryMirror)
		}
		defDockerDaemonCfg.RegistryMirrors = registryMirrorsToAdd
		ddcChanged = true
	}
	if ddcChanged == true {
		DeploymentCfg.Docker.Daemon = defDockerDaemonCfg
	}

	// kubernetes
	if firstAsk {
		DeploymentCfg.Kubernetes.BashCompletion = true
		DeploymentCfg.Kubernetes.SchedulePodsOnMasters, err =
			util.UserConfirmation("Do you want to schedule pods on masters?")
		if err != nil {
			return err
		}
	}
	if DeploymentCfg.Kubernetes.Repo.Address == "" {
		DeploymentCfg.Kubernetes.Repo.Enabled = true
		DeploymentCfg.Kubernetes.Repo.Name = "Kubernetes"
		if os.OS == os.Ubuntu {
			DeploymentCfg.Kubernetes.Repo.Address = constant.DefaultKubeAptRepoAddress
			DeploymentCfg.Kubernetes.Repo.Key = constant.DefaultKubeAptRepoKey
		} else if os.OS == os.CentOS {
			DeploymentCfg.Kubernetes.Repo.Address = constant.DefaultKubeYumRepoAddress
			DeploymentCfg.Kubernetes.Repo.Key = constant.DefaultKubeYumRepoKey
		}
	}
	if DeploymentCfg.Kubernetes.Calico.Url == "" {
		if DeploymentCfg.Kubernetes.CalicoUrl != "" {
			DeploymentCfg.Kubernetes.Calico.Url = DeploymentCfg.Kubernetes.CalicoUrl
			DeploymentCfg.Kubernetes.CalicoUrl = ""
		} else {
			DeploymentCfg.Kubernetes.Calico.Url = constant.DefaultCalicoUrl
		}
	}
	if DeploymentCfg.Kubernetes.Calico.EnvVars == nil {
		DeploymentCfg.Kubernetes.Calico.EnvVars = []string{}
	}
	if DeploymentCfg.Kubernetes.ImageRegistry == "" {
		DeploymentCfg.Kubernetes.ImageRegistry = constant.DefaultKubeImageRegistry
	}
	if DeploymentCfg.Kubernetes.PodSubnet == "" {
		DeploymentCfg.Kubernetes.PodSubnet = constant.DefaultPodSubnet
	}

	// helm
	if DeploymentCfg.Helm.DownloadUrl == "" {
		DeploymentCfg.Helm.DownloadUrl = constant.DefaultHelmUrl
	}

	// custom apt repo
	if len(DeploymentCfg.CustomRepos) == 0 {
		var aptRepo model.Repo
		aptRepo.Enabled = false
		aptRepo.Name = "Custom Repo 1"
		aptRepo.Address = "https://customrepo.com/repository/repo stable main"
		aptRepo.Key = "https://customrepo.com/repository/raw/gpg-keys/customrepo.gpg"
		DeploymentCfg.CustomRepos = []model.Repo{aptRepo}
	}
	return nil
}
