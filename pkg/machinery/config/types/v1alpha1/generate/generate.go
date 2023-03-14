// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package generate provides Talos machine configuration generation and client config generation.
//
// Please see the example for more information on using this package.
package generate

import (
	"bufio"
	"crypto/rand"
	stdx509 "crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/siderolabs/crypto/x509"
	tnet "github.com/siderolabs/net"

	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/cis"
	v1alpha1 "github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/role"
)

// Config returns the talos config for a given node type.
func Config(t machine.Type, in *Input) (*v1alpha1.Config, error) {
	switch t {
	case machine.TypeInit:
		return initUd(in)
	case machine.TypeControlPlane:
		return controlPlaneUd(in)
	case machine.TypeWorker:
		return workerUd(in)
	case machine.TypeUnknown:
		fallthrough
	default:
		return nil, errors.New("failed to determine config type to generate")
	}
}

// Input holds info about certs, ips, and node type.
//
//nolint:maligned
type Input struct {
	Certs           *Certs
	VersionContract *config.VersionContract

	// ControlplaneEndpoint is the canonical address of the kubernetes control
	// plane.  It can be a DNS name, the IP address of a load balancer, or
	// (default) the IP address of the first controlplane node.  It is NOT
	// multi-valued.  It may optionally specify the port.
	ControlPlaneEndpoint string

	LocalAPIServerPort int

	AdditionalSubjectAltNames []string
	AdditionalMachineCertSANs []string

	ClusterID         string
	ClusterName       string
	ClusterSecret     string
	ServiceDomain     string
	PodNet            []string
	ServiceNet        []string
	KubernetesVersion string
	Secrets           *Secrets
	TrustdInfo        *TrustdInfo

	ExternalEtcd bool

	InstallDisk            string
	InstallImage           string
	InstallEphemeralSize   string
	InstallExtraKernelArgs []string

	NetworkConfigOptions []v1alpha1.NetworkConfigOption
	CNIConfig            *v1alpha1.CNIConfig

	RegistryMirrors            map[string]*v1alpha1.RegistryMirrorConfig
	RegistryConfig             map[string]*v1alpha1.RegistryConfig
	MachineDisks               []*v1alpha1.MachineDisk
	SystemDiskEncryptionConfig *v1alpha1.SystemDiskEncryptionConfig
	Sysctls                    map[string]string

	Debug                          bool
	Persist                        bool
	AllowSchedulingOnControlPlanes bool
	DiscoveryEnabled               bool
}

// GetAPIServerEndpoint returns the formatted host:port of the API server endpoint.
func (i *Input) GetAPIServerEndpoint(port string) string {
	if port == "" {
		return tnet.FormatAddress(i.ControlPlaneEndpoint)
	}

	return net.JoinHostPort(i.ControlPlaneEndpoint, port)
}

// GetControlPlaneEndpoint returns the formatted host:port of the canonical controlplane address, defaulting to the first controlplane IP.
func (i *Input) GetControlPlaneEndpoint() string {
	if i == nil || i.ControlPlaneEndpoint == "" {
		panic("cannot GetControlPlaneEndpoint without any controlplane IPs")
	}

	return i.ControlPlaneEndpoint
}

// GetAPIServerSANs returns the formatted list of Subject Alt Name addresses for the API Server.
func (i *Input) GetAPIServerSANs() []string {
	list := []string{}

	endpointURL, err := url.Parse(i.ControlPlaneEndpoint)
	if err == nil {
		list = append(list, endpointURL.Hostname())
	}

	list = append(list, i.AdditionalSubjectAltNames...)

	return list
}

// Certs holds the base64 encoded keys and certificates.
type Certs struct {
	Admin             *x509.PEMEncodedCertificateAndKey `json:"Admin,omitempty" yaml:",omitempty"`
	Etcd              *x509.PEMEncodedCertificateAndKey `json:"Etcd"`
	K8s               *x509.PEMEncodedCertificateAndKey `json:"K8s"`
	K8sAggregator     *x509.PEMEncodedCertificateAndKey `json:"K8sAggregator"`
	K8sServiceAccount *x509.PEMEncodedKey               `json:"K8sServiceAccount"`
	OS                *x509.PEMEncodedCertificateAndKey `json:"OS"`
}

