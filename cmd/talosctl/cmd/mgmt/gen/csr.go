// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package gen

import (
	stdlibx509 "crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"os"
	"path"
	"strings"

	"github.com/siderolabs/crypto/x509"
	"github.com/spf13/cobra"

	"github.com/siderolabs/talos/pkg/cli"
	"github.com/siderolabs/talos/pkg/machinery/role"
)

var genCSRCmdFlags struct {
	key   string
	ip    string
	roles []string
}

// genCSRCmd represents the `gen csr` command.
var genCSRCmd = &cobra.Command{
	Use:   "csr",
	Short: "Generates a CSR using an Ed25519 private key",
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		keyBytes, err := os.ReadFile(genCSRCmdFlags.key)
		if err != nil {
			return fmt.Errorf("error reading key: %s", err)
		}

		pemBlock, _ := pem.Decode(keyBytes)
		if pemBlock == nil {
			return errors.New("error decoding PEM")
		}

		keyEC, err := stdlibx509.ParsePKCS8PrivateKey(pemBlock.Bytes)
		if err != nil {
			return fmt.Errorf("error parsing ECDSA key: %s", err)
		}

		var opts []x509.Option

		parsed := net.ParseIP(genCSRCmdFlags.ip)
		if parsed == nil {
			return fmt.Errorf("invalid IP: %s", genCSRCmdFlags.ip)
		}

		roles, unknownRoles := role.Parse(genCSRCmdFlags.roles)
		if len(unknownRoles) != 0 {
			return fmt.Errorf("unknown roles: %s", strings.Join(unknownRoles, ", "))
		}

		ips := []net.IP{parsed}
		opts = append(opts, x509.Organization(roles.Strings()...))
		opts = append(opts, x509.IPAddresses(ips))

		csr, err := x509.NewCertificateSigningRequest(keyEC, opts...)
		if err != nil {
			return fmt.Errorf("error generating CSR: %s", err)
		}

		csrFile := strings.TrimSuffix(genCSRCmdFlags.key, path.Ext(genCSRCmdFlags.key)) + ".csr"

		if err := validateFileExists(csrFile); err != nil {
			return err
		}

		if err := os.WriteFile(csrFile, csr.X509CertificateRequestPEM, 0o600); err != nil {
			return fmt.Errorf("error writing CSR: %s", err)
		}

		return nil
	},
}

func init() {
	genCSRCmd.Flags().StringVar(&genCSRCmdFlags.key, "key", "", "path to the PEM encoded EC or RSA PRIVATE KEY")
	cli.Should(cobra.MarkFlagRequired(genCSRCmd.Flags(), "key"))
	genCSRCmd.Flags().StringVar(&genCSRCmdFlags.ip, "ip", "", "generate the certificate for this IP address")
	cli.Should(cobra.MarkFlagRequired(genCSRCmd.Flags(), "ip"))
	genCSRCmd.Flags().StringSliceVar(&genCSRCmdFlags.roles, "roles", role.MakeSet(role.Admin).Strings(), "roles")

	Cmd.AddCommand(genCSRCmd)
}
