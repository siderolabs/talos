package userdata

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"

	"github.com/autonomy/talos/src/initramfs/pkg/crypto/x509"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"
	kubeadmapi "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"
	kubeadmscheme "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/scheme"
	kubeadmutil "k8s.io/kubernetes/cmd/kubeadm/app/util"
	configutil "k8s.io/kubernetes/cmd/kubeadm/app/util/config"

	yaml "gopkg.in/yaml.v2"
)

// Env represents a set of environment variables.
type Env = map[string]string

// UserData represents the user data.
type UserData struct {
	Version    string      `yaml:"version"`
	Security   *Security   `yaml:"security"`
	Networking *Networking `yaml:"networking"`
	Services   *Services   `yaml:"services"`
	Files      []*File     `yaml:"files"`
	Debug      bool        `yaml:"debug"`
	Env        Env         `yaml:"env,omitempty"`
	Install    *Install    `yaml:"install,omitempty"`
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
	Init    *Init    `yaml:"init"`
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

// Install represents the installation options for preparing a node
type Install struct {
	DataDevice string `yaml:"datadevice,omitempty"`
	RootDevice string `yaml:"rootdevice"`
	Wipe       bool   `yaml:"wipe"`
	RootFSURL  string `yaml:"rootfsurl"`
}

// Init describes the configuration of the init service.
type Init struct {
	ContainerRuntime string `yaml:"containerRuntime,omitempty"`
	CNI              string `yaml:"cni,omitempty"`
}

// Kubelet describes the configuration of the kubelet service.
type Kubelet struct {
	CommonServiceOptions `yaml:",inline"`
}

// Kubeadm describes the set of configuration options available for kubeadm.
type Kubeadm struct {
	CommonServiceOptions `yaml:",inline"`

	Configuration runtime.Object `yaml:"configuration"`
	bootstrap     bool
	controlPlane  bool
}

// MarshalYAML implements the yaml.Marshaler interface.
func (kdm *Kubeadm) MarshalYAML() (interface{}, error) {
	var aux struct {
		Configuration string `yaml:"configuration,omitempty"`
	}

	b, err := configutil.MarshalKubeadmConfigObject(kdm.Configuration)
	if err != nil {
		return nil, err
	}

	gvks, err := kubeadmutil.GroupVersionKindsFromBytes(b)
	if err != nil {
		return nil, err
	}

	if kubeadmutil.GroupVersionKindsHasInitConfiguration(gvks...) {
		kdm.bootstrap = true
	}
	if kubeadmutil.GroupVersionKindsHasJoinConfiguration(gvks...) {
		kdm.bootstrap = false
	}

	aux.Configuration = string(b)

	return aux, nil
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (kdm *Kubeadm) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var aux struct {
		ContainerRuntime string `yaml:"containerRuntime,omitempty"`
		Configuration    string `yaml:"configuration,omitempty"`
	}

	if err := unmarshal(&aux); err != nil {
		return err
	}

	b := []byte(aux.Configuration)

	gvks, err := kubeadmutil.GroupVersionKindsFromBytes(b)
	if err != nil {
		return err
	}

	if kubeadmutil.GroupVersionKindsHasInitConfiguration(gvks...) {
		// Since the ClusterConfiguration is embedded in the InitConfiguration
		// struct, it is required to (un)marshal it a special way. The kubeadm
		// API exposes one function (MarshalKubeadmConfigObject) to handle the
		// marshaling, but does not yet have that convenience for
		// unmarshaling.
		cfg, err := configutil.BytesToInternalConfig(b)
		if err != nil {
			return err
		}
		kdm.Configuration = cfg
		kdm.bootstrap = true
	}
	if kubeadmutil.GroupVersionKindsHasJoinConfiguration(gvks...) {
		cfg, err := kubeadmutil.UnmarshalFromYamlForCodecs(b, kubeadmapi.SchemeGroupVersion, kubeadmscheme.Codecs)
		if err != nil {
			return err
		}
		kdm.Configuration = cfg
		kdm.bootstrap = false
		joinConfiguration := cfg.(*kubeadm.JoinConfiguration)
		kdm.controlPlane = joinConfiguration.ControlPlane
	}

	return nil
}

// Trustd describes the configuration of the Root of Trust (RoT) service. The
// username and password are used by master nodes, and worker nodes. The master
// nodes use them to authenticate clients, while the workers use them to
// authenticate as a client. The endpoints should only be specified in the
// worker user data, and should include all master nodes participating as a RoT.
type Trustd struct {
	CommonServiceOptions `yaml:",inline"`

	Username  string   `yaml:"username"`
	Password  string   `yaml:"password"`
	Endpoints []string `yaml:"endpoints,omitempty"`
}

// OSD describes the configuration of the osd service.
type OSD struct {
	CommonServiceOptions `yaml:",inline"`
}

// Proxyd describes the configuration of the proxyd service.
type Proxyd struct {
	CommonServiceOptions `yaml:",inline"`
}

// Blockd describes the configuration of the blockd service.
type Blockd struct {
	CommonServiceOptions `yaml:",inline"`
}

// CRT describes the configuration of the container runtime service.
type CRT struct {
	CommonServiceOptions `yaml:",inline"`
}

// CommonServiceOptions represents the set of options common to all services.
type CommonServiceOptions struct {
	Image string `yaml:"image,omitempty"`
	Env   Env    `yaml:"env,omitempty"`
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

// IsBootstrap indicates if the current kubeadm configuration is a master init
// configuration.
func (data *UserData) IsBootstrap() bool {
	return data.Services.Kubeadm.bootstrap
}

// IsControlPlane indicates if the current kubeadm configuration is a worker
// acting as a master.
func (data *UserData) IsControlPlane() bool {
	return data.Services.Kubeadm.controlPlane
}

// IsMaster indicates if the current kubeadm configuration is a master
// configuration.
func (data *UserData) IsMaster() bool {
	return data.Services.Kubeadm.bootstrap || data.Services.Kubeadm.controlPlane
}

// IsWorker indicates if the current kubeadm configuration is a worker
// configuration.
func (data *UserData) IsWorker() bool {
	return !data.IsMaster()
}
