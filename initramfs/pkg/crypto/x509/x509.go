package x509

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"math/big"
	"net"
	"strings"
	"time"
)

// CertificateAuthority represents a CA.
type CertificateAuthority struct {
	Crt    *x509.Certificate
	CrtPEM []byte
	Key    interface{}
	KeyPEM []byte
}

// Key represents an ECDSA private key.
type Key struct {
	keyEC  *ecdsa.PrivateKey
	KeyPEM []byte
}

// Certificate represents an X.509 certificate.
type Certificate struct {
	X509Certificate    *x509.Certificate
	X509CertificatePEM []byte
}

// CertificateSigningRequest represents a CSR.
type CertificateSigningRequest struct {
	X509CertificateRequest    *x509.CertificateRequest
	X509CertificateRequestPEM []byte
}

// KeyPair represents a certificate and key pair.
type KeyPair struct {
	*tls.Certificate
}

// Options is the functional options struct.
type Options struct {
	Organization       string
	SignatureAlgorithm x509.SignatureAlgorithm
	IPAddresses        []net.IP
	Bits               int
	RSA                bool
	NotAfter           time.Time
}

// Option is the functional option func.
type Option func(*Options)

// Organization sets the subject organization of the certificate.
func Organization(o string) Option {
	return func(opts *Options) {
		opts.Organization = o
	}
}

// SignatureAlgorithm sets the hash algorithm used to sign the SSL certificate.
func SignatureAlgorithm(o x509.SignatureAlgorithm) Option {
	return func(opts *Options) {
		opts.SignatureAlgorithm = o
	}
}

// IPAddresses sets the value for the IP addresses in Subject Alternate Name of
// the certificate.
func IPAddresses(o []net.IP) Option {
	return func(opts *Options) {
		opts.IPAddresses = o
	}
}

// Bits sets the bit size of the RSA key pair.
func Bits(o int) Option {
	return func(opts *Options) {
		opts.Bits = o
	}
}

// RSA sets a flag for indicating that the requested operation should be
// performed under the context of RSA instead of the default ECDSA.
func RSA(o bool) Option {
	return func(opts *Options) {
		opts.RSA = o
	}
}

// NotAfter sets the validity bound describing when a certificate expires.
func NotAfter(o time.Time) Option {
	return func(opts *Options) {
		opts.NotAfter = o
	}
}

// NewDefaultOptions initializes the Options struct with default values.
func NewDefaultOptions(setters ...Option) *Options {
	opts := &Options{
		SignatureAlgorithm: x509.ECDSAWithSHA512,
		IPAddresses:        []net.IP{},
		Bits:               4096,
		RSA:                false,
		NotAfter:           time.Now().Add(8760 * time.Hour),
	}

	for _, setter := range setters {
		setter(opts)
	}

	return opts
}

// NewSerialNumber generates a random serial number for an X.509 certificate.
func NewSerialNumber() (sn *big.Int, err error) {
	snLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	sn, err = rand.Int(rand.Reader, snLimit)
	if err != nil {
		return
	}

	return sn, nil
}

// NewSelfSignedCertificateAuthority creates a self-signed CA configured for
// server and client authentication.
func NewSelfSignedCertificateAuthority(setters ...Option) (ca *CertificateAuthority, err error) {
	opts := NewDefaultOptions(setters...)

	serialNumber, err := NewSerialNumber()
	if err != nil {
		return nil, err
	}

	crt := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{opts.Organization},
		},
		SignatureAlgorithm:    opts.SignatureAlgorithm,
		NotBefore:             time.Now(),
		NotAfter:              opts.NotAfter,
		BasicConstraintsValid: true,
		IsCA:     true,
		KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
			x509.ExtKeyUsageClientAuth,
		},
		IPAddresses: opts.IPAddresses,
	}

	if opts.RSA {
		crt.SignatureAlgorithm = x509.SHA512WithRSA
		return rsaCertificateAuthority(crt, opts)
	}

	return ecdsaCertificateAuthority(crt)
}

// NewCertificateSigningRequest creates a CSR. If the IPAddresses option is not
// specified, the CSR will be generated with the default value set in
// NewDefaultOptions.
func NewCertificateSigningRequest(key *ecdsa.PrivateKey, setters ...Option) (csr *CertificateSigningRequest, err error) {
	opts := NewDefaultOptions(setters...)

	template := &x509.CertificateRequest{
		SignatureAlgorithm: opts.SignatureAlgorithm,
		IPAddresses:        opts.IPAddresses,
	}

	csrBytes, err := x509.CreateCertificateRequest(rand.Reader, template, key)
	if err != nil {
		return
	}
	csrPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE REQUEST",
		Bytes: csrBytes,
	})

	csr = &CertificateSigningRequest{
		X509CertificateRequest:    template,
		X509CertificateRequestPEM: csrPEM,
	}

	return csr, err
}

