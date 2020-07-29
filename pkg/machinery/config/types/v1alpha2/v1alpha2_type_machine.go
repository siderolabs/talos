package v1alpha2

import (
	"github.com/talos-systems/crypto/x509"
)

// MachineManifestV1Alpha1 represents a machine manifest.
type MachineManifestV1Alpha1 struct {
	Type    string `yaml:"type,omitempty"`
	Debug   bool   `yaml:"debug,omitempty"`
	Persist bool   `yaml:"persist,omitempty"`
	API     struct {
		Endpoint string `yaml:"endpoint,omitempty"`
		Auth     struct {
			Token string `yaml:"token"`
			PKI   struct {
				CA       *x509.PEMEncodedCertificateAndKey `yaml:"ca"`
				Identity struct {
					SANS []string `yaml:"sans"`
				} `yaml:"identity"`
			} `yaml:"pki"`
		} `yaml:"auth,omitempty"`
	} `yaml:"api,omitempty"`
	Kernel struct {
		Sysctls map[string]string `yaml:"sysctls,omitempty"`
		Args    []string          `yaml:"args,omitempty"`
	} `yaml:"kernel,omitempty"`
}
