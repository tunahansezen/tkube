package model

import (
	"github.com/cloudflare/cfssl/config"
	"time"
)

type CA struct {
	Signing Signing `json:"signing"`
}

type Signing struct {
	Default  Default  `json:"default"`
	Profiles Profiles `json:"profiles"`
}

type Default struct {
	Expiry string `json:"expiry"`
}

type Profiles struct {
	Kubernetes Kube `json:"kubernetes"`
}

type Kube struct {
	Usages []string `json:"usages"`
	Expiry string   `json:"expiry"`
}

func DefaultKubernetesCA() *config.Signing {
	return &config.Signing{
		Default: &config.SigningProfile{
			Expiry: 87000 * time.Hour,
		},
		Profiles: map[string]*config.SigningProfile{
			"kubernetes": {
				Usage:  []string{"signing", "key encipherment", "server auth", "client auth"},
				Expiry: 87000 * time.Hour,
			},
		},
	}
}
