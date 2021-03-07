// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/client"
)

var (
	recoverSource   string
	apiserverString = strings.ToLower(machine.RecoverRequest_APISERVER.String())
	etcdString      = strings.ToLower(machine.RecoverRequest_ETCD.String())
)

// recoverCmd represents the recover command.
var recoverCmd = &cobra.Command{
	Use:   "recover",
	Short: "Recover a control plane",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		return WithClient(func(ctx context.Context, c *client.Client) error {
			var source machine.RecoverRequest_Source

			switch recoverSource {
			case apiserverString:
				source = machine.RecoverRequest_APISERVER
			case etcdString:
				source = machine.RecoverRequest_ETCD
			default:
				return fmt.Errorf("unknown recovery source: %q", recoverSource)
			}

			if err := c.Recover(ctx, source); err != nil {
				return fmt.Errorf("error executing recovery: %s", err)
			}

			return nil
		})
	},
}

func init() {
	recoverCmd.Flags().StringVarP(
		&recoverSource,
		"source",
		"s",
		apiserverString,
		fmt.Sprintf(
			"The data source for restoring the control plane manifests from (valid options are %q and %q)",
			apiserverString,
			etcdString),
	)

	addCommand(recoverCmd)
}
