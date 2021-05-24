// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package gen

import (
	"fmt"
	"io/ioutil"
	"net"

	"github.com/spf13/cobra"
	"github.com/talos-systems/crypto/x509"

	"github.com/talos-systems/talos/pkg/cli"
)

var (
	keypairIP           string
	keypairOrganization string
)

// keypairCmd represents the gen keypair command.
var keypairCmd = &cobra.Command{
	Use:   "keypair",
	Short: "Generates an X.509 Ed25519 key pair",
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := []x509.Option{}
		if keypairIP != "" {
			parsed := net.ParseIP(keypairIP)
			if parsed == nil {
				return fmt.Errorf("invalid IP: %s", keypairIP)
			}
			ips := []net.IP{parsed}
			opts = append(opts, x509.IPAddresses(ips))
		}
		if keypairOrganization != "" {
			opts = append(opts, x509.Organization(keypairOrganization))
		}
		ca, err := x509.NewSelfSignedCertificateAuthority(opts...)
		if err != nil {
			return fmt.Errorf("error generating CA: %s", err)
		}
		if err := ioutil.WriteFile(keypairOrganization+".crt", ca.CrtPEM, 0o600); err != nil {
			return fmt.Errorf("error writing certificate: %s", err)
		}
		if err := ioutil.WriteFile(keypairOrganization+".key", ca.KeyPEM, 0o600); err != nil {
			return fmt.Errorf("error writing key: %s", err)
		}

		return nil
	},
}

func init() {
	keypairCmd.Flags().StringVar(&keypairIP, "ip", "", "generate the certificate for this IP address")
	keypairCmd.Flags().StringVar(&keypairOrganization, "organization", "", "X.509 distinguished name for the Organization")
	cli.Should(cobra.MarkFlagRequired(keypairCmd.Flags(), "organization"))

	Cmd.AddCommand(keypairCmd)
}
