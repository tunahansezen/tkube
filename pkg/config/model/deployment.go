package model

import (
	"encoding/json"
	"github.com/hashicorp/go-version"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"
	"net"
	"reflect"
	"strings"
)

type DeploymentConfig struct {
	Nodes       []KubeNode `yaml:"nodes"`
	CentOS      CentOS     `yaml:"centOS,omitempty"`
	Packages    []string   `yaml:"packages"`
	Keepalived  KeepAliveD `yaml:"keepalived"`
	Containerd  ContainerD `yaml:"containerd"`
	Docker      Docker     `yaml:"docker"`
	Etcd        Etcd       `yaml:"etcd"`
	Kubernetes  Kubernetes `yaml:"kubernetes"`
	Helm        Helm       `yaml:"helm"`
	CustomRepos []Repo     `yaml:"customRepos"`
}

type KubeNode struct {
	Hostname  string `yaml:"hostname"`
	IP        net.IP `yaml:"IP"`
	Interface string `yaml:"interface"`
	KubeType  string `yaml:"kubeType"`
	SshUser   string `yaml:"sshUser"`
	SshPass   string `yaml:"sshPass"`
}

type CentOS struct {
	SetSelinuxPermissive bool `yaml:"setSelinuxPermissive"`
}

type KeepAliveD struct {
	Enabled         bool   `yaml:"enabled"`
	VirtualIP       net.IP `yaml:"virtualIP"`
	VirtualRouterId int    `yaml:"virtualRouterId"`
}

type ContainerD struct {
	Cri CRI `yaml:"cri"`
}

type CRI struct {
	SandboxImage string `yaml:"sandboxImage"`
}

type Docker struct {
	Repo   Repo            `yaml:"repo"`
	Daemon DockerDaemonCfg `yaml:"daemon" json:"daemon"`
}

type DockerDaemonCfg struct {
	ExecOpts           []string            `yaml:"execOpts" json:"exec-opts"`
	LogDriver          string              `yaml:"logDriver" json:"log-driver"`
	LogOpts            DockerDaemonLogOpts `yaml:"logOpts" json:"log-opts"`
	RegistryMirrors    []string            `yaml:"registryMirrors" json:"registry-mirrors"`
	InsecureRegistries []string            `yaml:"insecureRegistries" json:"insecure-registries"`
	Debug              bool                `yaml:"debug" json:"debug"`
	Experimental       bool                `yaml:"experimental" json:"experimental"`
	StorageDriver      string              `yaml:"storageDriver" json:"storage-driver"`
}

type DockerDaemonLogOpts struct {
	MaxFile string `yaml:"maxFile" json:"max-file"`
	MaxSize string `yaml:"maxSize" json:"max-size"`
}

type Etcd struct {
	DownloadUrl string `yaml:"downloadUrl"`
}

type Kubernetes struct {
	BashCompletion        bool   `yaml:"bashCompletion"`
	Repo                  Repo   `yaml:"repo"`
	ImageRegistry         string `yaml:"imageRegistry"`
	PodSubnet             string `yaml:"podSubnet"`
	SchedulePodsOnMasters bool   `yaml:"schedulePodsOnMasters"`
	Calico                Calico `yaml:"calico"`
	CalicoUrl             string `yaml:"calicoUrl,omitempty"` // deprecated
}

type Calico struct {
	Url     string   `yaml:"url"`
	EnvVars []string `yaml:"envVars"`
}

type Helm struct {
	DownloadUrl string `yaml:"downloadUrl"`
}

type Repo struct {
	Enabled bool   `yaml:"enabled"`
	Name    string `yaml:"name"`
	Address string `yaml:"address"`
	Key     string `yaml:"key"`
}

func (repo Repo) ShortName() string {
	return strings.ReplaceAll(strings.ToLower(repo.Name), " ", "")
}

func DeploymentCfgViperDecodeHook() viper.DecoderConfigOption {
	return viper.DecodeHook(
		mapstructure.ComposeDecodeHookFunc(
			func(f reflect.Type, t reflect.Type, data interface{}) (interface{}, error) {
				if f.Kind() != reflect.String || t != reflect.TypeOf(net.IP{}) {
					return data, nil
				}
				ip := net.ParseIP(data.(string))
				return ip, nil
			},
		),
	)
}

func (dc DeploymentConfig) GetMasterKubeNodeIPs() []net.IP {
	var masterNodeIPs []net.IP
	for _, node := range dc.Nodes {
		if node.KubeType == "master" {
			masterNodeIPs = append(masterNodeIPs, node.IP)
		}
	}
	return masterNodeIPs
}

func (dc DeploymentConfig) GetMasterKubeNodes() []KubeNode {
	var nodes []KubeNode
	for _, node := range dc.Nodes {
		if node.KubeType == "master" {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

func (dc DeploymentConfig) GetWorkerKubeNodes() []KubeNode {
	var nodes []KubeNode
	for _, node := range dc.Nodes {
		if node.KubeType == "worker" {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

func DefaultDockerDaemonCfg() DockerDaemonCfg {
	var dockerDaemonCfg DockerDaemonCfg
	dockerDaemonCfg.ExecOpts = []string{"native.cgroupdriver=systemd"}
	dockerDaemonCfg.LogDriver = "json-file"
	dockerDaemonCfg.LogOpts.MaxFile = "3"
	dockerDaemonCfg.LogOpts.MaxSize = "100m"
	dockerDaemonCfg.RegistryMirrors = []string{}
	dockerDaemonCfg.InsecureRegistries = []string{}
	dockerDaemonCfg.Debug = false
	dockerDaemonCfg.Experimental = false
	dockerDaemonCfg.StorageDriver = "overlay2"

	return dockerDaemonCfg
}

func (ddc DockerDaemonCfg) MarshallJson() []byte {
	bytes, _ := json.MarshalIndent(ddc, "", "  ")
	return append(bytes, []byte("\n")...)
}

func (dc DeploymentConfig) GetEtcdExactUrl(etcdVersion string) (exactUrl string) {
	if dc.Etcd.DownloadUrl == "default" {
		url := "https://github.com/coreos/etcd/releases/download/v{version}/etcd-v{version}-linux-amd64.tar.gz"
		return strings.ReplaceAll(url, "{version}", etcdVersion)
	} else {
		return strings.ReplaceAll(dc.Etcd.DownloadUrl, "{version}", etcdVersion)
	}
}

func (dc DeploymentConfig) GetHelmExactUrl(helmVersion string) (exactUrl string) {
	if dc.Helm.DownloadUrl == "default" {
		url := "https://get.helm.sh/helm-v{version}-linux-amd64.tar.gz"
		return strings.ReplaceAll(url, "{version}", helmVersion)
	} else {
		return strings.ReplaceAll(dc.Helm.DownloadUrl, "{version}", helmVersion)
	}
}

func (dc DeploymentConfig) GetCalicoExactUrl(calicoVersion string) (exactUrl string) {
	if dc.Kubernetes.Calico.Url == "default" {
		var url string
		calicoSemVer, _ := version.NewVersion(calicoVersion)
		calicoArchivedSemVer, _ := version.NewVersion("3.23")
		if calicoSemVer.GreaterThan(calicoArchivedSemVer) {
			url = "https://raw.githubusercontent.com/projectcalico/calico/v{version}/manifests/calico.yaml"
		} else {
			url = "https://docs.projectcalico.org/archive/v{version}/manifests/calico.yaml"
		}
		return strings.ReplaceAll(url, "{version}", calicoVersion)
	} else {
		return strings.ReplaceAll(dc.Kubernetes.Calico.Url, "{version}", calicoVersion)
	}
}
