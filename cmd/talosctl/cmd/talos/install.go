// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/siderolabs/talos/pkg/images"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/version"
	"github.com/siderolabs/talos/pkg/reporter"
)

var installCmdFlags struct {
	imageCmdFlagsType

	installerImage string
}

var installCmd = &cobra.Command{
	Use:   "install <disk>",
	Short: "Install Talos to disk on the target node",
	Long:  ``,
	Args:  cobra.ExactArgs(1),
	// TODO: This API is not available in maintenance mode, and once the system is fully running, installation is not relevant.
	// Requires https://github.com/siderolabs/talos/issues/12702
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return WithClientMaintenance(nil, func(ctx context.Context, c *client.Client) error {
			return installInternal(ctx, c, args[0])
		})
	},
}

//nolint:gocyclo
func installInternal(ctx context.Context, c *client.Client, disk string) error {
	containerdInstance, err := installCmdFlags.containerdInstance()
	if err != nil {
		return err
	}

	stream, err := c.LifecycleClient.Install(ctx, &machine.LifecycleServiceInstallRequest{
		Containerd: containerdInstance,
		Source: &machine.InstallArtifactsSource{
			ImageName: installCmdFlags.installerImage,
		},
		Destination: &machine.InstallDestination{
			Disk: disk,
		},
	})
	if err != nil {
		return fmt.Errorf("error starting install: %w", err)
	}

	rep := reporter.New()

	exited := false

	for {
		resp, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return fmt.Errorf("error during install: %w", err)
		}

		switch payload := resp.GetProgress().GetResponse().(type) {
		case *machine.LifecycleServiceInstallProgress_Message:
			if rep.IsColorized() {
				rep.Report(reporter.Update{
					Message: payload.Message,
					Status:  reporter.StatusRunning,
				})
			}
		case *machine.LifecycleServiceInstallProgress_ExitCode:
			exited = true

			if payload.ExitCode != 0 {
				rep.Report(reporter.Update{
					Message: fmt.Sprintf("install failed with exit code %d", payload.ExitCode),
					Status:  reporter.StatusError,
				})

				return fmt.Errorf("install failed with exit code %d", payload.ExitCode)
			}

			rep.Report(reporter.Update{
				Message: "install completed successfully",
				Status:  reporter.StatusSucceeded,
			})
		}
	}

	if !exited {
		rep.Report(reporter.Update{
			Message: "install stream closed without exit code",
			Status:  reporter.StatusError,
		})
	}

	return nil
}

func init() {
	installCmd.Flags().StringVar(&installCmdFlags.namespace, "namespace", "system",
		"namespace to use: \"system\" (etcd and kubelet images), \"cri\" for all Kubernetes workloads, \"inmem\" for in-memory containerd instance",
	)
	installCmd.Flags().StringVarP(&installCmdFlags.installerImage, "image", "i",
		fmt.Sprintf("%s/%s/installer:%s", images.Registry, images.Username, version.Trim(version.Tag)),
		"the container image to use for performing the install")

	addCommand(installCmd)
}
