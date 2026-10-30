// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build !windows

package talos

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/safeout"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/reporter"
)

var debugCmdFlags struct {
	imageCmdFlagsType

	args []string
}

func init() {
	debugCmd.Flags().StringSliceVar(&debugCmdFlags.args, "args", nil, "arguments to pass to the container")
	debugCmd.Flags().StringVar(&debugCmdFlags.namespace, "namespace", "inmem", "namespace to use: `system` (CRI containerd) or `inmem` for in-memory containerd instance")

	// The --host-ns flag and its examples are only registered in debug builds
	// (sidero.debug). The server-side PROFILE_HOST_NS API is always available.
	registerDebugHostNsFlag(debugCmd)

	addCommand(debugCmd)
}

// debugCmd represents the debug command.
var debugCmd = &cobra.Command{
	Use:   "debug [<image-tar-path|image ref>]",
	Short: "Run a debug container from an image archive or reference",
	Example: `  # Run a debug container from a local tar archive (image will be loaded into Talos from the archive)
    talosctl debug ./debug-tools.tar --args /bin/sh

  # Run a debug container from an image reference (Talos will pull the image if not present)
    talosctl debug docker.io/library/alpine:latest --args /bin/sh`,

	Args: cobra.RangeArgs(0, 1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if debugCmdFlags.namespace == constants.TalosContainersContainerdNamespace {
			return fmt.Errorf("talosctl debug does not support the %q namespace", constants.TalosContainersContainerdNamespace)
		}

		ctx := cmd.Context()

		clientFactory, err := NewClientFactory(ctx, nil)
		if err != nil {
			return err
		}

		defer clientFactory.Close() //nolint:errcheck

		ctx, c, node, err := clientFactory.BuildClientEnforceSingleNode(ctx, "debug")
		if err != nil {
			return err
		}

		rep := reporter.New(reporter.WithLineFilter(safeout.String))

		ctrdInstance, err := debugCmdFlags.containerdInstance()
		if err != nil {
			return err
		}

		// Determine the image reference. A positional argument (tar path or image
		// ref) always wins; otherwise host-ns falls back to the default Nix tools
		// image, while the privileged profile requires an explicit image.
		var imageRef string

		switch {
		case len(args) > 0:
			imageRef = args[0]
		case debugHostNsEnabled():
			imageRef = constants.DebugHostNsImage
		default:
			return errors.New("an image tar path or reference is required")
		}

		// verify if we are sending a tarball or pulling an image
		_, err = os.Stat(imageRef)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to stat image argument: %w", err)
		}

		var imgName string

		if err == nil {
			imgName, err = imageImportInternal(ctx, c, ctrdInstance, node, imageRef, rep)
			if err != nil {
				return fmt.Errorf("failed to import image: %w", err)
			}
		} else {
			pullResult, err := imagePullInternal(ctx, clientFactory, ctrdInstance, imageRef, rep)
			if err != nil {
				return fmt.Errorf("failed to pull image: %w", err)
			}

			imgName = pullResult[node]
		}

		profile := machine.DebugContainerRunRequestSpec_PROFILE_PRIVILEGED
		if debugHostNsEnabled() {
			profile = machine.DebugContainerRunRequestSpec_PROFILE_HOST_NS
		}

		tty := term.IsTerminal(int(os.Stdin.Fd()))

		opts, restoreTerminal, err := containerStreamTerminal(ctx, true, tty)
		if err != nil {
			return err
		}

		exitCode, err := c.DebugContainerRun(ctx, &machine.DebugContainerRunRequestSpec{
			Containerd: ctrdInstance,
			ImageName:  imgName,
			Args:       debugCmdFlags.args,
			Profile:    profile,
			Tty:        tty,
		}, opts)

		restoreTerminal()

		if err != nil {
			return err
		}

		if exitCode != 0 {
			return fmt.Errorf("container exited with code %d", exitCode)
		}

		return nil
	},
}
