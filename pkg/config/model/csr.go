package model

import (
	"github.com/cloudflare/cfssl/csr"
	"net"
)

func DefaultKeyRequest() *csr.KeyRequest {
	return &csr.KeyRequest{
		A: "rsa",
		S: 2048,
	}
}
func DefaultName() *csr.Name {
	return &csr.Name{
		C:  "AU",
		ST: "stateA",
		L:  "cityA",
		O:  "companyA",
		OU: "sectionA",
	}
}

func DefaultKubernetesCSR() *csr.CertificateRequest {
	return &csr.CertificateRequest{
		CN:         "kubernetes",
		KeyRequest: DefaultKeyRequest(),
		Names:      []csr.Name{*DefaultName()},
		CA: &csr.CAConfig{
			Expiry: "8760h",
		},
	}
}

func DefaultEtcdCSR(hosts []net.IP) *csr.CertificateRequest {
	var hostsStr []string
	for _, host := range hosts {
		hostsStr = append(hostsStr, host.String())
	}
	return &csr.CertificateRequest{
		CN:         "etcd",
		Hosts:      append(hostsStr, "127.0.0.1"),
		KeyRequest: DefaultKeyRequest(),
		Names:      []csr.Name{*DefaultName()},
	}
}
