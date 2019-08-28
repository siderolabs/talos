/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package generate

import (
	"bufio"
	"bytes"
	"crypto/rand"
	stdlibx509 "crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"text/template"

	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/crypto/x509"
	tnet "github.com/talos-systems/talos/pkg/net"
)

// DefaultIPv4PodNet is the network to be used for kubernetes Pods when using IPv4-based master nodes
const DefaultIPv4PodNet = "10.244.0.0/16"

// DefaultIPv4ServiceNet is the network to be used for kubernetes Services when using IPv4-based master nodes
const DefaultIPv4ServiceNet = "10.96.0.0/12"

// DefaultIPv6PodNet is the network to be used for kubernetes Pods when using IPv6-based master nodes
const DefaultIPv6PodNet = "fc00:db8:10::/56"

// DefaultIPv6ServiceNet is the network to be used for kubernetes Services when using IPv6-based master nodes
const DefaultIPv6ServiceNet = "fc00:db8:20::/112"

// CertStrings holds the string representation of a certificate and key.
type CertStrings struct {
	Crt string
	Key string
}

// Input holds info about certs, ips, and node type.
type Input struct {
	Certs                     *Certs
	MasterIPs                 []string
	AdditionalSubjectAltNames []string

	ClusterName       string
	ServiceDomain     string
	PodNet            []string
	ServiceNet        []string
	KubernetesVersion string
	KubeadmTokens     *KubeadmTokens
	TrustdInfo        *TrustdInfo

	//
	// Runtime variables
	//

	// Index is the index of the current master
	Index int

	// IP is the IP address of the current master
	IP net.IP
}

// Endpoints returns the formatted set of Master IP addresses
func (i *Input) Endpoints() (out string) {
	if i == nil || len(i.MasterIPs) < 1 {
		panic("cannot Endpoints without any Master IPs")
	}
	for index, addr := range i.MasterIPs {
		if index > 0 {
			out += ", "
		}
		out += fmt.Sprintf(`"%s"`, addr)
	}
	return
}

// GetControlPlaneEndpoint returns the formatted host:port of the first master node
func (i *Input) GetControlPlaneEndpoint(port string) string {

	if i == nil || len(i.MasterIPs) < 1 {
		panic("cannot GetControlPlaneEndpoint without any Master IPs")
	}

	// Each master after the first should reference the next-lower master index.
	// Thus, master-2 references master-1 and master-3 references master-2.
	refMaster := 0
	if i.Index > 1 {
		refMaster = i.Index - 1
	}

	if port == "" {
		return tnet.FormatAddress(i.MasterIPs[refMaster])
	}
	return net.JoinHostPort(i.MasterIPs[refMaster], port)
}

// GetAPIServerSANs returns the formatted list of Subject Alt Name addresses for the API Server
func (i *Input) GetAPIServerSANs() []string {

	var list = []string{"127.0.0.1", "::1"}
	list = append(list, i.MasterIPs...)
	list = append(list, i.AdditionalSubjectAltNames...)

	return list
}

// Certs holds the base64 encoded keys and certificates.
type Certs struct {
	AdminCert string
	AdminKey  string
	OsCert    string
	OsKey     string
	K8sCert   string
	K8sKey    string
}

// KubeadmTokens holds the senesitve kubeadm data.
type KubeadmTokens struct {
	BootstrapToken string
	CertKey        string
}

// TrustdInfo holds the trustd credentials.
type TrustdInfo struct {
	Token string
}

// randBytes returns a random string consisting of the characters in
// validBootstrapTokenChars, with the length customized by the parameter
func randBytes(length int) (string, error) {

	// validBootstrapTokenChars defines the characters a bootstrap token can consist of
	const validBootstrapTokenChars = "0123456789abcdefghijklmnopqrstuvwxyz"

	// len("0123456789abcdefghijklmnopqrstuvwxyz") = 36 which doesn't evenly divide
	// the possible values of a byte: 256 mod 36 = 4. Discard any random bytes we
	// read that are >= 252 so the bytes we evenly divide the character set.
	const maxByteValue = 252

	var (
		b     byte
		err   error
		token = make([]byte, length)
	)

	reader := bufio.NewReaderSize(rand.Reader, length*2)
	for i := range token {
		for {
			if b, err = reader.ReadByte(); err != nil {
				return "", err
			}
			if b < maxByteValue {
				break
			}
		}
		token[i] = validBootstrapTokenChars[int(b)%len(validBootstrapTokenChars)]
	}

	return string(token), err
}

