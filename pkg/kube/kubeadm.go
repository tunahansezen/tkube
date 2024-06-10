package kube

import (
	"bytes"
	"com.github.tunahansezen/tkube/pkg/config/model"
	"com.github.tunahansezen/tkube/pkg/constant"
	"com.github.tunahansezen/tkube/pkg/os"
	"fmt"
	"github.com/hashicorp/go-version"
	"github.com/invopop/yaml"
	yamlv3 "gopkg.in/yaml.v3"
	kubeApi "k8s.io/client-go/tools/clientcmd/api"
	"net"
	"strings"
)

const (
	kubeAdminConfPath = "/etc/kubernetes/admin.conf"
	kubeletConfPath   = "/etc/kubernetes/kubelet.conf"
)

type KubeadmClusterCfg struct {
	ApiVersion           string      `yaml:"apiVersion" json:"apiVersion"`
	Kind                 string      `yaml:"kind" json:"kind"`
	KubernetesVersion    string      `yaml:"kubernetesVersion" json:"kubernetesVersion"`
	ApiServer            *apiServer  `yaml:"apiServer" json:"apiServer"`
	ControlPlaneEndpoint string      `yaml:"controlPlaneEndpoint" json:"controlPlaneEndpoint"`
	Networking           *networking `yaml:"networking" json:"networking"`
	Etcd                 *etcd       `yaml:"etcd" json:"etcd,omitempty"`
	ImageRepository      string      `yaml:"imageRepository" json:"imageRepository"`
	Dns                  *dns        `yaml:"dns" json:"dns,omitempty"`
}

type apiServer struct {
	CertSANs []string `yaml:"certSANs" json:"certSANs"`
}

type networking struct {
	PodSubnet string `yaml:"podSubnet" json:"podSubnet"`
}

type etcd struct {
	External *etcdExternal `yaml:"external" json:"external,omitempty"`
}

type etcdExternal struct {
	Endpoints []string `yaml:"endpoints" json:"endpoints,omitempty"`
	CAFile    string   `yaml:"caFile" json:"caFile,omitempty"`
	CertFile  string   `yaml:"certFile" json:"certFile,omitempty"`
	KeyFile   string   `yaml:"keyFile" json:"keyFile,omitempty"`
}

type dns struct {
	ImageRepository string `yaml:"imageRepository" json:"imageRepository"`
}

type KubeadmInitCfg struct {
	ApiVersion       string            `yaml:"apiVersion" json:"apiVersion"`
	Kind             string            `yaml:"kind" json:"kind"`
	CertificateKey   string            `yaml:"certificateKey" json:"certificateKey"`
	LocalApiEndpoint *localApiEndpoint `yaml:"localAPIEndpoint" json:"localAPIEndpoint"`
}

type localApiEndpoint struct {
	AdvertiseAddress string `yaml:"advertiseAddress" json:"advertiseAddress"`
	BindPort         int    `yaml:"bindPort" json:"bindPort"`
}

type KubeletCfg struct {
	ApiVersion   string `yaml:"apiVersion" json:"apiVersion"`
	Kind         string `yaml:"kind" json:"kind"`
	CGroupDriver string `yaml:"cgroupDriver" json:"cgroupDriver"`
}

type KubeConfig struct {
	Kind           string               `json:"kind,omitempty"`
	APIVersion     string               `json:"apiVersion,omitempty"`
	Preferences    *kubeApi.Preferences `json:"preferences"`
	Clusters       []ClusterMapItem     `json:"clusters"`
	AuthInfos      []AuthInfoMapItem    `json:"users"`
	Contexts       []ContextMapItem     `json:"contexts"`
	CurrentContext string               `json:"current-context"`
}

type ClusterMapItem struct {
	Key   string          `json:"name,omitempty"`
	Value kubeApi.Cluster `json:"cluster,omitempty"`
}

type AuthInfoMapItem struct {
	Key   string           `json:"name,omitempty"`
	Value kubeApi.AuthInfo `json:"user,omitempty"`
}

type ContextMapItem struct {
	Key   string          `json:"name,omitempty"`
	Value kubeApi.Context `json:"context,omitempty"`
}

func CreateCombinedKubeadmCfg(kubeVersion string, controlPlaneIP net.IP, certKey string,
	dc model.DeploymentConfig, multiMasterDeployment bool) []byte {

	var b bytes.Buffer
	yamlEncoder := yamlv3.NewEncoder(&b)
	yamlEncoder.SetIndent(2)
	kubeadmInitCfg := createDefaultKubeadmInitCfg(kubeVersion, controlPlaneIP, certKey)
	yamlEncoder.Encode(&kubeadmInitCfg)
	b.WriteString("\n")
	kubeadmClusterCfg := createKubeadmClusterCfg(kubeVersion, controlPlaneIP, dc, multiMasterDeployment)
	yamlEncoder.Encode(&kubeadmClusterCfg)
	b.WriteString("\n")
	kubeletCfg := createKubeadmKubeletCfg(dc)
	yamlEncoder.Encode(&kubeletCfg)
	b.WriteString("\n")
	return b.Bytes()
}

