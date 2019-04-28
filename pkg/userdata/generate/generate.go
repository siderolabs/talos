/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package generate

import (
	"bytes"
	stdlibx509 "crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"math/rand"
	"net"
	"strings"
	"text/template"

	"github.com/talos-systems/talos/pkg/crypto/x509"
)

// CertStrings holds the string representation of a certificate and key.
type CertStrings struct {
	Crt string
	Key string
}

// Input holds info about certs, ips, and node type.
type Input struct {
	Certs         *Certs
	MasterIPs     []string
	Index         int
	ClusterName   string
	ServiceDomain string
	PodNet        []string
	ServiceNet    []string
	Endpoints     string
	KubeadmTokens *KubeadmTokens
	TrustdInfo    *TrustdInfo
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
	Username string
	Password string
}

// RandomString returns a string of length n.
func RandomString(n int) string {
	var letter = []rune("abcdefghijklmnopqrstuvwxy0123456789")

	b := make([]rune, n)
	for i := range b {
		b[i] = letter[rand.Intn(len(letter))]
	}
	return string(b)
}

// NewInput generates the sensitive data required to generate all userdata
// types.
// nolint: gocyclo
func NewInput(clustername string, masterIPs []string) (input *Input, err error) {
	kubeadmTokens := &KubeadmTokens{
		BootstrapToken: RandomString(6) + "." + RandomString(16),
		CertKey:        RandomString(26),
	}

	trustdInfo := &TrustdInfo{
		Username: RandomString(14),
		Password: RandomString(24),
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
	ips := []net.IP{net.ParseIP("127.0.0.1")}
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
		Certs:         certs,
		MasterIPs:     masterIPs,
		PodNet:        []string{"10.244.0.0/16"},
		ServiceNet:    []string{"10.96.0.0/12"},
		ServiceDomain: "cluster.local",
		ClusterName:   clustername,
		Endpoints:     strings.Join(masterIPs[1:], ", "),
		KubeadmTokens: kubeadmTokens,
		TrustdInfo:    trustdInfo,
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

	ud, err := renderTemplate(in, template)
	if err != nil {
		return "", err
	}

	return ud, nil
}

// renderTemplate will output a templated string.
func renderTemplate(in *Input, udTemplate string) (string, error) {
	templ := template.Must(template.New("udTemplate").Parse(udTemplate))
	var buf bytes.Buffer
	if err := templ.Execute(&buf, in); err != nil {
		return "", err
	}
	return buf.String(), nil
}
