package cmd

import (
	stdlibx509 "crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path"
	"strings"
	"time"

	"github.com/autonomy/dianemo/initramfs/src/init/pkg/crypto/x509"
	"github.com/spf13/cobra"
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
			os.Exit(1)
		}
		if err := ioutil.WriteFile(organization+".crt", ca.CrtPEM, 0400); err != nil {
			os.Exit(1)
		}
		if err := ioutil.WriteFile(organization+".sha256", []byte(x509.Hash(ca.Crt)), 0400); err != nil {
			os.Exit(1)
		}
		if err := ioutil.WriteFile(organization+".key", ca.KeyPEM, 0400); err != nil {
			os.Exit(1)
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
			os.Exit(1)
		}
		if err := ioutil.WriteFile(name+".key", key.KeyPEM, 0400); err != nil {
			fmt.Println(err)
			os.Exit(1)
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
			fmt.Println(err)
			os.Exit(1)
		}
		pemBlock, _ := pem.Decode(keyBytes)
		if pemBlock == nil {
			os.Exit(1)
		}
		keyEC, err := stdlibx509.ParseECPrivateKey(pemBlock.Bytes)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		opts := []x509.Option{}
		parsed := net.ParseIP(ip)
		if parsed == nil {
			fmt.Printf("invalid IP: %s", ip)
			os.Exit(1)
		}
		ips := []net.IP{parsed}
		opts = append(opts, x509.IPAddresses(ips))
		opts = append(opts, x509.NotAfter(time.Now().Add(time.Duration(hours)*time.Hour)))
		csr, err := x509.NewCertificateSigningRequest(keyEC, opts...)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		if err := ioutil.WriteFile(strings.TrimSuffix(key, path.Ext(key))+".csr", csr.X509CertificateRequestPEM, 0400); err != nil {
			os.Exit(1)
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
			fmt.Println(err)
			os.Exit(1)
		}
		caPemBlock, _ := pem.Decode(caBytes)
		if caPemBlock == nil {
			os.Exit(1)
		}
		caCrt, err := stdlibx509.ParseCertificate(caPemBlock.Bytes)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		keyBytes, err := ioutil.ReadFile(ca + ".key")
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		keyPemBlock, _ := pem.Decode(keyBytes)
		if keyPemBlock == nil {
			os.Exit(1)
		}
		caKey, err := stdlibx509.ParseECPrivateKey(keyPemBlock.Bytes)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		csrBytes, err := ioutil.ReadFile(csr)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		csrPemBlock, _ := pem.Decode(csrBytes)
		if csrPemBlock == nil {
			fmt.Println(err)
			os.Exit(1)
		}
		ccsr, err := stdlibx509.ParseCertificateRequest(csrPemBlock.Bytes)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		signedCrt, err := x509.NewCertificateFromCSR(caCrt, caKey, ccsr, x509.NotAfter(time.Now().Add(time.Duration(hours)*time.Hour)))
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		if err := ioutil.WriteFile(name+".crt", signedCrt.X509CertificatePEM, 0400); err != nil {
			fmt.Println(err)
			os.Exit(1)
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
				fmt.Printf("invalid IP: %s", ip)
			}
			ips := []net.IP{parsed}
			opts = append(opts, x509.IPAddresses(ips))
		}
		if organization != "" {
			opts = append(opts, x509.Organization(organization))
		}
		ca, err := x509.NewSelfSignedCertificateAuthority(opts...)
		if err != nil {
			os.Exit(1)
		}
		if err := ioutil.WriteFile(organization+".crt", ca.CrtPEM, 0400); err != nil {
			os.Exit(1)
		}
		if err := ioutil.WriteFile(organization+".key", ca.KeyPEM, 0400); err != nil {
			os.Exit(1)
		}
	},
}

func init() {
	// Certificate Authorities
	caCmd.Flags().StringVar(&organization, "organization", "", "X.509 distinguished name for the Organization")
	if err := cobra.MarkFlagRequired(caCmd.Flags(), "organization"); err != nil {
		os.Exit(1)
	}
	caCmd.Flags().IntVar(&hours, "hours", 24, "the hours from now on which the certificate validity period ends")
	caCmd.Flags().BoolVar(&rsa, "rsa", false, "generate in RSA format")
	// Keys
	keyCmd.Flags().StringVar(&name, "name", "", "the basename of the generated file")
	if err := cobra.MarkFlagRequired(keyCmd.Flags(), "name"); err != nil {
		os.Exit(1)
	}
	// Certificates
	crtCmd.Flags().StringVar(&name, "name", "", "the basename of the generated file")
	if err := cobra.MarkFlagRequired(crtCmd.Flags(), "name"); err != nil {
		os.Exit(1)
	}
	crtCmd.Flags().StringVar(&ca, "ca", "", "path to the PEM encoded CERTIFICATE")
	if err := cobra.MarkFlagRequired(crtCmd.Flags(), "ca"); err != nil {
		os.Exit(1)
	}
	crtCmd.Flags().StringVar(&csr, "csr", "", "path to the PEM encoded CERTIFICATE REQUEST")
	if err := cobra.MarkFlagRequired(crtCmd.Flags(), "csr"); err != nil {
		os.Exit(1)
	}
	crtCmd.Flags().IntVar(&hours, "hours", 24, "the hours from now on which the certificate validity period ends")
	// Keypairs
	keypairCmd.Flags().StringVar(&ip, "ip", "", "generate the certificate for this IP address")
	keypairCmd.Flags().StringVar(&ca, "ca", "", "path to the PEM encoded CERTIFICATE")
	if err := cobra.MarkFlagRequired(keypairCmd.Flags(), "ca"); err != nil {
		os.Exit(1)
	}
	// Certificate Signing Requests
	csrCmd.Flags().StringVar(&key, "key", "", "path to the PEM encoded EC or RSA PRIVATE KEY")
	if err := cobra.MarkFlagRequired(csrCmd.Flags(), "key"); err != nil {
		os.Exit(1)
	}
	csrCmd.Flags().StringVar(&ip, "ip", "", "generate the certificate for this IP address")
	if err := cobra.MarkFlagRequired(csrCmd.Flags(), "ip"); err != nil {
		os.Exit(1)
	}

	genCmd.AddCommand(caCmd, keypairCmd, keyCmd, csrCmd, crtCmd)
	rootCmd.AddCommand(genCmd)
}
