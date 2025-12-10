package model

type IsoVersions struct {
	Kubernetes string `yaml:"kubernetes"`
	Docker     string `yaml:"docker"`
	Calico     string `yaml:"calico"`
	Etcd       string `yaml:"etcd"`
	Helm       string `yaml:"helm"`
	Helmfile   string `yaml:"helmfile"`
}
