// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package gen

import (
	"fmt"
	"io/ioutil"
	"time"

	"github.com/spf13/cobra"
	"github.com/talos-systems/crypto/x509"

	"github.com/talos-systems/talos/pkg/cli"
)

var (
	caOrganization string
	caHours        int
	caRSA          bool
)

// caCmd represents the gen ca command.
var caCmd = &cobra.Command{
	Use:   "ca",
	Short: "Generates a self-signed X.509 certificate authority",
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := []x509.Option{x509.RSA(caRSA)}
		if caOrganization != "" {
			opts = append(opts, x509.Organization(caOrganization))
		}

		opts = append(opts, x509.NotAfter(time.Now().Add(time.Duration(caHours)*time.Hour)))

		ca, err := x509.NewSelfSignedCertificateAuthority(opts...)
		if err != nil {
			return fmt.Errorf("error generating CA: %w", err)
		}

		if err := ioutil.WriteFile(caOrganization+".crt", ca.CrtPEM, 0o600); err != nil {
			return fmt.Errorf("error writing CA certificate: %w", err)
		}

		if err := ioutil.WriteFile(caOrganization+".sha256", []byte(x509.Hash(ca.Crt)), 0o600); err != nil {
			return fmt.Errorf("error writing certificate hash: %w", err)
		}

		if err := ioutil.WriteFile(caOrganization+".key", ca.KeyPEM, 0o600); err != nil {
			return fmt.Errorf("error writing key: %w", err)
		}

		return nil
	},
}

func init() {
	caCmd.Flags().StringVar(&caOrganization, "organization", "", "X.509 distinguished name for the Organization")
	cli.Should(cobra.MarkFlagRequired(caCmd.Flags(), "organization"))
	caCmd.Flags().IntVar(&caHours, "hours", 87600, "the hours from now on which the certificate validity period ends")
	caCmd.Flags().BoolVar(&caRSA, "rsa", false, "generate in RSA format")

	Cmd.AddCommand(caCmd)
}
