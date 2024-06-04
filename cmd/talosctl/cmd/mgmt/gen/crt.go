// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package gen

import (
	stdlibx509 "crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/siderolabs/crypto/x509"
	"github.com/spf13/cobra"

	"github.com/siderolabs/talos/pkg/cli"
)

var genCrtCmdFlags struct {
	name  string
	ca    string
	csr   string
	hours int
}

// GenCrtCmd represents the `gen crt` command.
var GenCrtCmd = &cobra.Command{
	Use:   "crt",
	Short: "Generates an X.509 Ed25519 certificate",
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		caBytes, err := os.ReadFile(genCrtCmdFlags.ca + crtExt)
		if err != nil {
			return fmt.Errorf("error reading CA cert: %s", err)
		}

		caPemBlock, _ := pem.Decode(caBytes)
		if caPemBlock == nil {
			return errors.New("error decoding cert PEM")
		}

		caCrt, err := stdlibx509.ParseCertificate(caPemBlock.Bytes)
		if err != nil {
			return fmt.Errorf("error parsing cert: %s", err)
		}

		keyBytes, err := os.ReadFile(genCrtCmdFlags.ca + keyExt)
		if err != nil {
			return fmt.Errorf("error reading key file: %s", err)
		}

		keyPemBlock, _ := pem.Decode(keyBytes)
		if keyPemBlock == nil {
			return errors.New("error decoding key PEM")
		}

		caKey, err := stdlibx509.ParsePKCS8PrivateKey(keyPemBlock.Bytes)
		if err != nil {
			return fmt.Errorf("error parsing EC key: %s", err)
		}

		csrBytes, err := os.ReadFile(genCrtCmdFlags.csr)
		if err != nil {
			return fmt.Errorf("error reading CSR: %s", err)
		}

		csrPemBlock, _ := pem.Decode(csrBytes)
		if csrPemBlock == nil {
			return errors.New("error parsing CSR PEM")
		}

		ccsr, err := stdlibx509.ParseCertificateRequest(csrPemBlock.Bytes)
		if err != nil {
			return fmt.Errorf("error parsing CSR: %s", err)
		}

		signedCrt, err := x509.NewCertificateFromCSR(caCrt, caKey, ccsr,
			x509.NotAfter(time.Now().Add(time.Duration(genCrtCmdFlags.hours)*time.Hour)),
			x509.KeyUsage(stdlibx509.KeyUsageDigitalSignature),
			x509.ExtKeyUsage([]stdlibx509.ExtKeyUsage{stdlibx509.ExtKeyUsageClientAuth}),
		)
		if err != nil {
			return fmt.Errorf("error signing certificate: %s", err)
		}

		certFile := genCrtCmdFlags.name + crtExt

		if err = validateFileExists(certFile); err != nil {
			return err
		}

		if err = os.WriteFile(certFile, signedCrt.X509CertificatePEM, 0o600); err != nil {
			return fmt.Errorf("error writing certificate: %s", err)
		}

		return err
	},
}

func init() {
	GenCrtCmd.Flags().StringVar(&genCrtCmdFlags.name, "name", "", "the basename of the generated file")
	cli.Should(cobra.MarkFlagRequired(GenCrtCmd.Flags(), "name"))
	GenCrtCmd.Flags().StringVar(&genCrtCmdFlags.ca, "ca", "", "path to the PEM encoded CERTIFICATE")
	cli.Should(cobra.MarkFlagRequired(GenCrtCmd.Flags(), "ca"))
	GenCrtCmd.Flags().StringVar(&genCrtCmdFlags.csr, "csr", "", "path to the PEM encoded CERTIFICATE REQUEST")
	cli.Should(cobra.MarkFlagRequired(GenCrtCmd.Flags(), "csr"))
	GenCrtCmd.Flags().IntVar(&genCrtCmdFlags.hours, "hours", 24, "the hours from now on which the certificate validity period ends")

	Cmd.AddCommand(GenCrtCmd)
}
