// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	yaml "gopkg.in/yaml.v3"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/cmd/util/editor"
	"k8s.io/kubectl/pkg/cmd/util/editor/crlf"

	"github.com/talos-systems/talos/cmd/talosctl/pkg/talos/helpers"
	"github.com/talos-systems/talos/pkg/cli"
	"github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/client"
	"github.com/talos-systems/talos/pkg/machinery/resources/config"
)

var editCmdFlags struct {
	namespace string
	immediate bool
	onReboot  bool
}

//nolint:gocyclo
func editFn(c *client.Client) func(context.Context, client.ResourceResponse) error {
	var lastError string

	edit := editor.NewDefaultEditor([]string{
		"TALOS_EDITOR",
		"EDITOR",
	})

	return func(ctx context.Context, msg client.ResourceResponse) error {
		if msg.Definition != nil {
			if msg.Definition.Metadata().ID() != strings.ToLower(config.MachineConfigType) {
				return fmt.Errorf("only the machineconfig resource can be edited")
			}
		}

		if msg.Resource == nil {
			return nil
		}

		metadata := msg.Resource.Metadata()
		id := metadata.ID()

		body, err := yaml.Marshal(msg.Resource.Spec())
		if err != nil {
			return err
		}

		edited := body

		for {
			var (
				buf bytes.Buffer
				w   io.Writer = &buf
			)

			if runtime.GOOS == "windows" {
				w = crlf.NewCRLFWriter(w)
			}

			_, err := w.Write([]byte(
				fmt.Sprintf(
					"# Editing %s/%s at node %s\n", msg.Resource.Metadata().Type(), id, msg.Metadata.GetHostname(),
				),
			))
			if err != nil {
				return err
			}

			if lastError != "" {
				lines := strings.Split(lastError, "\n")

				_, err = w.Write([]byte(
					fmt.Sprintf("# \n# %s\n", strings.Join(lines, "\n# ")),
				))
				if err != nil {
					return err
				}
			}

			_, err = w.Write(edited)
			if err != nil {
				return err
			}

			edited, _, err = edit.LaunchTempFile(fmt.Sprintf("%s-%s-edit-", msg.Resource.Metadata().Type(), id), ".yaml", &buf)
			if err != nil {
				return err
			}

			edited = stripEditingComment(edited)

			if len(bytes.TrimSpace(bytes.TrimSpace(cmdutil.StripComments(edited)))) == 0 {
				fmt.Println("Apply was skipped: empty file.")

				break
			}

			if bytes.Equal(edited, body) {
				fmt.Println("Apply was skipped: no changes detected.")

				break
			}

			resp, err := c.ApplyConfiguration(ctx, &machine.ApplyConfigurationRequest{
				Data:      edited,
				Immediate: editCmdFlags.immediate,
				OnReboot:  editCmdFlags.onReboot,
			})
			if err != nil {
				lastError = err.Error()

				continue
			}

			for _, m := range resp.GetMessages() {
				for _, w := range m.GetWarnings() {
					cli.Warning("%s", w)
				}
			}

			break
		}

		return nil
	}
}

func stripEditingComment(in []byte) []byte {
	for {
		idx := bytes.Index(in, []byte{'\n'})
		if idx == -1 {
			return in
		}

		if !bytes.HasPrefix(in, []byte("# ")) {
			return in
		}

		in = in[idx+1:]
	}
}

// editCmd represents the edit command.
var editCmd = &cobra.Command{
	Use:   "edit <type> [<id>]",
	Short: "Edit a resource from the default editor.",
	Args:  cobra.RangeArgs(1, 2),
	Long: `The edit command allows you to directly edit any API resource
you can retrieve via the command line tools.

It will open the editor defined by your TALOS_EDITOR,
or EDITOR environment variables, or fall back to 'vi' for Linux
or 'notepad' for Windows.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return WithClient(func(ctx context.Context, c *client.Client) error {
			for _, node := range Nodes {
				nodeCtx := client.WithNodes(ctx, node)
				if err := helpers.ForEachResource(nodeCtx, c, editFn(c), editCmdFlags.namespace, args...); err != nil {
					return err
				}
			}

			return nil
		})
	},
}

func init() {
	editCmd.Flags().StringVar(&editCmdFlags.namespace, "namespace", "", "resource namespace (default is to use default namespace per resource)")
	editCmd.Flags().BoolVar(&editCmdFlags.immediate, "immediate", false, "apply the change immediately (without a reboot)")
	editCmd.Flags().BoolVar(&editCmdFlags.onReboot, "on-reboot", false, "apply the change on next reboot")
	addCommand(editCmd)
}
