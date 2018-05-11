package userdata

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/autonomy/dianemo/initramfs/cmd/init/pkg/constants"
	"github.com/autonomy/dianemo/initramfs/cmd/init/pkg/kernel"
	yaml "gopkg.in/yaml.v2"
)

// UserData represents the user data.
type UserData struct {
	Version    string      `yaml:"version"`
	OS         *OS         `yaml:"os"`
	Kubernetes *Kubernetes `yaml:"kubernetes,omitempty"`
}

// OS represents the operating system specific configuration options.
type OS struct {
	Network  *Network  `yaml:"network,omitempty"`
	Security *Security `yaml:"security"`
}

// Network represents the operating system networking specific configuration
// options.
type Network struct {
	Nameservers []string `yaml:"nameservers,omitempty"`
}

// Security represents the operating system security specific configuration
// options.
type Security struct {
	CA       *CertificateAndKeyPaths `yaml:"ca"`
	Identity *CertificateAndKeyPaths `yaml:"identity"`
}

// CertificateAndKeyPaths represents the paths to the certificate and private
// key.
type CertificateAndKeyPaths struct {
	Crt string `yaml:"crt"`
	Key string `yaml:"key"`
}

// Kubernetes represents the Kubernetes specific configuration options.
type Kubernetes struct {
	CA                         *CertificateAndKeyPaths `yaml:"ca,omitempty"`
	Token                      string                  `yaml:"token"`
	Join                       bool                    `yaml:"join,omitempty"`
	APIServer                  string                  `yaml:"apiServer,omitempty"`
	NodeName                   string                  `yaml:"nodeName,omitempty"`
	Labels                     map[string]string       `yaml:"labels,omitempty"`
	ContainerRuntime           string                  `yaml:"containerRuntime,omitempty"`
	DiscoveryTokenCACertHashes []string                `yaml:"discoveryTokenCACertHashes,omitempty"`
}

// Download initializes a UserData struct from a remote URL.
func Download() (data UserData, err error) {
	arguments, err := kernel.ParseProcCmdline()
	if err != nil {
		return data, fmt.Errorf("parse kernel parameters: %s", err.Error())
	}
	url, ok := arguments[constants.KernelParamUserData]
	if !ok {
		return
	}

	resp, err := http.Get(url)
	if err != nil {
		return
	}
	// nolint: errcheck
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return
	}

	dataBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	if err != nil {
		return data, fmt.Errorf("download user data: %s", err.Error())
	}

	if err := yaml.Unmarshal(dataBytes, &data); err != nil {
		return data, fmt.Errorf("decode user data: %s", err.Error())
	}

	return data, nil
}