// Cluster holds Talos cluster-wide secrets.
type Cluster struct {
	ID     string `json:"Id"`
	Secret string `json:"Secret"`
}

// Secrets holds the sensitive kubeadm data.
type Secrets struct {
	BootstrapToken            string `json:"BootstrapToken"`
	AESCBCEncryptionSecret    string `json:"AESCBCEncryptionSecret,omitempty" yaml:",omitempty"`
	SecretboxEncryptionSecret string `json:"SecretboxEncryptionSecret,omitempty" yaml:",omitempty"`
}

// TrustdInfo holds the trustd credentials.
type TrustdInfo struct {
	Token string `json:"Token"`
}

// SecretsBundle holds trustd, kubeadm and certs information.
type SecretsBundle struct {
	Clock      Clock       `yaml:"-" json:"-"`
	Cluster    *Cluster    `json:"Cluster"`
	Secrets    *Secrets    `json:"Secrets"`
	TrustdInfo *TrustdInfo `json:"TrustdInfo"`
	Certs      *Certs      `json:"Certs"`
}

// Clock system clock.
type Clock interface {
	Now() time.Time
}

// SystemClock is a real system clock, but the time returned can be made fixed.
type SystemClock struct {
	Time time.Time
}

// NewClock creates new SystemClock.
func NewClock() *SystemClock {
	return &SystemClock{}
}

// Now implements Clock.
func (c *SystemClock) Now() time.Time {
	if c.Time.IsZero() {
		return time.Now()
	}

	return c.Time
}

// SetFixedTimestamp freezes the clock by setting a timestamp.
func (c *SystemClock) SetFixedTimestamp(t time.Time) {
	c.Time = t
}

// NewSecretsBundle creates secrets bundle generating all secrets or reading from the input options if provided.
func NewSecretsBundle(clock Clock, opts ...GenOption) (*SecretsBundle, error) {
	options := DefaultGenOptions()

	for _, opt := range opts {
		if err := opt(&options); err != nil {
			return nil, err
		}
	}

	// if secrets bundle is provided via gen options, just return it
	if options.Secrets != nil {
		return options.Secrets, nil
	}

	bundle := SecretsBundle{
		Clock: clock,
	}

	err := populateSecretsBundle(options.VersionContract, &bundle)
	if err != nil {
		return nil, err
	}

	return &bundle, nil
}

