// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"crypto/tls"
	"fmt"
	"io/ioutil"

	"github.com/spf13/cobra"
	"github.com/talos-systems/crypto/x509"

	machineapi "github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/client"
)

var applyConfigCmdFlags struct {
	filename         string
	insecure         bool
	certFingerprints []string
}

// applyConfigCmd represents the applyConfiguration command.
var applyConfigCmd = &cobra.Command{
	Use:   "apply-config",
	Short: "Apply a new configuration to a node",
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if applyConfigCmdFlags.filename == "" {
			return fmt.Errorf("no filename supplied for configuration")
		}

		cfgBytes, err := ioutil.ReadFile(applyConfigCmdFlags.filename)
		if err != nil {
			return fmt.Errorf("failed to read configuration from %q: %w", applyConfigCmdFlags.filename, err)
		}

		if len(cfgBytes) < 1 {
			return fmt.Errorf("no configuration data read")
		}

		if applyConfigCmdFlags.insecure {
			ctx := context.Background()

			if len(Nodes) != 1 {
				return fmt.Errorf("insecure mode requires one and only one node, got %d", len(Nodes))
			}

			tlsConfig := &tls.Config{
				InsecureSkipVerify: true,
			}

			if len(applyConfigCmdFlags.certFingerprints) > 0 {
				fingerprints := make([]x509.Fingerprint, len(applyConfigCmdFlags.certFingerprints))

				for i, stringFingerprint := range applyConfigCmdFlags.certFingerprints {
					fingerprints[i], err = x509.ParseFingerprint(stringFingerprint)
					if err != nil {
						return fmt.Errorf("error parsing certificate fingerprint %q: %v", stringFingerprint, err)
					}
				}

				tlsConfig.VerifyConnection = x509.MatchSPKIFingerprints(fingerprints...)
			}

			c, err := client.New(ctx, client.WithTLSConfig(tlsConfig), client.WithEndpoints(Nodes...))
			if err != nil {
				return err
			}

			//nolint: errcheck
			defer c.Close()

			if _, err := c.ApplyConfiguration(ctx, &machineapi.ApplyConfigurationRequest{
				Data: cfgBytes,
			}); err != nil {
				return fmt.Errorf("error applying new configuration: %s", err)
			}

			return nil
		}

		return WithClient(func(ctx context.Context, c *client.Client) error {
			if _, err := c.ApplyConfiguration(ctx, &machineapi.ApplyConfigurationRequest{
				Data: cfgBytes,
			}); err != nil {
				return fmt.Errorf("error applying new configuration: %s", err)
			}

			return nil
		})
	},
}

func init() {
	applyConfigCmd.Flags().StringVarP(&applyConfigCmdFlags.filename, "file", "f", "", "the filename of the updated configuration")
	applyConfigCmd.Flags().BoolVarP(&applyConfigCmdFlags.insecure, "insecure", "i", false, "apply the config using the insecure (encrypted with no auth) maintenance service")
	applyConfigCmd.Flags().StringSliceVar(&applyConfigCmdFlags.certFingerprints, "cert-fingerprint", nil, "list of server certificate fingeprints to accept (defaults to no check)")

	addCommand(applyConfigCmd)
}
