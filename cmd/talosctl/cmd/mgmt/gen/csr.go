// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package gen

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
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

var (
	csrKey string
	csrIP  string
)

// csrCmd represents the gen csr command.
var csrCmd = &cobra.Command{
	Use:   "csr",
	Short: "Generates a CSR using an Ed25519 private key",
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		keyBytes, err := ioutil.ReadFile(csrKey)
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

		parsed := net.ParseIP(csrIP)
		if parsed == nil {
			return fmt.Errorf("invalid IP: %s", csrIP)
		}

		ips := []net.IP{parsed}
		opts = append(opts, x509.Organization(constants.RoleAdmin))
		opts = append(opts, x509.IPAddresses(ips))
		opts = append(opts, x509.NotAfter(time.Now().Add(time.Duration(crtHours)*time.Hour))) // BUG

		csr, err := x509.NewCertificateSigningRequest(keyEC, opts...)
		if err != nil {
			return fmt.Errorf("error generating CSR: %s", err)
		}

		if err := ioutil.WriteFile(strings.TrimSuffix(csrKey, path.Ext(csrKey))+".csr", csr.X509CertificateRequestPEM, 0o600); err != nil {
			return fmt.Errorf("error writing CSR: %s", err)
		}

		return nil
	},
}

func init() {
	csrCmd.Flags().StringVar(&csrKey, "key", "", "path to the PEM encoded EC or RSA PRIVATE KEY")
	cli.Should(cobra.MarkFlagRequired(csrCmd.Flags(), "key"))
	csrCmd.Flags().StringVar(&csrIP, "ip", "", "generate the certificate for this IP address")
	cli.Should(cobra.MarkFlagRequired(csrCmd.Flags(), "ip"))

	Cmd.AddCommand(csrCmd)
}