// NewSecretsBundleFromKubernetesPKI creates secrets bundle by reading the contents
// of a Kubernetes PKI directory (typically `/etc/kubernetes/pki`) and using the provided bootstrapToken as input.
//
//nolint:gocyclo
func NewSecretsBundleFromKubernetesPKI(pkiDir, bootstrapToken string, versionContract *config.VersionContract) (*SecretsBundle, error) {
	dirStat, err := os.Stat(pkiDir)
	if err != nil {
		return nil, err
	}

	if !dirStat.IsDir() {
		return nil, fmt.Errorf("%q is not a directory", pkiDir)
	}

	var (
		ca           *x509.PEMEncodedCertificateAndKey
		etcdCA       *x509.PEMEncodedCertificateAndKey
		aggregatorCA *x509.PEMEncodedCertificateAndKey
		sa           *x509.PEMEncodedKey
	)

	ca, err = x509.NewCertificateAndKeyFromFiles(filepath.Join(pkiDir, "ca.crt"), filepath.Join(pkiDir, "ca.key"))
	if err != nil {
		return nil, err
	}

	err = validatePEMEncodedCertificateAndKey(ca)
	if err != nil {
		return nil, err
	}

	etcdDir := filepath.Join(pkiDir, "etcd")

	etcdCA, err = x509.NewCertificateAndKeyFromFiles(filepath.Join(etcdDir, "ca.crt"), filepath.Join(etcdDir, "ca.key"))
	if err != nil {
		return nil, err
	}

	err = validatePEMEncodedCertificateAndKey(etcdCA)
	if err != nil {
		return nil, err
	}

	aggregatorCACrtPath := filepath.Join(pkiDir, "front-proxy-ca.crt")
	_, err = os.Stat(aggregatorCACrtPath)

	aggregatorCAFound := err == nil
	if aggregatorCAFound && !versionContract.SupportsAggregatorCA() {
		return nil, fmt.Errorf("aggregator CA found in pki dir but is not supported by the requested version")
	}

	if versionContract.SupportsAggregatorCA() {
		aggregatorCA, err = x509.NewCertificateAndKeyFromFiles(aggregatorCACrtPath, filepath.Join(pkiDir, "front-proxy-ca.key"))
		if err != nil {
			return nil, err
		}

		err = validatePEMEncodedCertificateAndKey(aggregatorCA)
		if err != nil {
			return nil, err
		}
	}

	saKeyPath := filepath.Join(pkiDir, "sa.key")
	_, err = os.Stat(saKeyPath)

	saKeyFound := err == nil
	if saKeyFound && !versionContract.SupportsServiceAccount() {
		return nil, fmt.Errorf("service account key found in pki dir but is not supported by the requested version")
	}

	if versionContract.SupportsServiceAccount() {
		var saBytes []byte

		saBytes, err = os.ReadFile(filepath.Join(pkiDir, "sa.key"))
		if err != nil {
			return nil, err
		}

		sa = &x509.PEMEncodedKey{
			Key: saBytes,
		}

		_, err = sa.GetKey()
		if err != nil {
			return nil, err
		}
	}

	bundle := SecretsBundle{
		Secrets: &Secrets{
			BootstrapToken: bootstrapToken,
		},
		Certs: &Certs{
			Etcd:              etcdCA,
			K8s:               ca,
			K8sAggregator:     aggregatorCA,
			K8sServiceAccount: sa,
		},
	}

	err = populateSecretsBundle(versionContract, &bundle)
	if err != nil {
		return nil, err
	}

	return &bundle, nil
}

