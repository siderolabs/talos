/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package cmd

import (
	stdlibx509 "crypto/x509"
	"encoding/pem"
	"io/ioutil"
	"net"
	"path"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/talos-systems/talos/cmd/osctl/pkg/helpers"
	"github.com/talos-systems/talos/pkg/crypto/x509"
)

// genCmd represents the gen command
var genCmd = &cobra.Command{
	Use:   "gen",
	Short: "Generate CAs, certificates, and private keys",
	Long:  ``,
}

// caCmd represents the gen ca command
var caCmd = &cobra.Command{
	Use:   "ca",
	Short: "Generates a self-signed X.509 certificate authority",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		opts := []x509.Option{x509.RSA(rsa)}
		if organization != "" {
			opts = append(opts, x509.Organization(organization))
		}
		ca, err := x509.NewSelfSignedCertificateAuthority(opts...)
		if err != nil {
			helpers.Fatalf("error generating CA: %s", err)
		}
		if err := ioutil.WriteFile(organization+".crt", ca.CrtPEM, 0400); err != nil {
			helpers.Fatalf("error writing CA certificate: %s", err)
		}
		if err := ioutil.WriteFile(organization+".sha256", []byte(x509.Hash(ca.Crt)), 0400); err != nil {
			helpers.Fatalf("error writing certificate hash: %s", err)
		}
		if err := ioutil.WriteFile(organization+".key", ca.KeyPEM, 0400); err != nil {
			helpers.Fatalf("error writing key: %s", err)
		}
	},
}

// keyCmd represents the gen key command
var keyCmd = &cobra.Command{
	Use:   "key",
	Short: "Generates an ECSDA private key",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		key, err := x509.NewKey()
		if err != nil {
			helpers.Fatalf("error generating key: %s", err)
		}
		if err := ioutil.WriteFile(name+".key", key.KeyPEM, 0400); err != nil {
			helpers.Fatalf("error writing key: %s", err)
		}
	},
}

// csrCmd represents the gen csr command
var csrCmd = &cobra.Command{
	Use:   "csr",
	Short: "Generates a CSR using an ECDSA private key",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		keyBytes, err := ioutil.ReadFile(key)
		if err != nil {
			helpers.Fatalf("error reading key: %s", err)
		}
		pemBlock, _ := pem.Decode(keyBytes)
		if pemBlock == nil {
			helpers.Fatalf("error decoding PEM: %s", err)
		}
		keyEC, err := stdlibx509.ParseECPrivateKey(pemBlock.Bytes)
		if err != nil {
			helpers.Fatalf("error parsing ECDSA key: %s", err)
		}
		opts := []x509.Option{}
		parsed := net.ParseIP(ip)
		if parsed == nil {
			helpers.Fatalf("invalid IP: %s", ip)
		}
		ips := []net.IP{parsed}
		opts = append(opts, x509.IPAddresses(ips))
		opts = append(opts, x509.NotAfter(time.Now().Add(time.Duration(hours)*time.Hour)))
		csr, err := x509.NewCertificateSigningRequest(keyEC, opts...)
		if err != nil {
			helpers.Fatalf("error generating CSR: %s", err)
		}
		if err := ioutil.WriteFile(strings.TrimSuffix(key, path.Ext(key))+".csr", csr.X509CertificateRequestPEM, 0400); err != nil {
			helpers.Fatalf("error writing CSR: %s", err)
		}
	},
}

// crtCmd represents the gen crt command
var crtCmd = &cobra.Command{
	Use:   "crt",
	Short: "Generates an X.509 ECDSA certificate",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		caBytes, err := ioutil.ReadFile(ca + ".crt")
		if err != nil {
			helpers.Fatalf("error reading CA cert: %s", err)
		}
		caPemBlock, _ := pem.Decode(caBytes)
		if caPemBlock == nil {
			helpers.Fatalf("error decoding cert PEM: %s", err)
		}
		caCrt, err := stdlibx509.ParseCertificate(caPemBlock.Bytes)
		if err != nil {
			helpers.Fatalf("error parsing cert: %s", err)
		}
		keyBytes, err := ioutil.ReadFile(ca + ".key")
		if err != nil {
			helpers.Fatalf("error reading key file: %s", err)
		}
		keyPemBlock, _ := pem.Decode(keyBytes)
		if keyPemBlock == nil {
			helpers.Fatalf("error decoding key PEM: %s", err)
		}
		caKey, err := stdlibx509.ParseECPrivateKey(keyPemBlock.Bytes)
		if err != nil {
			helpers.Fatalf("error parsing EC key: %s", err)
		}
		csrBytes, err := ioutil.ReadFile(csr)
		if err != nil {
			helpers.Fatalf("error reading CSR: %s", err)
		}
		csrPemBlock, _ := pem.Decode(csrBytes)
		if csrPemBlock == nil {
			helpers.Fatalf("error parsing CSR PEM: %s", err)
		}
		ccsr, err := stdlibx509.ParseCertificateRequest(csrPemBlock.Bytes)
		if err != nil {
			helpers.Fatalf("error parsing CSR: %s", err)
		}
		signedCrt, err := x509.NewCertificateFromCSR(caCrt, caKey, ccsr, x509.NotAfter(time.Now().Add(time.Duration(hours)*time.Hour)))
		if err != nil {
			helpers.Fatalf("error signing certificate: %s", err)
		}
		if err := ioutil.WriteFile(name+".crt", signedCrt.X509CertificatePEM, 0400); err != nil {
			helpers.Fatalf("error writing certificate: %s", err)
		}
	},
}

