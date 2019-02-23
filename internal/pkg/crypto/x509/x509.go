/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

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
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"io/ioutil"
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

// PEMEncodedCertificateAndKey represents the PEM encoded certificate and
// private key pair.
type PEMEncodedCertificateAndKey struct {
	Crt []byte
	Key []byte
}

// Options is the functional options struct.
type Options struct {
	Organization       string
	SignatureAlgorithm x509.SignatureAlgorithm
	IPAddresses        []net.IP
	DNSNames           []string
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

// DNSNames sets the value for the DNS Names in Subject Alternate Name of
// the certificate.
func DNSNames(o []string) Option {
	return func(opts *Options) {
		opts.DNSNames = o
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
		DNSNames:           []string{},
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
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
			x509.ExtKeyUsageClientAuth,
		},
		IPAddresses: opts.IPAddresses,
		DNSNames:    opts.DNSNames,
	}

	if opts.RSA {
		crt.SignatureAlgorithm = x509.SHA512WithRSA
		return rsaCertificateAuthority(crt, opts)
	}

	return ecdsaCertificateAuthority(crt)
}

// NewCertificateSigningRequest creates a CSR. If the IPAddresses or DNSNames options are not
// specified, the CSR will be generated with the default values set in
// NewDefaultOptions.
func NewCertificateSigningRequest(key *ecdsa.PrivateKey, setters ...Option) (csr *CertificateSigningRequest, err error) {
	opts := NewDefaultOptions(setters...)

	template := &x509.CertificateRequest{
		SignatureAlgorithm: opts.SignatureAlgorithm,
		IPAddresses:        opts.IPAddresses,
		DNSNames:           opts.DNSNames,
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
		DNSNames:    csr.DNSNames,
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

// NewCertificateFromCSRBytes creates a signed certificate using the provided
// certificate, key, and CSR.
func NewCertificateFromCSRBytes(ca, key, csr []byte, setters ...Option) (crt *Certificate, err error) {
	caPemBlock, _ := pem.Decode(ca)
	if caPemBlock == nil {
		return nil, fmt.Errorf("decode PEM: %v", err)
	}
	caCrt, err := x509.ParseCertificate(caPemBlock.Bytes)
	if err != nil {
		return
	}
	keyPemBlock, _ := pem.Decode(key)
	if keyPemBlock == nil {
		return nil, fmt.Errorf("decode PEM: %v", err)
	}
	caKey, err := x509.ParseECPrivateKey(keyPemBlock.Bytes)
	if err != nil {
		return
	}
	csrPemBlock, _ := pem.Decode(csr)
	if csrPemBlock == nil {
		return
	}
	request, err := x509.ParseCertificateRequest(csrPemBlock.Bytes)
	if err != nil {
		return
	}
	crt, err = NewCertificateFromCSR(caCrt, caKey, request, setters...)
	if err != nil {
		return
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

// NewCertificateAndKeyFromFiles initializes and returns a
// PEMEncodedCertificateAndKey from the path to a crt and key.
func NewCertificateAndKeyFromFiles(crt, key string) (p *PEMEncodedCertificateAndKey, err error) {
	p = &PEMEncodedCertificateAndKey{}

	crtBytes, err := ioutil.ReadFile(crt)
	if err != nil {
		return
	}
	p.Crt = crtBytes

	keyBytes, err := ioutil.ReadFile(key)
	if err != nil {
		return
	}
	p.Key = keyBytes

	return p, nil
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
