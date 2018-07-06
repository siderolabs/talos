package userdata

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"

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
type Network struct{}

// Security represents the operating system security specific configuration
// options.
type Security struct {
	CA           *PEMEncodedCertificateAndKey `yaml:"ca"`
	Identity     *PEMEncodedCertificateAndKey `yaml:"identity"`
	RootsOfTrust *RootsOfTrust                `yaml:"rootsOfTrust"`
}

// RootsOfTrust describes the configuration of the Root of Trust (RoT) services.
// The username and password are used by master nodes, and worker nodes. The
// master nodes use them to authentication clients, while the workers use them
// to authenticate as a client. The endpoints should only be specified in the
// worker user data, and should include all master nodes participating as a RoT.
type RootsOfTrust struct {
	Generate  bool     `yaml:"generate,omitempty"`
	Username  string   `yaml:"username,omitempty"`
	Password  string   `yaml:"password,omitempty"`
	Endpoints []string `yaml:"endpoints,omitempty"`
}

// PEMEncodedCertificateAndKey represents the PEM encoded certificate and
// private key pair.
type PEMEncodedCertificateAndKey struct {
	Crt []byte
	Key []byte
}

// Kubernetes represents the Kubernetes specific configuration options.
type Kubernetes struct {
	CA               *PEMEncodedCertificateAndKey `yaml:"ca,omitempty"`
	Init             bool                         `yaml:"init,omitempty"`
	Kubelet          Kubelet                      `yaml:"kubelet,omitempty"`
	ContainerRuntime string                       `yaml:"containerRuntime,omitempty"`
	Configuration    string                       `yaml:"configuration,omitempty"`
}

// Kubelet describes the set of configuration options available for the kubelet.
type Kubelet struct {
	Labels       map[string]string `yaml:"labels,omitempty"`
	FeatureGates map[string]string `yaml:"featureGates,omitempty"`
	ExtraArgs    map[string]string `yaml:"extraArgs,omitempty"`
}

// UnmarshalYAML implements the yaml.Unmarshaler interface for
// PEMEncodedCertificateAndKey. It is expected that the Crt and Key are a base64
// encoded string in the YAML file. This function decodes the strings into byte
// slices.
func (p *PEMEncodedCertificateAndKey) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var aux struct {
		Crt string `yaml:"crt"`
		Key string `yaml:"key"`
	}
	if err := unmarshal(&aux); err != nil {
		return err
	}

	decodedCrt, err := base64.StdEncoding.DecodeString(aux.Crt)
	if err != nil {
		return err
	}

	decodedKey, err := base64.StdEncoding.DecodeString(aux.Key)
	if err != nil {
		return err
	}

	p.Crt = decodedCrt
	p.Key = decodedKey

	return nil
}

// MarshalYAML implements the yaml.Marshaler interface for
// PEMEncodedCertificateAndKey. It is expected that the Crt and Key are a base64
// encoded string in the YAML file. This function encodes the byte slices into
// strings
func (p *PEMEncodedCertificateAndKey) MarshalYAML() (interface{}, error) {
	var aux struct {
		Crt string `yaml:"crt"`
		Key string `yaml:"key"`
	}

	aux.Crt = base64.StdEncoding.EncodeToString(p.Crt)
	aux.Key = base64.StdEncoding.EncodeToString(p.Key)

	return aux, nil
}

// Download initializes a UserData struct from a remote URL.
func Download(url string) (data UserData, err error) {
	resp, err := http.Get(url)
	if err != nil {
		return
	}
	// nolint: errcheck
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return data, fmt.Errorf("download user data: %d", resp.StatusCode)
	}

	dataBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	if err != nil {
		return data, fmt.Errorf("read user data: %s", err.Error())
	}

	if err := yaml.Unmarshal(dataBytes, &data); err != nil {
		return data, fmt.Errorf("unmarshal user data: %s", err.Error())
	}

	return data, nil
}

// Open is a convenience function that reads the user data from disk, and
// unmarshals it.
func Open(p string) (data *UserData, err error) {
	fileBytes, err := ioutil.ReadFile(p)
	if err != nil {
		return nil, fmt.Errorf("read user data: %v", err)
	}

	data = &UserData{}
	if err = yaml.Unmarshal(fileBytes, data); err != nil {
		return nil, fmt.Errorf("unmarshal user data: %v", err)
	}

	return data, nil
}