// populateSecretsBundle fills all the missing fields in the secrets bundle.
//
//nolint:gocyclo,cyclop
func populateSecretsBundle(versionContract *config.VersionContract, bundle *SecretsBundle) error {
	if bundle.Clock == nil {
		bundle.Clock = NewClock()
	}

	if bundle.Certs == nil {
		bundle.Certs = &Certs{}
	}

	if bundle.Certs.Etcd == nil {
		etcd, err := NewEtcdCA(bundle.Clock.Now(), versionContract)
		if err != nil {
			return err
		}

		bundle.Certs.Etcd = &x509.PEMEncodedCertificateAndKey{
			Crt: etcd.CrtPEM,
			Key: etcd.KeyPEM,
		}
	}

	if bundle.Certs.K8s == nil {
		kubernetesCA, err := NewKubernetesCA(bundle.Clock.Now(), versionContract)
		if err != nil {
			return err
		}

		bundle.Certs.K8s = &x509.PEMEncodedCertificateAndKey{
			Crt: kubernetesCA.CrtPEM,
			Key: kubernetesCA.KeyPEM,
		}
	}

	if versionContract.SupportsAggregatorCA() && bundle.Certs.K8sAggregator == nil {
		aggregatorCA, err := NewAggregatorCA(bundle.Clock.Now(), versionContract)
		if err != nil {
			return err
		}

		bundle.Certs.K8sAggregator = &x509.PEMEncodedCertificateAndKey{
			Crt: aggregatorCA.CrtPEM,
			Key: aggregatorCA.KeyPEM,
		}
	}

	if versionContract.SupportsServiceAccount() && bundle.Certs.K8sServiceAccount == nil {
		serviceAccount, err := x509.NewECDSAKey()
		if err != nil {
			return err
		}

		bundle.Certs.K8sServiceAccount = &x509.PEMEncodedKey{
			Key: serviceAccount.KeyPEM,
		}
	}

	if bundle.Certs.OS == nil {
		talosCA, err := NewTalosCA(bundle.Clock.Now())
		if err != nil {
			return err
		}

		bundle.Certs.OS = &x509.PEMEncodedCertificateAndKey{
			Crt: talosCA.CrtPEM,
			Key: talosCA.KeyPEM,
		}
	}

	if bundle.Secrets == nil {
		bundle.Secrets = &Secrets{}
	}

	if bundle.Secrets.BootstrapToken == "" {
		token, err := genToken(6, 16)
		if err != nil {
			return err
		}

		bundle.Secrets.BootstrapToken = token
	}

	if versionContract.Greater(config.TalosVersion1_2) {
		if bundle.Secrets.SecretboxEncryptionSecret == "" {
			secretboxEncryptionSecret, err := cis.CreateEncryptionToken()
			if err != nil {
				return err
			}

			bundle.Secrets.SecretboxEncryptionSecret = secretboxEncryptionSecret
		}
	} else {
		if bundle.Secrets.AESCBCEncryptionSecret == "" {
			aesCBCEncryptionSecret, err := cis.CreateEncryptionToken()
			if err != nil {
				return err
			}

			bundle.Secrets.AESCBCEncryptionSecret = aesCBCEncryptionSecret
		}
	}

	if bundle.TrustdInfo == nil {
		bundle.TrustdInfo = &TrustdInfo{}
	}

	if bundle.TrustdInfo.Token == "" {
		token, err := genToken(6, 16)
		if err != nil {
			return err
		}

		bundle.TrustdInfo.Token = token
	}

	if bundle.Cluster == nil {
		bundle.Cluster = &Cluster{}
	}

	if bundle.Cluster.ID == "" {
		clusterID, err := randBytes(constants.DefaultClusterIDSize)
		if err != nil {
			return fmt.Errorf("failed to generate cluster ID: %w", err)
		}

		bundle.Cluster.ID = base64.URLEncoding.EncodeToString(clusterID)
	}

	if bundle.Cluster.Secret == "" {
		clusterSecret, err := randBytes(constants.DefaultClusterSecretSize)
		if err != nil {
			return fmt.Errorf("failed to generate cluster secret: %w", err)
		}

		bundle.Cluster.Secret = base64.StdEncoding.EncodeToString(clusterSecret)
	}

	return nil
}

// NewSecretsBundleFromConfig creates secrets bundle using existing config.
func NewSecretsBundleFromConfig(clock Clock, c config.Provider) *SecretsBundle {
	certs := &Certs{
		K8s:               c.Cluster().CA(),
		K8sAggregator:     c.Cluster().AggregatorCA(),
		K8sServiceAccount: c.Cluster().ServiceAccount(),
		Etcd:              c.Cluster().Etcd().CA(),
		OS:                c.Machine().Security().CA(),
	}

	cluster := &Cluster{
		ID:     c.Cluster().ID(),
		Secret: c.Cluster().Secret(),
	}

	trustd := &TrustdInfo{
		Token: c.Machine().Security().Token(),
	}

	bootstrapToken := fmt.Sprintf(
		"%s.%s",
		c.Cluster().Token().ID(),
		c.Cluster().Token().Secret(),
	)

	secrets := &Secrets{
		AESCBCEncryptionSecret:    c.Cluster().AESCBCEncryptionSecret(),
		SecretboxEncryptionSecret: c.Cluster().SecretboxEncryptionSecret(),
		BootstrapToken:            bootstrapToken,
	}

	return &SecretsBundle{
		Clock:      clock,
		Cluster:    cluster,
		Secrets:    secrets,
		TrustdInfo: trustd,
		Certs:      certs,
	}
}