// keypairCmd represents the gen keypair command
var keypairCmd = &cobra.Command{
	Use:   "keypair",
	Short: "Generates an X.509 ECDSA key pair",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		opts := []x509.Option{}
		if ip != "" {
			parsed := net.ParseIP(ip)
			if parsed == nil {
				helpers.Fatalf("invalid IP: %s", ip)
			}
			ips := []net.IP{parsed}
			opts = append(opts, x509.IPAddresses(ips))
		}
		if organization != "" {
			opts = append(opts, x509.Organization(organization))
		}
		ca, err := x509.NewSelfSignedCertificateAuthority(opts...)
		if err != nil {
			helpers.Fatalf("error generating CA: %s", err)
		}
		if err := ioutil.WriteFile(organization+".crt", ca.CrtPEM, 0400); err != nil {
			helpers.Fatalf("error writing certificate: %s", err)
		}
		if err := ioutil.WriteFile(organization+".key", ca.KeyPEM, 0400); err != nil {
			helpers.Fatalf("error writing key: %s", err)
		}
	},
}

func init() {
	// Certificate Authorities
	caCmd.Flags().StringVar(&organization, "organization", "", "X.509 distinguished name for the Organization")
	helpers.Should(cobra.MarkFlagRequired(caCmd.Flags(), "organization"))
	caCmd.Flags().IntVar(&hours, "hours", 24, "the hours from now on which the certificate validity period ends")
	caCmd.Flags().BoolVar(&rsa, "rsa", false, "generate in RSA format")
	// Keys
	keyCmd.Flags().StringVar(&name, "name", "", "the basename of the generated file")
	helpers.Should(cobra.MarkFlagRequired(keyCmd.Flags(), "name"))
	// Certificates
	crtCmd.Flags().StringVar(&name, "name", "", "the basename of the generated file")
	helpers.Should(cobra.MarkFlagRequired(crtCmd.Flags(), "name"))
	crtCmd.Flags().StringVar(&ca, "ca", "", "path to the PEM encoded CERTIFICATE")
	helpers.Should(cobra.MarkFlagRequired(crtCmd.Flags(), "ca"))
	crtCmd.Flags().StringVar(&csr, "csr", "", "path to the PEM encoded CERTIFICATE REQUEST")
	helpers.Should(cobra.MarkFlagRequired(crtCmd.Flags(), "csr"))
	crtCmd.Flags().IntVar(&hours, "hours", 24, "the hours from now on which the certificate validity period ends")
	// Keypairs
	keypairCmd.Flags().StringVar(&ip, "ip", "", "generate the certificate for this IP address")
	keypairCmd.Flags().StringVar(&ca, "ca", "", "path to the PEM encoded CERTIFICATE")
	helpers.Should(cobra.MarkFlagRequired(keypairCmd.Flags(), "ca"))
	// Certificate Signing Requests
	csrCmd.Flags().StringVar(&key, "key", "", "path to the PEM encoded EC or RSA PRIVATE KEY")
	helpers.Should(cobra.MarkFlagRequired(csrCmd.Flags(), "key"))
	csrCmd.Flags().StringVar(&ip, "ip", "", "generate the certificate for this IP address")
	helpers.Should(cobra.MarkFlagRequired(csrCmd.Flags(), "ip"))

	genCmd.AddCommand(caCmd, keypairCmd, keyCmd, csrCmd, crtCmd)
	rootCmd.AddCommand(genCmd)
}