// NewKey generates an ECDSA private key.
func NewKey() (key *Key, err error) {
	keyEC, err := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	if err != nil {
		return
	}

	keyBytes, err := x509.MarshalECPrivateKey(keyEC)
	if err != nil {
		return
	}

	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: keyBytes,
	})

	key = &Key{
		keyEC:  keyEC,
		KeyPEM: keyPEM,
	}

	return key, nil
}

// NewCertificateFromCSR creates and signs X.509 certificate using the provided
// CSR.
func NewCertificateFromCSR(ca *x509.Certificate, key *ecdsa.PrivateKey, csr *x509.CertificateRequest, setters ...Option) (crt *Certificate, err error) {
	opts := NewDefaultOptions(setters...)
	serialNumber, err := NewSerialNumber()
	if err != nil {
		return nil, err
	}

	template := &x509.Certificate{
		Signature:          csr.Signature,
		SignatureAlgorithm: csr.SignatureAlgorithm,

		PublicKeyAlgorithm: csr.PublicKeyAlgorithm,
		PublicKey:          csr.PublicKey,

		SerialNumber: serialNumber,
		Issuer:       ca.Subject,
		Subject:      csr.Subject,
		NotBefore:    time.Now(),
		NotAfter:     opts.NotAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
			x509.ExtKeyUsageClientAuth,
		},
		IPAddresses: csr.IPAddresses,
	}

	crtDER, err := x509.CreateCertificate(rand.Reader, template, ca, csr.PublicKey, key)
	if err != nil {
		return
	}

	x509Certificate, err := x509.ParseCertificate(crtDER)
	if err != nil {
		return
	}

	crtPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: crtDER,
	})

	crt = &Certificate{
		X509Certificate:    x509Certificate,
		X509CertificatePEM: crtPEM,
	}

	return crt, nil
}

// NewKeyPair generates a certificate signed by the provided CA, and an ECDSA
// private key. The certifcate and private key are then used to create an
// tls.X509KeyPair.
func NewKeyPair(ca *x509.Certificate, key *ecdsa.PrivateKey, setters ...Option) (keypair *KeyPair, err error) {
	csr, err := NewCertificateSigningRequest(key, setters...)
	if err != nil {
		return
	}
	k, err := NewKey()
	if err != nil {
		return
	}
	crt, err := NewCertificateFromCSR(ca, key, csr.X509CertificateRequest, setters...)
	if err != nil {
		return
	}

	x509KeyPair, err := tls.X509KeyPair(crt.X509CertificatePEM, k.KeyPEM)
	if err != nil {
		return
	}

	keypair = &KeyPair{
		&x509KeyPair,
	}

	return keypair, nil
}

// Hash calculates the SHA-256 hash of the Subject Public Key Information (SPKI)
// object in an x509 certificate (in DER encoding). It returns the full hash as
// a hex encoded string (suitable for passing to Set.Allow). See
// https://github.com/kubernetes/kubernetes/blob/f557e0f7e3ee9089769ed3f03187fdd4acbb9ac1/cmd/kubeadm/app/util/pubkeypin/pubkeypin.go
func Hash(crt *x509.Certificate) string {
	spkiHash := sha256.Sum256(crt.RawSubjectPublicKeyInfo)
	return "sha256" + ":" + strings.ToLower(hex.EncodeToString(spkiHash[:]))
}

func rsaCertificateAuthority(template *x509.Certificate, opts *Options) (ca *CertificateAuthority, err error) {
	key, e := rsa.GenerateKey(rand.Reader, opts.Bits)
	if e != nil {
		return
	}
	keyBytes := x509.MarshalPKCS1PrivateKey(key)
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: keyBytes,
	})
	crtDER, e := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if e != nil {
		return
	}
	crt, err := x509.ParseCertificate(crtDER)
	if err != nil {
		return
	}
	crtPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: crtDER,
	})

	ca = &CertificateAuthority{
		Crt:    crt,
		CrtPEM: crtPEM,
		Key:    key,
		KeyPEM: keyPEM,
	}

	return ca, nil
}

func ecdsaCertificateAuthority(template *x509.Certificate) (ca *CertificateAuthority, err error) {
	key, e := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	if e != nil {
		return
	}
	keyBytes, e := x509.MarshalECPrivateKey(key)
	if e != nil {
		return
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: keyBytes,
	})
	crtDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return
	}
	crt, err := x509.ParseCertificate(crtDER)
	if err != nil {
		return
	}
	crtPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: crtDER,
	})

	ca = &CertificateAuthority{
		Crt:    crt,
		CrtPEM: crtPEM,
		Key:    key,
		KeyPEM: keyPEM,
	}

	return ca, nil
}
