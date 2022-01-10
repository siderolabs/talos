// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/spf13/cobra"
	yaml "gopkg.in/yaml.v3"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/talos-systems/talos/cmd/talosctl/pkg/talos/helpers"
	"github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/client"
	"github.com/talos-systems/talos/pkg/machinery/config/configpatcher"
	"github.com/talos-systems/talos/pkg/machinery/resources/config"
)

var patchCmdFlags struct {
	helpers.Mode
	namespace string
	patch     string
	patchFile string
}

func patchFn(c *client.Client, patch jsonpatch.Patch) func(context.Context, client.ResourceResponse) error {
	return func(ctx context.Context, msg client.ResourceResponse) error {
		if msg.Resource == nil {
			if msg.Definition.Metadata().ID() != strings.ToLower(config.MachineConfigType) {
				return fmt.Errorf("only the machineconfig resource can be edited")
			}

			return nil
		}

		body, err := yaml.Marshal(msg.Resource.Spec())
		if err != nil {
			return err
		}

		patched, err := configpatcher.JSON6902(body, patch)
		if err != nil {
			return err
		}

		resp, err := c.ApplyConfiguration(ctx, &machine.ApplyConfigurationRequest{
			Data:      patched,
			Mode:      patchCmdFlags.Mode.Mode,
			OnReboot:  patchCmdFlags.OnReboot,
			Immediate: patchCmdFlags.Immediate,
		})

		if bytes.Equal(
			bytes.TrimSpace(cmdutil.StripComments(patched)),
			bytes.TrimSpace(cmdutil.StripComments(body)),
		) {
			fmt.Println("Apply was skipped: no changes detected.")

			return nil
		}

		fmt.Printf("patched %s/%s at the node %s\n",
			msg.Resource.Metadata().Type(),
			msg.Resource.Metadata().ID(),
			msg.Metadata.GetHostname(),
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
			var (
				patch     jsonpatch.Patch
				patchData []byte
			)

			switch {
			case patchCmdFlags.patch != "":
				patchData = []byte(patchCmdFlags.patch)
			case patchCmdFlags.patchFile != "":
				f, err := os.Open(patchCmdFlags.patchFile)
				if err != nil {
					return err
				}

				patchData, err = ioutil.ReadAll(f)
				if err != nil {
					return err
				}
			default:
				return fmt.Errorf("either --patch or --patch-file should be defined")
			}

			patch, err := jsonpatch.DecodePatch(patchData)
			if err != nil {
				return err
			}

			for _, node := range Nodes {
				nodeCtx := client.WithNodes(ctx, node)
				if err := helpers.ForEachResource(nodeCtx, c, patchFn(c, patch), patchCmdFlags.namespace, args...); err != nil {
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
	patchCmd.Flags().StringVarP(&patchCmdFlags.patch, "patch", "p", "", "the patch to be applied to the resource file.")
	helpers.AddModeFlags(&patchCmdFlags.Mode, patchCmd)
	addCommand(patchCmd)
}
