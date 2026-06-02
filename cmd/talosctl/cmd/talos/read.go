// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

// readCmd represents the read command.
var readCmd = &cobra.Command{
	Use:     "read <path>",
	Short:   "Read a file on the machine",
	Long:    ``,
	Args:    cobra.ExactArgs(1),
	Aliases: []string{"cat"},
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveError | cobra.ShellCompDirectiveNoFileComp
		}

		return completePathFromNode(cmd.Context(), toComplete), cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		clientFactory, err := NewClientFactory(ctx, nil)
		if err != nil {
			return err
		}

		defer clientFactory.Close() //nolint:errcheck

		ctx, c, _, err := clientFactory.BuildClientEnforceSingleNode(ctx, "read")
		if err != nil {
			return err
		}

		r, err := c.Read(ctx, args[0])
		if err != nil {
			return fmt.Errorf("error reading file: %w", err)
		}

		defer r.Close() //nolint:errcheck

		_, err = io.Copy(os.Stdout, r)
		if err != nil {
			return fmt.Errorf("error reading: %w", err)
		}

		return r.Close()
	},
}

func init() {
	addCommand(readCmd)
}