func createKubeadmClusterCfg(kubeVersion string, controlPlaneIP net.IP,
	dc model.DeploymentConfig, multiMasterDeployment bool) (kcc KubeadmClusterCfg) {

	kubeSemVer, _ := version.NewVersion(kubeVersion)
	kubeadmCfgApiSemVerV3, _ := version.NewVersion("1.20.0")
	kubeadmCfgApiSemVerV2, _ := version.NewVersion("1.17.0")
	if kubeSemVer.GreaterThanOrEqual(kubeadmCfgApiSemVerV3) {
		kcc.ApiVersion = "kubeadm.k8s.io/v1beta3"
	} else if kubeSemVer.GreaterThanOrEqual(kubeadmCfgApiSemVerV2) {
		kcc.ApiVersion = "kubeadm.k8s.io/v1beta2"
	} else {
		kcc.ApiVersion = "kubeadm.k8s.io/v1beta1"
	}
	kcc.Kind = "ClusterConfiguration"
	kcc.KubernetesVersion = fmt.Sprintf("v%s", kubeSemVer.String())
	var apiServerCertSANs []string
	for _, ip := range dc.GetMasterKubeNodeIPs() {
		apiServerCertSANs = append(apiServerCertSANs, ip.String())
	}
	if dc.Keepalived.Enabled {
		apiServerCertSANs = append(apiServerCertSANs, dc.Keepalived.VirtualIP.String())
	}
	kcc.ApiServer = &apiServer{
		CertSANs: apiServerCertSANs,
	}
	kcc.ControlPlaneEndpoint = fmt.Sprintf("%s:6443", controlPlaneIP)
	kcc.Networking = &networking{
		PodSubnet: dc.Kubernetes.PodSubnet,
	}
	if multiMasterDeployment {
		var etcdExternalEndpoints []string
		for _, ip := range dc.GetMasterKubeNodeIPs() {
			etcdExternalEndpoints = append(etcdExternalEndpoints, fmt.Sprintf("https://%s:2379", ip))
		}
		kcc.Etcd = &etcd{
			External: &etcdExternal{
				Endpoints: etcdExternalEndpoints,
				CAFile:    constant.EtcdCaCertPath,
				CertFile:  constant.EtcdClientCertPath,
				KeyFile:   constant.EtcdClientKeyPath,
			},
		}
	}
	kcc.ImageRepository = dc.Kubernetes.ImageRegistry
	if kcc.ImageRepository != constant.DefaultKubeImageRegistry {
		kcc.Dns = &dns{ImageRepository: fmt.Sprintf("%s/coredns", kcc.ImageRepository)}
	}
	return kcc
}

func createDefaultKubeadmInitCfg(kubeVersion string, advertiseIP net.IP,
	certKey string) (kic KubeadmInitCfg) {

	kubeSemVer, _ := version.NewVersion(kubeVersion)
	kubeadmCfgApiSemVerV3, _ := version.NewVersion("1.20.0")
	kubeadmCfgApiSemVerV2, _ := version.NewVersion("1.17.0")
	if kubeSemVer.GreaterThanOrEqual(kubeadmCfgApiSemVerV3) {
		kic.ApiVersion = "kubeadm.k8s.io/v1beta3"
	} else if kubeSemVer.GreaterThanOrEqual(kubeadmCfgApiSemVerV2) {
		kic.ApiVersion = "kubeadm.k8s.io/v1beta2"
	} else {
		kic.ApiVersion = "kubeadm.k8s.io/v1beta1"
	}
	kic.Kind = "InitConfiguration"
	kic.CertificateKey = certKey
	if advertiseIP != nil {
		kic.LocalApiEndpoint = &localApiEndpoint{
			AdvertiseAddress: advertiseIP.String(),
			BindPort:         6443,
		}
		kic.LocalApiEndpoint.AdvertiseAddress = advertiseIP.String()
		kic.LocalApiEndpoint.BindPort = 6443
	}
	return kic
}

func createKubeadmKubeletCfg(dc model.DeploymentConfig) (kc KubeletCfg) {
	kc.ApiVersion = "kubelet.config.k8s.io/v1beta1"
	kc.Kind = "KubeletConfiguration"
	for _, opt := range dc.Docker.Daemon.ExecOpts {
		if strings.Contains(opt, "native.cgroupdriver=") {
			kc.CGroupDriver = strings.Split(opt, "=")[1]
		}
	}
	if kc.CGroupDriver == "" {
		kc.CGroupDriver = "systemd"
	}
	return kc
}

func UpdateServerInfoOnKubeAdminConf(ip net.IP) {
	os.RunCommandOn(fmt.Sprintf("sudo chmod -R 777 %s", "/etc/kubernetes"), ip, true)
	updateServerInfoOnKubeConfig(kubeAdminConfPath, ip)
	os.RunCommandOn(fmt.Sprintf("sudo chmod -R 644 %s", "/etc/kubernetes"), ip, true)
}

func UpdateServerInfoOnKubeletConf(ip net.IP) {
	os.RunCommandOn(fmt.Sprintf("sudo chmod -R 777 %s", "/etc/kubernetes"), ip, true)
	updateServerInfoOnKubeConfig(kubeletConfPath, ip)
	os.RunCommandOn(fmt.Sprintf("sudo chmod -R 644 %s", "/etc/kubernetes"), ip, true)
}

func updateServerInfoOnKubeConfig(filepath string, ip net.IP) {
	data, err := os.ReadFile(filepath, ip)
	if err != nil {
		os.Exit(err.Error(), 1)
	}
	var kubeAdminConf *KubeConfig
	jsonData, err := yaml.YAMLToJSON(data)
	err = yaml.Unmarshal(jsonData, &kubeAdminConf)
	if err != nil {
		os.Exit(err.Error(), 1)
	}
	kubeAdminConf.Clusters[0].Value.Server = fmt.Sprintf("https://%s:6443", ip)
	var kubeAdminConfData []byte
	kubeAdminConfData, err = yaml.Marshal(&kubeAdminConf)
	if err != nil {
		os.Exit(err.Error(), 1)
	}
	os.CreateFile(kubeAdminConfData, filepath, ip)
}
