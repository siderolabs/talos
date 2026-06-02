// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/global"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/client/multiplex"
)

var metaCmdFlags struct {
	global.InsecureFlags
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
		key, err := strconv.ParseUint(args[0], 0, 8)
		if err != nil {
			return err
		}

		ctx := cmd.Context()

		clientFactory, err := NewClientFactory(ctx, &metaCmdFlags)
		if err != nil {
			return err
		}

		defer clientFactory.Close() //nolint:errcheck

		respCh := multiplex.UnaryViaFactory(
			ctx, clientFactory,
			func(ctx context.Context, c *client.Client) (struct{}, error) {
				return struct{}{}, c.MetaWrite(ctx, uint8(key), []byte(args[1]))
			},
		)

		var errs error

		for resp := range respCh {
			if resp.Err != nil {
				errs = errors.Join(errs, fmt.Errorf("error writing meta to node %s: %w", resp.Node, resp.Err))
			}
		}

		return errs
	},
}

var metaDeleteCmd = &cobra.Command{
	Use:   "delete key",
	Short: "Delete a key from the META partition.",
	Long:  ``,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		key, err := strconv.ParseUint(args[0], 0, 8)
		if err != nil {
			return err
		}

		ctx := cmd.Context()

		clientFactory, err := NewClientFactory(ctx, &metaCmdFlags)
		if err != nil {
			return err
		}

		defer clientFactory.Close() //nolint:errcheck

		respCh := multiplex.UnaryViaFactory(
			ctx, clientFactory,
			func(ctx context.Context, c *client.Client) (struct{}, error) {
				return struct{}{}, c.MetaDelete(ctx, uint8(key))
			},
		)

		var errs error

		for resp := range respCh {
			if resp.Err != nil {
				errs = errors.Join(errs, fmt.Errorf("error deleting meta from node %s: %w", resp.Node, resp.Err))
			}
		}

		return errs
	},
}

func init() {
	metaCmdFlags.InsecureFlags.AddFlags(metaCmd)

	metaCmd.AddCommand(metaWriteCmd)
	metaCmd.AddCommand(metaDeleteCmd)
	addCommand(metaCmd)
}
