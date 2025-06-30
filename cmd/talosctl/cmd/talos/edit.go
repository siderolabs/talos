// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/durationpb"
	"gopkg.in/yaml.v3"
	"k8s.io/kubectl/pkg/cmd/util/editor"
	"k8s.io/kubectl/pkg/cmd/util/editor/crlf"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/helpers"
	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/yamlstrip"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
)

var editCmdFlags struct {
	helpers.Mode

	namespace        string
	dryRun           bool
	configTryTimeout time.Duration
}

//nolint:gocyclo
func editFn(c *client.Client) func(context.Context, string, resource.Resource, error) error {
	var (
		path      string
		lastError string
	)

	edit := editor.NewDefaultEditor([]string{
		"TALOS_EDITOR",
		"EDITOR",
	})

	return func(ctx context.Context, node string, mc resource.Resource, callError error) error {
		if callError != nil {
			return fmt.Errorf("%s: %w", node, callError)
		}

		if mc.Metadata().Type() != config.MachineConfigType {
			return errors.New("only the machineconfig resource can be edited")
		}

		id := mc.Metadata().ID()

		if id != config.ActiveID {
			return nil
		}

		body, err := yaml.Marshal(mc.Spec())
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

			_, err := fmt.Fprintf(w,
				"# Editing %s/%s at node %s\n", mc.Metadata().Type(), id, node,
			)
			if err != nil {
				return err
			}

			if lastError != "" {
				_, err = w.Write([]byte(addEditingComment(lastError)))
				if err != nil {
					return err
				}
			}

			_, err = w.Write(edited)
			if err != nil {
				return err
			}

			editedDiff := edited

			edited, path, err = edit.LaunchTempFile(fmt.Sprintf("%s-%s-edit-", mc.Metadata().Type(), id), ".yaml", &buf)
			if err != nil {
				return err
			}

			defer os.Remove(path) //nolint:errcheck

			edited = stripEditingComment(edited)

			// If we're retrying the loop because of an error, and no change was made in the file, short-circuit
			if lastError != "" && bytes.Equal(yamlstrip.Comments(editedDiff), yamlstrip.Comments(edited)) {
				if _, err = os.Stat(path); !os.IsNotExist(err) {
					message := addEditingComment(lastError)
					message += fmt.Sprintf("A copy of your changes has been stored to %q\nEdit canceled, no valid changes were saved.\n", path)

					return errors.New(message)
				}
			}

			if len(bytes.TrimSpace(bytes.TrimSpace(yamlstrip.Comments(edited)))) == 0 {
				fmt.Fprintln(os.Stderr, "Apply was skipped: empty file.")

				break
			}

			if bytes.Equal(edited, body) {
				fmt.Fprintln(os.Stderr, "Apply was skipped: no changes detected.")

				break
			}

			resp, err := c.ApplyConfiguration(ctx, &machine.ApplyConfigurationRequest{
				Data:           edited,
				Mode:           editCmdFlags.Mode.Mode,
				DryRun:         editCmdFlags.dryRun,
				TryModeTimeout: durationpb.New(editCmdFlags.configTryTimeout),
			})
			if err != nil {
				lastError = err.Error()

				continue
			}

			helpers.PrintApplyResults(resp)

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

func addEditingComment(in string) string {
	lines := strings.Split(in, "\n")

	return fmt.Sprintf("# \n# %s\n", strings.Join(lines, "\n# "))
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
			if err := helpers.ClientVersionCheck(ctx, c); err != nil {
				return err
			}

			for _, node := range GlobalArgs.Nodes {
				nodeCtx := client.WithNodes(ctx, node)
				if err := helpers.ForEachResource(nodeCtx, c, nil, editFn(c), editCmdFlags.namespace, args...); err != nil {
					return err
				}
			}

			return nil
		})
	},
}

func init() {
	editCmd.Flags().StringVar(&editCmdFlags.namespace, "namespace", "", "resource namespace (default is to use default namespace per resource)")
	helpers.AddModeFlags(&editCmdFlags.Mode, editCmd)
	editCmd.Flags().BoolVar(&editCmdFlags.dryRun, "dry-run", false, "do not apply the change after editing and print the change summary instead")
	editCmd.Flags().DurationVar(&editCmdFlags.configTryTimeout, "timeout", constants.ConfigTryTimeout, "the config will be rolled back after specified timeout (if try mode is selected)")
	addCommand(editCmd)
}
