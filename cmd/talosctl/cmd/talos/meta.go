// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/siderolabs/talos/pkg/machinery/client"
)

var metaCmdFlags struct {
	insecure bool
}

var metaCmd = &cobra.Command{
	Use:   "meta",
	Short: "Write and delete keys in the META partition",
	Long:  ``,
	Args:  cobra.NoArgs,
}

var metaWriteCmd = &cobra.Command{
	Use:   "write key value",
	Short: "Write a key-value pair to the META partition.",
	Long:  ``,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		fn := func(ctx context.Context, c *client.Client) error {
			key, err := strconv.ParseUint(args[0], 0, 8)
			if err != nil {
				return err
			}

			return c.MetaWrite(ctx, uint8(key), []byte(args[1]))
		}

		if metaCmdFlags.insecure {
			return WithClientMaintenance(nil, fn)
		}

		if GlobalArgs.SkipVerify {
			return WithClientSkipVerify(fn)
		}

		return WithClient(fn)
	},
}

var metaDeleteCmd = &cobra.Command{
	Use:   "delete key",
	Short: "Delete a key from the META partition.",
	Long:  ``,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fn := func(ctx context.Context, c *client.Client) error {
			key, err := strconv.ParseUint(args[0], 0, 8)
			if err != nil {
				return err
			}

			return c.MetaDelete(ctx, uint8(key))
		}

		if metaCmdFlags.insecure {
			return WithClientMaintenance(nil, fn)
		}

		if GlobalArgs.SkipVerify {
			return WithClientSkipVerify(fn)
		}

		return WithClient(fn)
	},
}

func init() {
	metaCmd.PersistentFlags().BoolVarP(&metaCmdFlags.insecure, "insecure", "i", false, "write|delete meta using the insecure (encrypted with no auth) maintenance service")

	metaCmd.AddCommand(metaWriteCmd)
	metaCmd.AddCommand(metaDeleteCmd)
	addCommand(metaCmd)
}
