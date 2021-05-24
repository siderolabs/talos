// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package gen

import (
	stdlibx509 "crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/spf13/cobra"
	"github.com/talos-systems/crypto/x509"

	"github.com/talos-systems/talos/pkg/cli"
)

var (
	crtName  string
	crtCA    string
	crtCSR   string
	crtHours int
)

// crtCmd represents the gen crt command.
var crtCmd = &cobra.Command{
	Use:   "crt",
	Short: "Generates an X.509 Ed25519 certificate",
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		caBytes, err := ioutil.ReadFile(crtCA + ".crt")
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

		keyBytes, err := ioutil.ReadFile(crtCA + ".key")
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

		csrBytes, err := ioutil.ReadFile(crtCSR)
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

		if err = ioutil.WriteFile(crtName+".crt", signedCrt.X509CertificatePEM, 0o600); err != nil {
			return fmt.Errorf("error writing certificate: %s", err)
		}

		return err
	},
}

func init() {
	crtCmd.Flags().StringVar(&crtName, "name", "", "the basename of the generated file")
	cli.Should(cobra.MarkFlagRequired(crtCmd.Flags(), "name"))
	crtCmd.Flags().StringVar(&crtCA, "ca", "", "path to the PEM encoded CERTIFICATE")
	cli.Should(cobra.MarkFlagRequired(crtCmd.Flags(), "ca"))
	crtCmd.Flags().StringVar(&crtCSR, "csr", "", "path to the PEM encoded CERTIFICATE REQUEST")
	cli.Should(cobra.MarkFlagRequired(crtCmd.Flags(), "csr"))
	crtCmd.Flags().IntVar(&crtHours, "hours", 24, "the hours from now on which the certificate validity period ends")

	Cmd.AddCommand(crtCmd)
}
