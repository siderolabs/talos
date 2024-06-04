// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package machineconfig

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/siderolabs/talos/pkg/machinery/config/configpatcher"
)

var patchCmdFlags struct {
	patches []string
	output  string
}

// PatchCmd represents the `machineconfig patch` command.
var PatchCmd = &cobra.Command{
	Use:   "patch <machineconfig-file>",
	Short: "Patch a machine config",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		data, err := os.ReadFile(args[0])
		if err != nil {
			return err
		}

		patches, err := configpatcher.LoadPatches(patchCmdFlags.patches)
		if err != nil {
			return err
		}

		patched, err := configpatcher.Apply(configpatcher.WithBytes(data), patches)
		if err != nil {
			return err
		}

		patchedData, err := patched.Bytes()
		if err != nil {
			return err
		}

		if patchCmdFlags.output == "" { // write to stdout
			fmt.Printf("%s\n", patchedData)

			return nil
		}

		// write to file

		parentDir := filepath.Dir(patchCmdFlags.output)

		// Create dir path, ignoring "already exists" messages
		if err := os.MkdirAll(parentDir, os.ModePerm); err != nil && !os.IsExist(err) {
			return fmt.Errorf("failed to create output dir: %w", err)
		}

		return os.WriteFile(patchCmdFlags.output, patchedData, 0o644)
	},
}

func init() {
	// use StringArrayVarP instead of StringSliceVarP to prevent cobra from splitting the patch string on commas
	PatchCmd.Flags().StringArrayVarP(&patchCmdFlags.patches, "patch", "p", nil, "patch generated machineconfigs (applied to all node types), use @file to read a patch from file")
	PatchCmd.Flags().StringVarP(&patchCmdFlags.output, "output", "o", "", "output destination. if not specified, output will be printed to stdout")

	Cmd.AddCommand(PatchCmd)
}
