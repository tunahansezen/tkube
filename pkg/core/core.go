package core

import (
	cfg "com.github.tunahansezen/tkube/pkg/config"
	conn "com.github.tunahansezen/tkube/pkg/connection"
	"com.github.tunahansezen/tkube/pkg/os"
	"com.github.tunahansezen/tkube/pkg/path"
	"fmt"
	"github.com/hashicorp/go-version"
)

func PreRun() {
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
	os.RemoteNode = &conn.Node{IP: os.RemoteNodeIP}
	toggleDebug()
	os.DetectOS()
	path.CalculatePaths()
	cfg.ReadConfig()
	os.RunCommand(fmt.Sprintf("mkdir -p %s", path.GetTKubeResourcesDir()), true)
	for _, kubeNode := range cfg.DeploymentCfg.Nodes {
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
