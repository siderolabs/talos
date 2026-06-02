// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/global"
	"github.com/siderolabs/talos/pkg/cli"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/client/multiplex"
	"github.com/siderolabs/talos/pkg/machinery/version"
)

// versionCmdFlags represents the `talosctl version` command's flags.
var versionCmdFlags struct {
	global.InsecureFlags

	clientOnly   bool
	shortVersion bool
	json         bool
}

// versionCmd represents the `talosctl version` command.
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Prints the version",
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !versionCmdFlags.json {
			fmt.Println("Client:")

			if versionCmdFlags.shortVersion {
				version.PrintShortVersion()
			} else {
				version.PrintLongVersion()
			}

			// Exit early if we're only looking for client version
			if versionCmdFlags.clientOnly {
				return nil
			}

			fmt.Println("Server:")
		}

		ctx := cmd.Context()

		clientFactory, err := NewClientFactory(ctx, &versionCmdFlags)
		if err != nil {
			return err
		}

		defer clientFactory.Close() //nolint:errcheck

		respCh := multiplex.UnaryViaFactory(
			ctx, clientFactory,
			func(ctx context.Context, c *client.Client) (*machine.VersionResponse, error) {
				return c.Version(ctx)
			},
		)

		var errs error

		for resp := range respCh {
			if resp.Err != nil {
				errs = errors.Join(errs, fmt.Errorf("error getting version from node %s: %w", resp.Node, resp.Err))

				continue
			}

			errs = errors.Join(errs, printVersionResponse(resp.Node, resp.Payload))
		}

		return errs
	},
}

func printVersionResponse(node string, resp *machine.VersionResponse) error {
	for _, msg := range resp.Messages {
		if !versionCmdFlags.json {
			fmt.Printf("\t%s:        %s\n", "NODE", node)

			version.PrintLongVersionFromExisting(msg.Version)

			var enabledFeatures []string
			if msg.Features.GetRbac() {
				enabledFeatures = append(enabledFeatures, "RBAC")
			}

			fmt.Printf("\tEnabled:     %s\n", strings.Join(enabledFeatures, ", "))

			continue
		}

		b, err := protojson.Marshal(msg)
		if err != nil {
			return err
		}

		fmt.Printf("%s\n", b)
	}

	return nil
}

func init() {
	versionCmdFlags.InsecureFlags.AddFlags(versionCmd)
	versionCmd.Flags().BoolVar(&versionCmdFlags.shortVersion, "short", false, "Print the short version")
	versionCmd.Flags().BoolVar(&versionCmdFlags.clientOnly, "client", false, "Print client version only")

	// TODO remove when https://github.com/siderolabs/talos/issues/907 is implemented
	versionCmd.Flags().BoolVar(&versionCmdFlags.json, "json", false, "")
	cli.Should(versionCmd.Flags().MarkHidden("json"))

	addCommand(versionCmd)
}
