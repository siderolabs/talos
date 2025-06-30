// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/durationpb"
	"gopkg.in/yaml.v3"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/helpers"
	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/yamlstrip"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/configpatcher"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
)

var patchCmdFlags struct {
	helpers.Mode

	namespace        string
	patch            []string
	patchFile        string
	dryRun           bool
	configTryTimeout time.Duration
}

func patchFn(c *client.Client, patches []configpatcher.Patch) func(context.Context, string, resource.Resource, error) error {
	return func(ctx context.Context, node string, mc resource.Resource, callError error) error {
		if callError != nil {
			return fmt.Errorf("%s: %w", node, callError)
		}

		if mc.Metadata().Type() != config.MachineConfigType {
			return fmt.Errorf("%s: unsupported resource type: %s", node, mc.Metadata().Type())
		}

		if mc.Metadata().ID() != config.ActiveID {
			return nil
		}

		body, err := yaml.Marshal(mc.Spec())
		if err != nil {
			return err
		}

		cfg, err := configpatcher.Apply(configpatcher.WithBytes(body), patches)
		if err != nil {
			return err
		}

		patched, err := cfg.Bytes()
		if err != nil {
			return err
		}

		resp, err := c.ApplyConfiguration(ctx, &machine.ApplyConfigurationRequest{
			Data:           patched,
			Mode:           patchCmdFlags.Mode.Mode,
			DryRun:         patchCmdFlags.dryRun,
			TryModeTimeout: durationpb.New(patchCmdFlags.configTryTimeout),
		})

		if bytes.Equal(
			bytes.TrimSpace(yamlstrip.Comments(patched)),
			bytes.TrimSpace(yamlstrip.Comments(body)),
		) {
			fmt.Fprintln(os.Stderr, "Apply was skipped: no changes detected.")

			return nil
		}

		fmt.Fprintf(os.Stderr, "patched %s/%s at the node %s\n",
			mc.Metadata().Type(),
			mc.Metadata().ID(),
			node,
		)

		helpers.PrintApplyResults(resp)

		return err
	}
}

// patchCmd represents the edit command.
var patchCmd = &cobra.Command{
	Use:   "patch <type> [<id>]",
	Short: "Update field(s) of a resource using a JSON patch.",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return WithClient(func(ctx context.Context, c *client.Client) error {
			if patchCmdFlags.patchFile != "" {
				patchCmdFlags.patch = append(patchCmdFlags.patch, "@"+patchCmdFlags.patchFile)
			}

			if len(patchCmdFlags.patch) == 0 {
				return errors.New("either --patch or --patch-file should be defined")
			}

			patches, err := configpatcher.LoadPatches(patchCmdFlags.patch)
			if err != nil {
				return err
			}

			if err := helpers.ClientVersionCheck(ctx, c); err != nil {
				return err
			}

			for _, node := range GlobalArgs.Nodes {
				nodeCtx := client.WithNodes(ctx, node)
				if err := helpers.ForEachResource(nodeCtx, c, nil, patchFn(c, patches), patchCmdFlags.namespace, args...); err != nil {
					return err
				}
			}

			return nil
		})
	},
}

func init() {
	patchCmd.Flags().StringVar(&patchCmdFlags.namespace, "namespace", "", "resource namespace (default is to use default namespace per resource)")
	patchCmd.Flags().StringVar(&patchCmdFlags.patchFile, "patch-file", "", "a file containing a patch to be applied to the resource.")
	patchCmd.Flags().StringArrayVarP(&patchCmdFlags.patch, "patch", "p", nil, "the patch to be applied to the resource file, use @file to read a patch from file.")
	patchCmd.Flags().BoolVar(&patchCmdFlags.dryRun, "dry-run", false, "print the change summary and patch preview without applying the changes")
	patchCmd.Flags().DurationVar(&patchCmdFlags.configTryTimeout, "timeout", constants.ConfigTryTimeout, "the config will be rolled back after specified timeout (if try mode is selected)")
	helpers.AddModeFlags(&patchCmdFlags.Mode, patchCmd)
	addCommand(patchCmd)
}
