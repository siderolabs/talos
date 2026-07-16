// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/mgmt/helpers"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/provision/providers"
	"github.com/siderolabs/talos/pkg/provision/providers/remote"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync kernel and initramfs to a remote cluster",
	Long: `Uploads the locally-built kernel and initramfs to a remote QEMU cluster.

Artifacts are content-addressed, so unchanged files are not uploaded. The command
updates the stable boot paths used by clusters created without a bootloader. Run
'talosctl cluster reboot' afterward to restart the VMs with the new artifacts.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return syncRemoteBootArtifacts(cmd.Context())
	},
}

func syncRemoteBootArtifacts(ctx context.Context) error {
	if PersistentFlags.RemoteEndpoint == "" {
		return errors.New("cluster sync requires --remote-endpoint")
	}

	provisioner, err := providers.Factory(ctx, providers.RemoteProviderName, providers.WithRemoteEndpoint(PersistentFlags.RemoteEndpoint))
	if err != nil {
		return err
	}

	defer provisioner.Close() //nolint:errcheck

	remoteProvisioner, ok := provisioner.(*remote.Provisioner)
	if !ok {
		return errors.New("remote provisioner expected")
	}

	arch, err := remoteProvisioner.ServerArch(ctx)
	if err != nil {
		return err
	}

	kernelPath := strings.ReplaceAll(helpers.ArtifactPath(constants.KernelAssetWithArch), constants.ArchVariable, arch)
	initramfsPath := strings.ReplaceAll(helpers.ArtifactPath(constants.InitramfsAssetWithArch), constants.ArchVariable, arch)

	changed, err := remoteProvisioner.SyncBootArtifacts(ctx, PersistentFlags.ClusterName, kernelPath, initramfsPath)
	if err != nil {
		return err
	}

	for _, key := range []string{"kernel", "initramfs"} {
		if changed[key] {
			fmt.Printf("synced %s\n", key)
		} else {
			fmt.Printf("%s already up to date\n", key)
		}
	}

	return nil
}

func init() {
	Cmd.AddCommand(syncCmd)
}
