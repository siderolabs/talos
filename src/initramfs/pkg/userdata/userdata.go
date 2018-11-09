package userdata

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"

	"github.com/autonomy/dianemo/src/initramfs/pkg/crypto/x509"
	yaml "gopkg.in/yaml.v2"
)

// UserData represents the user data.
type UserData struct {
	Version    string      `yaml:"version"`
	Security   *Security   `yaml:"security"`
	Networking *Networking `yaml:"networking"`
	Services   *Services   `yaml:"services"`
	Files      []*File     `yaml:"files"`
	Debug      bool        `yaml:"debug"`
}

// Security represents the set of options available to configure security.
type Security struct {
	OS         *OSSecurity         `yaml:"os"`
	Kubernetes *KubernetesSecurity `yaml:"kubernetes"`
}

// OSSecurity represents the set of security options specific to the OS.
type OSSecurity struct {
	CA       *x509.PEMEncodedCertificateAndKey `yaml:"ca"`
	Identity *x509.PEMEncodedCertificateAndKey `yaml:"identity"`
}

// KubernetesSecurity represents the set of security options specific to
// Kubernetes.
type KubernetesSecurity struct {
	CA *x509.PEMEncodedCertificateAndKey `yaml:"ca"`
}

// Networking represents the set of options available to configure networking.
type Networking struct {
	OS         struct{} `yaml:"os"`
	Kubernetes struct{} `yaml:"kubernetes"`
}

// Services represents the set of services available to configure.
type Services struct {
	Kubelet *Kubelet `yaml:"kubelet"`
	Kubeadm *Kubeadm `yaml:"kubeadm"`
	Trustd  *Trustd  `yaml:"trustd"`
	Proxyd  *Proxyd  `yaml:"proxyd"`
	Blockd  *Blockd  `yaml:"blockd"`
	OSD     *OSD     `yaml:"osd"`
	CRT     *CRT     `yaml:"crt"`
}

// File represents a files to write to disk.
type File struct {
	Contents    string      `yaml:"contents"`
	Permissions os.FileMode `yaml:"permissions"`
	Path        string      `yaml:"path"`
}

// Kubelet describes the configuration of the kubelet service.
type Kubelet struct {
	Image string `yaml:"image,omitempty"`
}

// Kubeadm describes the set of configuration options available for kubeadm.
type Kubeadm struct {
	Image            string             `yaml:"image,omitempty"`
	ContainerRuntime string             `yaml:"containerRuntime,omitempty"`
	Configuration    string             `yaml:"configuration,omitempty"`
	Init             *InitConfiguration `yaml:"init,omitempty"`
}

// InitConfiguration describes the init strategy.
type InitConfiguration struct {
	Bootstrap      bool     `yaml:"bootstrap"`
	TrustEndpoints []string `yaml:"trustEndpoints,omitempty"`
}

// Trustd describes the configuration of the Root of Trust (RoT) service. The
// username and password are used by master nodes, and worker nodes. The master
// nodes use them to authenticate clients, while the workers use them to
// authenticate as a client. The endpoints should only be specified in the
// worker user data, and should include all master nodes participating as a RoT.
type Trustd struct {
	Image     string   `yaml:"image,omitempty"`
	Username  string   `yaml:"username"`
	Password  string   `yaml:"password"`
	Endpoints []string `yaml:"endpoints,omitempty"`
}

// OSD describes the configuration of the osd service.
type OSD struct {
	Image string `yaml:"image,omitempty"`
}

// Proxyd describes the configuration of the proxyd service.
type Proxyd struct {
	Image string `yaml:"image,omitempty"`
}

// Blockd describes the configuration of the blockd service.
type Blockd struct {
	Image string `yaml:"image,omitempty"`
}

// CRT describes the configuration of the container runtime service.
type CRT struct {
	Image string `yaml:"image,omitempty"`
}

// WriteFiles writes the requested files to disk.
func (data *UserData) WriteFiles() (err error) {
	for _, f := range data.Files {
		p := path.Join("/var", f.Path)
		if err = os.MkdirAll(path.Dir(p), os.ModeDir); err != nil {
			return
		}
		if err = ioutil.WriteFile(p, []byte(f.Contents), f.Permissions); err != nil {
			return
		}
	}

	return nil
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
