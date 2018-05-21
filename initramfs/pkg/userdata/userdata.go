package userdata

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/autonomy/dianemo/initramfs/cmd/init/pkg/constants"
	"github.com/autonomy/dianemo/initramfs/cmd/init/pkg/kernel"
	"github.com/fullsailor/pkcs7"
	yaml "gopkg.in/yaml.v2"
)

const (
	// AWSUserDataEndpoint is the local EC2 endpoint for the user data.
	AWSUserDataEndpoint = "http://169.254.169.254/latest/user-data"
	// AWSPKCS7Endpoint is the local EC2 endpoint for the PKCS7 signature.
	AWSPKCS7Endpoint = "http://169.254.169.254/latest/dynamic/instance-identity/pkcs7"
	// AWSPublicCertificate is the AWS public certificate for the regions
	// provided by an AWS account.
	AWSPublicCertificate = `-----BEGIN CERTIFICATE-----
MIIC7TCCAq0CCQCWukjZ5V4aZzAJBgcqhkjOOAQDMFwxCzAJBgNVBAYTAlVTMRkw
FwYDVQQIExBXYXNoaW5ndG9uIFN0YXRlMRAwDgYDVQQHEwdTZWF0dGxlMSAwHgYD
VQQKExdBbWF6b24gV2ViIFNlcnZpY2VzIExMQzAeFw0xMjAxMDUxMjU2MTJaFw0z
ODAxMDUxMjU2MTJaMFwxCzAJBgNVBAYTAlVTMRkwFwYDVQQIExBXYXNoaW5ndG9u
IFN0YXRlMRAwDgYDVQQHEwdTZWF0dGxlMSAwHgYDVQQKExdBbWF6b24gV2ViIFNl
cnZpY2VzIExMQzCCAbcwggEsBgcqhkjOOAQBMIIBHwKBgQCjkvcS2bb1VQ4yt/5e
ih5OO6kK/n1Lzllr7D8ZwtQP8fOEpp5E2ng+D6Ud1Z1gYipr58Kj3nssSNpI6bX3
VyIQzK7wLclnd/YozqNNmgIyZecN7EglK9ITHJLP+x8FtUpt3QbyYXJdmVMegN6P
hviYt5JH/nYl4hh3Pa1HJdskgQIVALVJ3ER11+Ko4tP6nwvHwh6+ERYRAoGBAI1j
k+tkqMVHuAFcvAGKocTgsjJem6/5qomzJuKDmbJNu9Qxw3rAotXau8Qe+MBcJl/U
hhy1KHVpCGl9fueQ2s6IL0CaO/buycU1CiYQk40KNHCcHfNiZbdlx1E9rpUp7bnF
lRa2v1ntMX3caRVDdbtPEWmdxSCYsYFDk4mZrOLBA4GEAAKBgEbmeve5f8LIE/Gf
MNmP9CM5eovQOGx5ho8WqD+aTebs+k2tn92BBPqeZqpWRa5P/+jrdKml1qx4llHW
MXrs3IgIb6+hUIB+S8dz8/mmO0bpr76RoZVCXYab2CZedFut7qc3WUH9+EUAH5mw
vSeDCOUMYQR7R9LINYwouHIziqQYMAkGByqGSM44BAMDLwAwLAIUWXBlk40xTwSw
7HX32MxXYruse9ACFBNGmdX2ZBrVNGrN9N2f6ROk0k9K
-----END CERTIFICATE-----`
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
	ExtraArgs    map[string]string `json:"extraArgs,omitempty"`
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
func Download() (data UserData, err error) {
	var url string

	arguments, err := kernel.ParseProcCmdline()
	if err != nil {
		return data, fmt.Errorf("parse kernel parameters: %s", err.Error())
	}
	url, ok := arguments[constants.KernelParamUserData]
	if !ok {
		if IsEC2() {
			url = AWSUserDataEndpoint
			goto L
		}

		return data, fmt.Errorf("no user data was found")
	}

L:

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

// IsEC2 uses the EC2 PKCS7 signature to verify the instance by validating it
// against the appropriate AWS public certificate. See
// https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/instance-identity-documents.html
func IsEC2() (b bool) {
	resp, err := http.Get(AWSPKCS7Endpoint)
	if err != nil {
		return
	}
	// nolint: errcheck
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("failed to download PKCS7 signature: %d\n", resp.StatusCode)
		return
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
		return
	}

	data = append([]byte("-----BEGIN PKCS7-----\n"), data...)
	data = append(data, []byte("\n-----END PKCS7-----\n")...)

	pemBlock, _ := pem.Decode(data)
	if pemBlock == nil {
		log.Println("failed to decode PEM block")
		return
	}

	p7, err := pkcs7.Parse(pemBlock.Bytes)
	if err != nil {
		log.Printf("failed to parse PKCS7 signature: %v\n", err)
		return
	}

	pemBlock, _ = pem.Decode([]byte(AWSPublicCertificate))
	if pemBlock == nil {
		log.Println("failed to decode PEM block")
		return
	}

	certificate, err := x509.ParseCertificate(pemBlock.Bytes)
	if err != nil {
		log.Printf("failed to parse X509 certificate: %v\n", err)
		return
	}

	p7.Certificates = []*x509.Certificate{certificate}

	err = p7.Verify()
	if err != nil {
		log.Printf("failed to verify PKCS7 signature: %v", err)
		return
	}

	b = true

	return b
}
