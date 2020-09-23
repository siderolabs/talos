// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"fmt"
	"io/ioutil"

	"github.com/spf13/cobra"

	machineapi "github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/client"
)

var applyConfigCmdFlags struct {
	filename string
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

		return WithClient(func(ctx context.Context, c *client.Client) error {
			if _, err := c.ApplyConfiguration(ctx, &machineapi.ApplyConfigurationRequest{
				Data: cfgBytes,
			}); err != nil {
				return fmt.Errorf("error applyinh new configuration: %s", err)
			}

			return nil
		})
	},
}

func init() {
	applyConfigCmd.Flags().StringVar(&applyConfigCmdFlags.filename, "f", "", "the filename of the updated configuration")

	addCommand(applyConfigCmd)
}
