// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package generate

import (
	"bufio"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"net"
	"net/url"
	"time"

	stdlibx509 "crypto/x509"

	"github.com/talos-systems/crypto/x509"

	tnet "github.com/talos-systems/net"

	"github.com/talos-systems/talos/pkg/machinery/config/internal/cis"
	v1alpha1 "github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// Config returns the talos config for a given node type.
// nolint: gocyclo
func Config(t machine.Type, in *Input) (c *v1alpha1.Config, err error) {
	switch t {
	case machine.TypeInit:
		if c, err = initUd(in); err != nil {
			return c, err
		}
	case machine.TypeControlPlane:
		if c, err = controlPlaneUd(in); err != nil {
			return c, err
		}
	case machine.TypeJoin:
		if c, err = workerUd(in); err != nil {
			return c, err
		}
	case machine.TypeUnknown:
		fallthrough
	default:
		return c, errors.New("failed to determine config type to generate")
	}

	return c, nil
}

// Input holds info about certs, ips, and node type.
//
//nolint: maligned
type Input struct {
	Certs *Certs

	// ControlplaneEndpoint is the canonical address of the kubernetes control
	// plane.  It can be a DNS name, the IP address of a load balancer, or
	// (default) the IP address of the first master node.  It is NOT
	// multi-valued.  It may optionally specify the port.
	ControlPlaneEndpoint string

	AdditionalSubjectAltNames []string
	AdditionalMachineCertSANs []string

	ClusterName       string
	Architecture      string
	ServiceDomain     string
	PodNet            []string
	ServiceNet        []string
	KubernetesVersion string
	Secrets           *Secrets
	TrustdInfo        *TrustdInfo

	ExternalEtcd bool

	InstallDisk            string
	InstallImage           string
	InstallExtraKernelArgs []string

	NetworkConfig *v1alpha1.NetworkConfig
	CNIConfig     *v1alpha1.CNIConfig

	RegistryMirrors map[string]*v1alpha1.RegistryMirrorConfig

	Debug   bool
	Persist bool
}

// GetAPIServerEndpoint returns the formatted host:port of the API server endpoint.
func (i *Input) GetAPIServerEndpoint(port string) string {
	if port == "" {
		return tnet.FormatAddress(i.ControlPlaneEndpoint)
	}

	return net.JoinHostPort(i.ControlPlaneEndpoint, port)
}

// GetControlPlaneEndpoint returns the formatted host:port of the canonical controlplane address, defaulting to the first master IP.
func (i *Input) GetControlPlaneEndpoint() string {
	if i == nil || i.ControlPlaneEndpoint == "" {
		panic("cannot GetControlPlaneEndpoint without any Master IPs")
	}

	return i.ControlPlaneEndpoint
}

// GetAPIServerSANs returns the formatted list of Subject Alt Name addresses for the API Server.
func (i *Input) GetAPIServerSANs() []string {
	list := []string{}

	endpointURL, err := url.Parse(i.ControlPlaneEndpoint)
	if err == nil {
		host, _, err := net.SplitHostPort(endpointURL.Host)
		if err == nil {
			list = append(list, host)
		}
	}

	list = append(list, i.AdditionalSubjectAltNames...)

	return list
}

// Certs holds the base64 encoded keys and certificates.
type Certs struct {
	Admin *x509.PEMEncodedCertificateAndKey
	Etcd  *x509.PEMEncodedCertificateAndKey
	K8s   *x509.PEMEncodedCertificateAndKey
	OS    *x509.PEMEncodedCertificateAndKey
}

// Secrets holds the senesitve kubeadm data.
type Secrets struct {
	BootstrapToken         string
	AESCBCEncryptionSecret string
}

// TrustdInfo holds the trustd credentials.
type TrustdInfo struct {
	Token string
}

// NewEtcdCA generates a CA for the Etcd PKI.
func NewEtcdCA() (ca *x509.CertificateAuthority, err error) {
	opts := []x509.Option{
		x509.RSA(true),
		x509.Organization("etcd"),
		x509.NotAfter(time.Now().Add(87600 * time.Hour)),
	}

	return x509.NewSelfSignedCertificateAuthority(opts...)
}

// NewKubernetesCA generates a CA for the Kubernetes PKI.
func NewKubernetesCA() (ca *x509.CertificateAuthority, err error) {
	opts := []x509.Option{
		x509.RSA(true),
		x509.Organization("kubernetes"),
		x509.NotAfter(time.Now().Add(87600 * time.Hour)),
	}

	return x509.NewSelfSignedCertificateAuthority(opts...)
}

// NewTalosCA generates a CA for the Talos PKI.
func NewTalosCA() (ca *x509.CertificateAuthority, err error) {
	opts := []x509.Option{
		x509.RSA(false),
		x509.Organization("talos"),
		x509.NotAfter(time.Now().Add(87600 * time.Hour)),
	}

	return x509.NewSelfSignedCertificateAuthority(opts...)
}

// NewAdminCertificateAndKey generates the admin Talos certifiate and key.
func NewAdminCertificateAndKey(crt, key []byte, loopback string) (p *x509.PEMEncodedCertificateAndKey, err error) {
	ips := []net.IP{net.ParseIP(loopback)}

	opts := []x509.Option{
		x509.IPAddresses(ips),
		x509.NotAfter(time.Now().Add(87600 * time.Hour)),
	}

	caPemBlock, _ := pem.Decode(crt)
	if caPemBlock == nil {
		return nil, errors.New("failed to decode ca cert pem")
	}

	caCrt, err := stdlibx509.ParseCertificate(caPemBlock.Bytes)
	if err != nil {
		return nil, err
	}

	caKeyPemBlock, _ := pem.Decode(key)
	if caKeyPemBlock == nil {
		return nil, errors.New("failed to decode ca key pem")
	}

	caKey, err := stdlibx509.ParsePKCS8PrivateKey(caKeyPemBlock.Bytes)
	if err != nil {
		return nil, err
	}

	return x509.NewCertficateAndKey(caCrt, caKey, opts...)
}

// NewInput generates the sensitive data required to generate all config
// types.
// nolint: dupl,gocyclo
func NewInput(clustername, endpoint, kubernetesVersion string, opts ...GenOption) (input *Input, err error) {
	options := DefaultGenOptions()

	for _, opt := range opts {
		if err = opt(&options); err != nil {
			return nil, err
		}
	}

	var loopback, podNet, serviceNet string

	if tnet.IsIPv6(net.ParseIP(endpoint)) {
		loopback = "::1"
		podNet = constants.DefaultIPv6PodNet
		serviceNet = constants.DefaultIPv6ServiceNet
	} else {
		loopback = "127.0.0.1"
		podNet = constants.DefaultIPv4PodNet
		serviceNet = constants.DefaultIPv4ServiceNet
	}

	// Gen trustd token strings
	kubeadmBootstrapToken, err := genToken(6, 16)
	if err != nil {
		return nil, err
	}

	aescbcEncryptionSecret, err := cis.CreateEncryptionToken()
	if err != nil {
		return nil, err
	}

	// Gen trustd token strings
	trustdToken, err := genToken(6, 16)
	if err != nil {
		return nil, err
	}

	kubeadmTokens := &Secrets{
		BootstrapToken:         kubeadmBootstrapToken,
		AESCBCEncryptionSecret: aescbcEncryptionSecret,
	}

	trustdInfo := &TrustdInfo{
		Token: trustdToken,
	}

	etcdCA, err := NewEtcdCA()
	if err != nil {
		return nil, err
	}

	kubernetesCA, err := NewKubernetesCA()
	if err != nil {
		return nil, err
	}

	talosCA, err := NewTalosCA()
	if err != nil {
		return nil, err
	}

	admin, err := NewAdminCertificateAndKey(talosCA.CrtPEM, talosCA.KeyPEM, loopback)
	if err != nil {
		return nil, err
	}

	certs := &Certs{
		Admin: admin,
		Etcd: &x509.PEMEncodedCertificateAndKey{
			Crt: etcdCA.CrtPEM,
			Key: etcdCA.KeyPEM,
		},
		K8s: &x509.PEMEncodedCertificateAndKey{
			Crt: kubernetesCA.CrtPEM,
			Key: kubernetesCA.KeyPEM,
		},
		OS: &x509.PEMEncodedCertificateAndKey{
			Crt: talosCA.CrtPEM,
			Key: talosCA.KeyPEM,
		},
	}

	var additionalSubjectAltNames []string

	var additionalMachineCertSANs []string

	if len(options.EndpointList) > 0 {
		additionalSubjectAltNames = options.EndpointList
		additionalMachineCertSANs = options.EndpointList
	}

	additionalSubjectAltNames = append(additionalSubjectAltNames, options.AdditionalSubjectAltNames...)

	if options.NetworkConfig == nil {
		options.NetworkConfig = &v1alpha1.NetworkConfig{}
	}

	input = &Input{
		Certs:                     certs,
		ControlPlaneEndpoint:      endpoint,
		PodNet:                    []string{podNet},
		ServiceNet:                []string{serviceNet},
		ServiceDomain:             options.DNSDomain,
		ClusterName:               clustername,
		Architecture:              options.Architecture,
		KubernetesVersion:         kubernetesVersion,
		Secrets:                   kubeadmTokens,
		TrustdInfo:                trustdInfo,
		AdditionalSubjectAltNames: additionalSubjectAltNames,
		AdditionalMachineCertSANs: additionalMachineCertSANs,
		InstallDisk:               options.InstallDisk,
		InstallImage:              options.InstallImage,
		InstallExtraKernelArgs:    options.InstallExtraKernelArgs,
		NetworkConfig:             options.NetworkConfig,
		CNIConfig:                 options.CNIConfig,
		RegistryMirrors:           options.RegistryMirrors,
		Debug:                     options.Debug,
		Persist:                   options.Persist,
	}

	return input, nil
}

// randBytes returns a random string consisting of the characters in
// validBootstrapTokenChars, with the length customized by the parameter.
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

// genToken will generate a token of the format abc.123 (like kubeadm/trustd), where the length of the first string (before the dot)
// and length of the second string (after dot) are specified as inputs.
func genToken(lenFirst, lenSecond int) (string, error) {
	var err error

	tokenTemp := make([]string, 2)

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

// emptyIf returns empty string if the 2nd argument is empty string, otherwise returns the first argumewnt.
func emptyIf(str, check string) string {
	if check == "" {
		return ""
	}

	return str
}