//genToken will generate a token of the format abc.123 (like kubeadm/trustd), where the length of the first string (before the dot)
//and length of the second string (after dot) are specified as inputs
func genToken(lenFirst int, lenSecond int) (string, error) {

	var err error
	var tokenTemp = make([]string, 2)

	tokenTemp[0], err = randBytes(lenFirst)
	if err != nil {
		return "", err
	}
	tokenTemp[1], err = randBytes(lenSecond)
	if err != nil {
		return "", err
	}

	return tokenTemp[0] + "." + tokenTemp[1], nil
}

func isIPv6(addrs ...string) bool {
	for _, a := range addrs {
		if ip := net.ParseIP(a); ip != nil {
			if ip.To4() == nil {
				return true
			}
		}
	}
	return false
}

// NewInput generates the sensitive data required to generate all userdata
// types.
// nolint: dupl,gocyclo
func NewInput(clustername string, masterIPs []string) (input *Input, err error) {

	var loopbackIP, podNet, serviceNet string

	if isIPv6(masterIPs...) {
		loopbackIP = "::1"
		podNet = DefaultIPv6PodNet
		serviceNet = DefaultIPv6ServiceNet
	} else {
		loopbackIP = "127.0.0.1"
		podNet = DefaultIPv4PodNet
		serviceNet = DefaultIPv4ServiceNet
	}

	//Gen trustd token strings
	kubeadmBootstrapToken, err := genToken(6, 16)
	if err != nil {
		return nil, err
	}

	//TODO: Can be dropped
	//Gen kubeadm cert key
	kubeadmCertKey, err := randBytes(26)
	if err != nil {
		return nil, err
	}

	//Gen trustd token strings
	trustdToken, err := genToken(6, 16)
	if err != nil {
		return nil, err
	}

	kubeadmTokens := &KubeadmTokens{
		BootstrapToken: kubeadmBootstrapToken,
		CertKey:        kubeadmCertKey,
	}

	trustdInfo := &TrustdInfo{
		Token: trustdToken,
	}

	// Generate Kubernetes CA.
	opts := []x509.Option{x509.RSA(true), x509.Organization("talos-k8s")}
	k8sCert, err := x509.NewSelfSignedCertificateAuthority(opts...)
	if err != nil {
		return nil, err
	}

	// Generate Talos CA.
	opts = []x509.Option{x509.RSA(false), x509.Organization("talos-os")}
	osCert, err := x509.NewSelfSignedCertificateAuthority(opts...)
	if err != nil {
		return nil, err
	}

	// Generate the admin talosconfig.
	adminKey, err := x509.NewKey()
	if err != nil {
		return nil, err
	}
	pemBlock, _ := pem.Decode(adminKey.KeyPEM)
	if pemBlock == nil {
		return nil, errors.New("failed to decode admin key pem")
	}
	adminKeyEC, err := stdlibx509.ParseECPrivateKey(pemBlock.Bytes)
	if err != nil {
		return nil, err
	}
	ips := []net.IP{net.ParseIP(loopbackIP)}
	opts = []x509.Option{x509.IPAddresses(ips)}
	csr, err := x509.NewCertificateSigningRequest(adminKeyEC, opts...)
	if err != nil {
		return nil, err
	}
	csrPemBlock, _ := pem.Decode(csr.X509CertificateRequestPEM)
	if csrPemBlock == nil {
		return nil, errors.New("failed to decode csr pem")
	}
	ccsr, err := stdlibx509.ParseCertificateRequest(csrPemBlock.Bytes)
	if err != nil {
		return nil, err
	}
	caPemBlock, _ := pem.Decode(osCert.CrtPEM)
	if caPemBlock == nil {
		return nil, errors.New("failed to decode ca cert pem")
	}
	caCrt, err := stdlibx509.ParseCertificate(caPemBlock.Bytes)
	if err != nil {
		return nil, err
	}
	caKeyPemBlock, _ := pem.Decode(osCert.KeyPEM)
	if caKeyPemBlock == nil {
		return nil, errors.New("failed to decode ca key pem")
	}
	caKey, err := stdlibx509.ParseECPrivateKey(caKeyPemBlock.Bytes)
	if err != nil {
		return nil, err
	}
	adminCrt, err := x509.NewCertificateFromCSR(caCrt, caKey, ccsr)
	if err != nil {
		return nil, err
	}

	certs := &Certs{
		AdminCert: base64.StdEncoding.EncodeToString(adminCrt.X509CertificatePEM),
		AdminKey:  base64.StdEncoding.EncodeToString(adminKey.KeyPEM),
		OsCert:    base64.StdEncoding.EncodeToString(osCert.CrtPEM),
		OsKey:     base64.StdEncoding.EncodeToString(osCert.KeyPEM),
		K8sCert:   base64.StdEncoding.EncodeToString(k8sCert.CrtPEM),
		K8sKey:    base64.StdEncoding.EncodeToString(k8sCert.KeyPEM),
	}

	input = &Input{
		Certs:             certs,
		MasterIPs:         masterIPs,
		PodNet:            []string{podNet},
		ServiceNet:        []string{serviceNet},
		ServiceDomain:     "cluster.local",
		ClusterName:       clustername,
		KubernetesVersion: constants.KubernetesVersion,
		KubeadmTokens:     kubeadmTokens,
		TrustdInfo:        trustdInfo,
	}

	return input, nil
}

