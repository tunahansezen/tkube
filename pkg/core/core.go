package core

import (
	"fmt"

	cfg "com.github.tunahansezen/tkube/pkg/config"
	"com.github.tunahansezen/tkube/pkg/config/model"
	conn "com.github.tunahansezen/tkube/pkg/connection"
	"com.github.tunahansezen/tkube/pkg/constant"
	"com.github.tunahansezen/tkube/pkg/os"
	"com.github.tunahansezen/tkube/pkg/path"
	"com.github.tunahansezen/tkube/pkg/util"
	"github.com/guumaster/logsymbols"
	"github.com/hashicorp/go-version"
	"gopkg.in/yaml.v3"
)

func PreRun() {
	os.RemoteNode = &conn.Node{IP: os.RemoteNodeIP}
	if IsoPath != "" {
		err := conn.CheckSSHConnection(&conn.Node{IP: os.RemoteNode.IP})
		if err != nil {
			os.Exit(err.Error(), 1)
		}
		os.AddToSudoers(os.RemoteNode.IP)
		util.StartSpinner(fmt.Sprintf("Umounting previous iso dir \"%s\" if exists on \"%s\"", constant.IsoMountDir, os.RemoteNode.IP))
		os.UmountISO(constant.IsoMountDir, os.RemoteNode.IP)
		util.StopSpinner("", logsymbols.Success)
		util.StartSpinner(fmt.Sprintf("Mounting iso \"%s\" to dir \"%s\" on \"%s\"", IsoPath, constant.IsoMountDir, os.RemoteNode.IP))
		os.MountISO(constant.IsoMountDir, IsoPath, os.RemoteNode.IP)
		util.StopSpinner("", logsymbols.Success)
		println("Reading versions from iso file")
		fileStr := os.RunCommand(fmt.Sprintf("sudo cat %s/versions", constant.IsoMountDir), true)
		println(fileStr)
		var isoVersions model.IsoVersions
		err = yaml.Unmarshal([]byte(fileStr), &isoVersions)
		if err != nil {
			os.Exit(err.Error(), 1)
		}
		KubeVersion = isoVersions.Kubernetes
		DockerVersion = isoVersions.Docker
		CalicoVersion = isoVersions.Calico
		EtcdVersion = isoVersions.Etcd
		HelmVersion = isoVersions.Helm
		HelmfileVersion = isoVersions.Helmfile
	}
	helmSemVer, _ := version.NewVersion(HelmVersion)
	minHelmVer, _ := version.NewVersion("3.0.0")
	if helmSemVer.LessThan(minHelmVer) {
		os.Exit(fmt.Sprintf("Minimum supported helm version is  \"%s\"", minHelmVer), 1)
	}
	kubeSemVer, _ := version.NewVersion(KubeVersion)
	minKubeVer, _ := version.NewVersion("1.17.0")
	if kubeSemVer.LessThan(minKubeVer) {
		os.Exit(fmt.Sprintf("Minimum supported kubernetes version is \"%s\"", minKubeVer), 1)
	}
	if kubeSemVer.String() != KubeVersion {
		os.Exit(fmt.Sprintf("Kubernetes version needed to be defined exactly. ex: %s", kubeSemVer.String()), 1)
	}
	toggleDebug()
	os.DetectOS()
	path.CalculatePaths()
	cfg.ReadConfig()
	os.RunCommand(fmt.Sprintf("mkdir -p %s", path.GetTKubeResourcesDir()), true)
	for _, kubeNode := range cfg.DeploymentCfg.GetKubeNodes() {
		err := conn.CheckSSHConnection(&conn.Node{IP: kubeNode.IP})
		if err != nil {
			os.Exit(err.Error(), 1)
		}
		os.AddToSudoers(kubeNode.IP)
		// todo check sshpass
		os.RunCommandOn(fmt.Sprintf("mkdir -p %s", path.GetTKubeTmpDir(kubeNode.IP)),
			kubeNode.IP, true)
		if err != nil {
			os.Exit(err.Error(), 1)
		}
	}
}
