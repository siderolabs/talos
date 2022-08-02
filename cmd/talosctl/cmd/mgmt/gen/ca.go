// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package gen

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/talos-systems/crypto/x509"

	"github.com/talos-systems/talos/pkg/cli"
)

var genCACmdFlags struct {
	organization string
	hours        int
	rsa          bool
}

// genCACmd represents the `gen ca` command.
var genCACmd = &cobra.Command{
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

		if err := os.WriteFile(genCACmdFlags.organization+".crt", ca.CrtPEM, 0o600); err != nil {
			return fmt.Errorf("error writing CA certificate: %w", err)
		}

		if err := os.WriteFile(genCACmdFlags.organization+".sha256", []byte(x509.Hash(ca.Crt)), 0o600); err != nil {
			return fmt.Errorf("error writing certificate hash: %w", err)
		}

		if err := os.WriteFile(genCACmdFlags.organization+".key", ca.KeyPEM, 0o600); err != nil {
			return fmt.Errorf("error writing key: %w", err)
		}

		return nil
	},
}

func init() {
	genCACmd.Flags().StringVar(&genCACmdFlags.organization, "organization", "", "X.509 distinguished name for the Organization")
	cli.Should(cobra.MarkFlagRequired(genCACmd.Flags(), "organization"))
	genCACmd.Flags().IntVar(&genCACmdFlags.hours, "hours", 87600, "the hours from now on which the certificate validity period ends")
	genCACmd.Flags().BoolVar(&genCACmdFlags.rsa, "rsa", false, "generate in RSA format")

	Cmd.AddCommand(genCACmd)
}