// Type represents a userdata type.
type Type int

const (
	// TypeInit indicates a userdata type should correspond to the kubeadm
	// InitConfiguration type.
	TypeInit Type = iota
	// TypeControlPlane indicates a userdata type should correspond to the
	// kubeadm JoinConfiguration type that has the ControlPlane field
	// defined.
	TypeControlPlane
	// TypeJoin indicates a userdata type should correspond to the kubeadm
	// JoinConfiguration type.
	TypeJoin
)

// Sring returns the string representation of Type.
func (t Type) String() string {
	return [...]string{"Init", "ControlPlane", "Join"}[t]
}

// Userdata returns the talos userdata for a given node type.
func Userdata(t Type, in *Input) (string, error) {
	var template string
	switch t {
	case TypeInit:
		template = initTempl
	case TypeControlPlane:
		template = controlPlaneTempl
	case TypeJoin:
		template = workerTempl
	default:
		return "", errors.New("failed to determine userdata type to generate")
	}

	var err error
	var ud string

	ud, err = renderTemplate(in, template)
	if err != nil {
		return "", err
	}

	// TODO: We cant implement this currently because of
	// issues with kubeadm dependency mismatch between
	// talos and clusterapi//kubebuilder.
	// We should figure out way we can work around/through
	// this
	/*
		// Create an actual userdata struct from the
		// generated data so we can call validate
		// and ensure we are providing proper data
		data := &userdata.UserData{}
		if err = yaml.Unmarshal([]byte(ud), data); err != nil {
			return "", err
		}

		if err = data.Validate(); err != nil {
			return "", err
		}
	*/

	return ud, nil
}

// renderTemplate will output a templated string.
func renderTemplate(in *Input, udTemplate string) (string, error) {
	// So we can have a simple add func
	funcs := template.FuncMap{"add": add}

	templ := template.Must(template.New("udTemplate").Funcs(funcs).Parse(udTemplate))
	var buf bytes.Buffer
	if err := templ.Execute(&buf, in); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func add(a, b int) int {
	return a + b
}
