// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package gen implements the genration of various artifacts.
package gen

import (
	"fmt"
	"os"
	"time"

	"github.com/siderolabs/crypto/x509"
	"github.com/spf13/cobra"

	"github.com/siderolabs/talos/pkg/cli"
)

var genCACmdFlags struct {
	organization string
	hours        int
	rsa          bool
}

// GenCACmd represents the `gen ca` command.
var GenCACmd = &cobra.Command{
	Use:   "ca",
	Short: "Generates a self-signed X.509 certificate authority",
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := []x509.Option{x509.RSA(genCACmdFlags.rsa)}
		if genCACmdFlags.organization != "" {
			opts = append(opts, x509.Organization(genCACmdFlags.organization))
		}

		opts = append(opts, x509.NotAfter(time.Now().Add(time.Duration(genCACmdFlags.hours)*time.Hour)))

		ca, err := x509.NewSelfSignedCertificateAuthority(opts...)
		if err != nil {
			return fmt.Errorf("error generating CA: %w", err)
		}

		caCertFile := genCACmdFlags.organization + crtExt
		caHashFile := genCACmdFlags.organization + ".sha256"
		caKeyFile := genCACmdFlags.organization + keyExt

		if err := validateFilesExists([]string{caCertFile, caHashFile, caKeyFile}); err != nil {
			return err
		}

		if err := os.WriteFile(caCertFile, ca.CrtPEM, 0o600); err != nil {
			return fmt.Errorf("error writing CA certificate: %w", err)
		}

		if err := os.WriteFile(caHashFile, []byte(x509.Hash(ca.Crt)), 0o600); err != nil {
			return fmt.Errorf("error writing certificate hash: %w", err)
		}

		if err := os.WriteFile(caKeyFile, ca.KeyPEM, 0o600); err != nil {
			return fmt.Errorf("error writing key: %w", err)
		}

		return nil
	},
}

func init() {
	GenCACmd.Flags().StringVar(&genCACmdFlags.organization, "organization", "", "X.509 distinguished name for the Organization")
	cli.Should(cobra.MarkFlagRequired(GenCACmd.Flags(), "organization"))
	GenCACmd.Flags().IntVar(&genCACmdFlags.hours, "hours", 87600, "the hours from now on which the certificate validity period ends")
	GenCACmd.Flags().BoolVar(&genCACmdFlags.rsa, "rsa", false, "generate in RSA format")

	Cmd.AddCommand(GenCACmd)
}
