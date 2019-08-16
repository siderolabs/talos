/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package generate

import (
	"bufio"
	"crypto/rand"
	stdlibx509 "crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"net"
	"strings"

	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/crypto/x509"
	"github.com/talos-systems/talos/pkg/userdata/token"
)

// CertStrings holds the string representation of a certificate and key.
type CertStrings struct {
	Crt string
	Key string
}

// Input holds info about certs, ips, and node type.
type Input struct {
	Certs             *Certs
	MasterIPs         []string
	Index             int
	ClusterName       string
	ServiceDomain     string
	PodNet            []string
	ServiceNet        []string
	Endpoints         string
	KubernetesVersion string
	KubeadmTokens     *KubeadmTokens
	TrustdInfo        *TrustdInfo
	InitToken         *token.Token
	IP                net.IP
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

// NewInput generates the sensitive data required to generate all userdata
// types.
// nolint: gocyclo
func NewInput(clustername string, masterIPs []string) (input *Input, err error) {

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

	// Create the init token
	tok, err := token.NewToken()
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
		Certs:             certs,
		MasterIPs:         masterIPs,
		PodNet:            []string{"10.244.0.0/16"},
		ServiceNet:        []string{"10.96.0.0/12"},
		ServiceDomain:     "cluster.local",
		ClusterName:       clustername,
		Endpoints:         strings.Join(masterIPs, ", "),
		KubernetesVersion: constants.KubernetesVersion,
		KubeadmTokens:     kubeadmTokens,
		TrustdInfo:        trustdInfo,
		InitToken:         tok,
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
	switch t {
	case TypeInit:
		return initUd(in)
	case TypeControlPlane:
		return controlPlaneUd(in)
	case TypeJoin:
		return workerUd(in)
	default:
	}
	return "", errors.New("failed to determine userdata type to generate")
}
