// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
)

var hypervisorScreenshotCmdFlags struct {
	output string
}

// hypervisorScreenshotCmd represents the hypervisor screenshot command.
var hypervisorScreenshotCmd = &cobra.Command{
	Use:   "screenshot <vm-name>",
	Short: "Capture the display of a virtual machine",
	Long:  `Captures the current display of a running virtual machine with VNC enabled and writes the image as produced by libvirt (usually PNG).`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return withHypervisorNode(cmd.Context(), "talosctl hypervisor screenshot", func(ctx context.Context, c *client.Client) error {
			resp, err := c.HypervisorService().Screenshot(ctx, &machine.VirtualMachineScreenshotRequest{Name: args[0]})
			if err != nil {
				return err
			}

			output := hypervisorScreenshotCmdFlags.output
			if output == "" {
				output = args[0] + screenshotExtension(resp.GetMimeType())
			}

			if err = os.WriteFile(output, resp.GetImage(), 0o600); err != nil {
				return fmt.Errorf("write screenshot: %w", err)
			}

			fmt.Fprintf(cmd.ErrOrStderr(), "wrote %s (%s, %d bytes)\n", output, resp.GetMimeType(), len(resp.GetImage()))

			return nil
		})
	},
}

func screenshotExtension(mime string) string {
	switch mime {
	case "image/png":
		return ".png"
	case "image/x-portable-pixmap":
		return ".ppm"
	default:
		return ".img"
	}
}

func init() {
	hypervisorScreenshotCmd.Flags().StringVarP(&hypervisorScreenshotCmdFlags.output, "output", "o", "",
		"output file path (defaults to <vm-name> with an extension matching the image format)")

	hypervisorCmd.AddCommand(hypervisorScreenshotCmd)
}
