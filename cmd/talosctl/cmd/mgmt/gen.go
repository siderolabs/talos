// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mgmt

import (
	stdlibx509 "crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"net"
	"path"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/talos-systems/crypto/x509"

	"github.com/talos-systems/talos/pkg/cli"
)

var (
	ca           string
	csr          string
	caHours      int
	crtHours     int
	ip           string
	key          string
	name         string
	organization string
	rsa          bool
)

// genCmd represents the gen command.
var genCmd = &cobra.Command{
	Use:   "gen",
	Short: "Generate CAs, certificates, and private keys",
	Long:  ``,
}

// caCmd represents the gen ca command.
var caCmd = &cobra.Command{
	Use:   "ca",
	Short: "Generates a self-signed X.509 certificate authority",
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := []x509.Option{x509.RSA(rsa)}
		if organization != "" {
			opts = append(opts, x509.Organization(organization))
		}

		opts = append(opts, x509.NotAfter(time.Now().Add(time.Duration(caHours)*time.Hour)))

		ca, err := x509.NewSelfSignedCertificateAuthority(opts...)
		if err != nil {
			return fmt.Errorf("error generating CA: %w", err)
		}

		if err := ioutil.WriteFile(organization+".crt", ca.CrtPEM, 0o600); err != nil {
			return fmt.Errorf("error writing CA certificate: %w", err)
		}

		if err := ioutil.WriteFile(organization+".sha256", []byte(x509.Hash(ca.Crt)), 0o600); err != nil {
			return fmt.Errorf("error writing certificate hash: %w", err)
		}

		if err := ioutil.WriteFile(organization+".key", ca.KeyPEM, 0o600); err != nil {
			return fmt.Errorf("error writing key: %w", err)
		}

		return nil
	},
}

// keyCmd represents the gen key command.
var keyCmd = &cobra.Command{
	Use:   "key",
	Short: "Generates an Ed25519 private key",
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		key, err := x509.NewEd25519Key()
		if err != nil {
			return fmt.Errorf("error generating key: %w", err)
		}

		if err := ioutil.WriteFile(name+".key", key.PrivateKeyPEM, 0o600); err != nil {
			return fmt.Errorf("error writing key: %w", err)
		}

		return nil
	},
}

// csrCmd represents the gen csr command.
var csrCmd = &cobra.Command{
	Use:   "csr",
	Short: "Generates a CSR using an Ed25519 private key",
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		keyBytes, err := ioutil.ReadFile(key)
		if err != nil {
			return fmt.Errorf("error reading key: %s", err)
		}

		pemBlock, _ := pem.Decode(keyBytes)
		if pemBlock == nil {
			return fmt.Errorf("error decoding PEM: %s", err)
		}

		keyEC, err := stdlibx509.ParsePKCS8PrivateKey(pemBlock.Bytes)
		if err != nil {
			return fmt.Errorf("error parsing ECDSA key: %s", err)
		}

		opts := []x509.Option{}

		parsed := net.ParseIP(ip)
		if parsed == nil {
			return fmt.Errorf("invalid IP: %s", ip)
		}

		ips := []net.IP{parsed}
		opts = append(opts, x509.IPAddresses(ips))
		opts = append(opts, x509.NotAfter(time.Now().Add(time.Duration(crtHours)*time.Hour)))

		csr, err := x509.NewCertificateSigningRequest(keyEC, opts...)
		if err != nil {
			return fmt.Errorf("error generating CSR: %s", err)
		}

		if err := ioutil.WriteFile(strings.TrimSuffix(key, path.Ext(key))+".csr", csr.X509CertificateRequestPEM, 0o600); err != nil {
			return fmt.Errorf("error writing CSR: %s", err)
		}

		return nil
	},
}