// NewEtcdCA generates a CA for the Etcd PKI.
func NewEtcdCA(currentTime time.Time, contract *config.VersionContract) (ca *x509.CertificateAuthority, err error) {
	opts := []x509.Option{
		x509.Organization("etcd"),
		x509.NotAfter(currentTime.Add(87600 * time.Hour)),
		x509.NotBefore(currentTime),
	}

	if !contract.SupportsECDSAKeys() {
		opts = append(opts, x509.RSA(true))
	} else {
		if contract.SupportsECDSASHA256() {
			opts = append(opts, x509.ECDSA(true))
		} else {
			opts = append(opts, x509.ECDSASHA512(true))
		}
	}

	return x509.NewSelfSignedCertificateAuthority(opts...)
}

// NewKubernetesCA generates a CA for the Kubernetes PKI.
func NewKubernetesCA(currentTime time.Time, contract *config.VersionContract) (ca *x509.CertificateAuthority, err error) {
	opts := []x509.Option{
		x509.Organization("kubernetes"),
		x509.NotAfter(currentTime.Add(87600 * time.Hour)),
		x509.NotBefore(currentTime),
	}

	if !contract.SupportsECDSAKeys() {
		opts = append(opts, x509.RSA(true))
	} else {
		if contract.SupportsECDSASHA256() {
			opts = append(opts, x509.ECDSA(true))
		} else {
			opts = append(opts, x509.ECDSASHA512(true))
		}
	}

	return x509.NewSelfSignedCertificateAuthority(opts...)
}

// NewAggregatorCA generates a CA for the Kubernetes aggregator/front-proxy.
func NewAggregatorCA(currentTime time.Time, contract *config.VersionContract) (ca *x509.CertificateAuthority, err error) {
	opts := []x509.Option{
		x509.ECDSA(true),
		x509.CommonName("front-proxy"),
		x509.NotAfter(currentTime.Add(87600 * time.Hour)),
		x509.NotBefore(currentTime),
	}

	if contract.SupportsECDSASHA256() {
		opts = append(opts, x509.ECDSA(true))
	} else {
		opts = append(opts, x509.ECDSASHA512(true))
	}

	return x509.NewSelfSignedCertificateAuthority(opts...)
}

// NewTalosCA generates a CA for the Talos PKI.
func NewTalosCA(currentTime time.Time) (ca *x509.CertificateAuthority, err error) {
	opts := []x509.Option{
		x509.Organization("talos"),
		x509.NotAfter(currentTime.Add(87600 * time.Hour)),
		x509.NotBefore(currentTime),
	}

	return x509.NewSelfSignedCertificateAuthority(opts...)
}

// NewAdminCertificateAndKey generates the admin Talos certificate and key.
func NewAdminCertificateAndKey(currentTime time.Time, ca *x509.PEMEncodedCertificateAndKey, roles role.Set, ttl time.Duration) (p *x509.PEMEncodedCertificateAndKey, err error) {
	opts := []x509.Option{
		x509.Organization(roles.Strings()...),
		x509.NotAfter(currentTime.Add(ttl)),
		x509.NotBefore(currentTime),
		x509.KeyUsage(stdx509.KeyUsageDigitalSignature),
		x509.ExtKeyUsage([]stdx509.ExtKeyUsage{stdx509.ExtKeyUsageClientAuth}),
	}

	talosCA, err := x509.NewCertificateAuthorityFromCertificateAndKey(ca)
	if err != nil {
		return nil, err
	}

	keyPair, err := x509.NewKeyPair(talosCA, opts...)
	if err != nil {
		return nil, err
	}

	return x509.NewCertificateAndKeyFromKeyPair(keyPair), nil
}