// crtCmd represents the gen crt command.
var crtCmd = &cobra.Command{
	Use:   "crt",
	Short: "Generates an X.509 Ed25519 certificate",
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		caBytes, err := ioutil.ReadFile(ca + ".crt")
		if err != nil {
			return fmt.Errorf("error reading CA cert: %s", err)
		}

		caPemBlock, _ := pem.Decode(caBytes)
		if caPemBlock == nil {
			return fmt.Errorf("error decoding cert PEM: %s", err)
		}

		caCrt, err := stdlibx509.ParseCertificate(caPemBlock.Bytes)
		if err != nil {
			return fmt.Errorf("error parsing cert: %s", err)
		}

		keyBytes, err := ioutil.ReadFile(ca + ".key")
		if err != nil {
			return fmt.Errorf("error reading key file: %s", err)
		}

		keyPemBlock, _ := pem.Decode(keyBytes)
		if keyPemBlock == nil {
			return fmt.Errorf("error decoding key PEM: %s", err)
		}

		caKey, err := stdlibx509.ParsePKCS8PrivateKey(keyPemBlock.Bytes)
		if err != nil {
			return fmt.Errorf("error parsing EC key: %s", err)
		}

		csrBytes, err := ioutil.ReadFile(csr)
		if err != nil {
			return fmt.Errorf("error reading CSR: %s", err)
		}

		csrPemBlock, _ := pem.Decode(csrBytes)
		if csrPemBlock == nil {
			return fmt.Errorf("error parsing CSR PEM: %s", err)
		}

		ccsr, err := stdlibx509.ParseCertificateRequest(csrPemBlock.Bytes)
		if err != nil {
			return fmt.Errorf("error parsing CSR: %s", err)
		}

		signedCrt, err := x509.NewCertificateFromCSR(caCrt, caKey, ccsr, x509.NotAfter(time.Now().Add(time.Duration(crtHours)*time.Hour)))
		if err != nil {
			return fmt.Errorf("error signing certificate: %s", err)
		}

		if err = ioutil.WriteFile(name+".crt", signedCrt.X509CertificatePEM, 0o600); err != nil {
			return fmt.Errorf("error writing certificate: %s", err)
		}

		return err
	},
}

// keypairCmd represents the gen keypair command.
var keypairCmd = &cobra.Command{
	Use:   "keypair",
	Short: "Generates an X.509 Ed25519 key pair",
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := []x509.Option{}
		if ip != "" {
			parsed := net.ParseIP(ip)
			if parsed == nil {
				return fmt.Errorf("invalid IP: %s", ip)
			}
			ips := []net.IP{parsed}
			opts = append(opts, x509.IPAddresses(ips))
		}
		if organization != "" {
			opts = append(opts, x509.Organization(organization))
		}
		ca, err := x509.NewSelfSignedCertificateAuthority(opts...)
		if err != nil {
			return fmt.Errorf("error generating CA: %s", err)
		}
		if err := ioutil.WriteFile(organization+".crt", ca.CrtPEM, 0o600); err != nil {
			return fmt.Errorf("error writing certificate: %s", err)
		}
		if err := ioutil.WriteFile(organization+".key", ca.KeyPEM, 0o600); err != nil {
			return fmt.Errorf("error writing key: %s", err)
		}

		return nil
	},
}

func init() {
	// Certificate Authorities
	caCmd.Flags().StringVar(&organization, "organization", "", "X.509 distinguished name for the Organization")
	cli.Should(cobra.MarkFlagRequired(caCmd.Flags(), "organization"))
	caCmd.Flags().IntVar(&caHours, "hours", 87600, "the hours from now on which the certificate validity period ends")
	caCmd.Flags().BoolVar(&rsa, "rsa", false, "generate in RSA format")
	// Keys
	keyCmd.Flags().StringVar(&name, "name", "", "the basename of the generated file")
	cli.Should(cobra.MarkFlagRequired(keyCmd.Flags(), "name"))
	// Certificates
	crtCmd.Flags().StringVar(&name, "name", "", "the basename of the generated file")
	cli.Should(cobra.MarkFlagRequired(crtCmd.Flags(), "name"))
	crtCmd.Flags().StringVar(&ca, "ca", "", "path to the PEM encoded CERTIFICATE")
	cli.Should(cobra.MarkFlagRequired(crtCmd.Flags(), "ca"))
	crtCmd.Flags().StringVar(&csr, "csr", "", "path to the PEM encoded CERTIFICATE REQUEST")
	cli.Should(cobra.MarkFlagRequired(crtCmd.Flags(), "csr"))
	crtCmd.Flags().IntVar(&crtHours, "hours", 24, "the hours from now on which the certificate validity period ends")
	// Keypairs
	keypairCmd.Flags().StringVar(&ip, "ip", "", "generate the certificate for this IP address")
	keypairCmd.Flags().StringVar(&organization, "organization", "", "X.509 distinguished name for the Organization")
	cli.Should(cobra.MarkFlagRequired(keypairCmd.Flags(), "organization"))
	// Certificate Signing Requests
	csrCmd.Flags().StringVar(&key, "key", "", "path to the PEM encoded EC or RSA PRIVATE KEY")
	cli.Should(cobra.MarkFlagRequired(csrCmd.Flags(), "key"))
	csrCmd.Flags().StringVar(&ip, "ip", "", "generate the certificate for this IP address")
	cli.Should(cobra.MarkFlagRequired(csrCmd.Flags(), "ip"))

	genCmd.AddCommand(caCmd, keypairCmd, keyCmd, csrCmd, crtCmd)
	addCommand(genCmd)
}