// NewInput generates the sensitive data required to generate all config
// types.
func NewInput(clustername, endpoint, kubernetesVersion string, secrets *SecretsBundle, opts ...GenOption) (input *Input, err error) {
	options := DefaultGenOptions()

	for _, opt := range opts {
		if err = opt(&options); err != nil {
			return nil, err
		}
	}

	var podNet, serviceNet string

	if addr, addrErr := netip.ParseAddr(endpoint); addrErr == nil && addr.Is6() {
		podNet = constants.DefaultIPv6PodNet
		serviceNet = constants.DefaultIPv6ServiceNet
	} else {
		podNet = constants.DefaultIPv4PodNet
		serviceNet = constants.DefaultIPv4ServiceNet
	}

	secrets.Certs.Admin, err = NewAdminCertificateAndKey(
		secrets.Clock.Now(),
		secrets.Certs.OS,
		options.Roles,
		87600*time.Hour,
	)

	if err != nil {
		return nil, err
	}

	additionalSubjectAltNames := append([]string(nil), options.AdditionalSubjectAltNames...)

	if !options.VersionContract.SupportsDynamicCertSANs() {
		additionalSubjectAltNames = append(additionalSubjectAltNames, options.EndpointList...)
	}

	discoveryEnabled := options.VersionContract.ClusterDiscoveryEnabled()

	if options.DiscoveryEnabled != nil {
		discoveryEnabled = *options.DiscoveryEnabled
	}

	input = &Input{
		Certs:                          secrets.Certs,
		VersionContract:                options.VersionContract,
		ControlPlaneEndpoint:           endpoint,
		LocalAPIServerPort:             options.LocalAPIServerPort,
		PodNet:                         []string{podNet},
		ServiceNet:                     []string{serviceNet},
		ServiceDomain:                  options.DNSDomain,
		ClusterID:                      secrets.Cluster.ID,
		ClusterName:                    clustername,
		ClusterSecret:                  secrets.Cluster.Secret,
		KubernetesVersion:              kubernetesVersion,
		Secrets:                        secrets.Secrets,
		TrustdInfo:                     secrets.TrustdInfo,
		AdditionalSubjectAltNames:      additionalSubjectAltNames,
		AdditionalMachineCertSANs:      additionalSubjectAltNames,
		InstallDisk:                    options.InstallDisk,
		InstallImage:                   options.InstallImage,
		InstallExtraKernelArgs:         options.InstallExtraKernelArgs,
		InstallEphemeralSize:           options.InstallEphemeralSize,
		NetworkConfigOptions:           options.NetworkConfigOptions,
		CNIConfig:                      options.CNIConfig,
		RegistryMirrors:                options.RegistryMirrors,
		RegistryConfig:                 options.RegistryConfig,
		Sysctls:                        options.Sysctls,
		Debug:                          options.Debug,
		Persist:                        options.Persist,
		AllowSchedulingOnControlPlanes: options.AllowSchedulingOnControlPlanes,
		MachineDisks:                   options.MachineDisks,
		SystemDiskEncryptionConfig:     options.SystemDiskEncryptionConfig,
		DiscoveryEnabled:               discoveryEnabled,
	}

	return input, nil
}

// randBootstrapTokenString returns a random string consisting of the characters in
// validBootstrapTokenChars, with the length customized by the parameter.
func randBootstrapTokenString(length int) (string, error) {
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

	tokenTemp[0], err = randBootstrapTokenString(lenFirst)
	if err != nil {
		return "", err
	}

	tokenTemp[1], err = randBootstrapTokenString(lenSecond)
	if err != nil {
		return "", err
	}

	return tokenTemp[0] + "." + tokenTemp[1], nil
}

// emptyIf returns empty string if the 2nd argument is empty string, otherwise returns the first argument.
func emptyIf(str, check string) string {
	if check == "" {
		return ""
	}

	return str
}

func randBytes(size int) ([]byte, error) {
	buf := make([]byte, size)

	n, err := io.ReadFull(rand.Reader, buf)
	if err != nil {
		return nil, fmt.Errorf("failed to read from random generator: %w", err)
	}

	if n != size {
		return nil, fmt.Errorf("failed to generate sufficient number of random bytes (%d != %d)", n, size)
	}

	return buf, nil
}

func validatePEMEncodedCertificateAndKey(certs *x509.PEMEncodedCertificateAndKey) error {
	_, err := certs.GetKey()
	if err != nil {
		return err
	}

	_, err = certs.GetCert()

	return err
}
