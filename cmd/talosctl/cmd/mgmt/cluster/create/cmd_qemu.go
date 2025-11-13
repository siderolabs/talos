// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package create

import (
	"context"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/siderolabs/talos/cmd/talosctl/cmd/constants"
	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create/clusterops"
	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create/clusterops/configmaker/preset"
	"github.com/siderolabs/talos/pkg/cli"
	"github.com/siderolabs/talos/pkg/provision/providers"
)

type presetOptions struct {
	schematicID     string
	imageFactoryURL string
	presets         []string
}

func init() {
	presetOptions := presetOptions{}
	qOps := clusterops.GetQemu()
	cOps := clusterops.GetCommon()
	cOps.SkipInjectingConfig = true
	cOps.ApplyConfigEnabled = true

	commonFlags := getCommonUserFacingFlags(&cOps)
	addControlplanesFlag(commonFlags, &cOps.Controlplanes)
	addTalosVersionFlag(commonFlags, &cOps.TalosVersion, "the desired talos version")
	commonFlags.StringVar(&cOps.NetworkCIDR, networkCIDRFlagName, "10.5.0.0/24", "CIDR of the cluster network")

	getQemuFlags := func() *pflag.FlagSet {
		qemu := pflag.NewFlagSet("qemu", pflag.PanicOnError)

		addDisksFlag(qemu, &qOps.Disks)
		qemu.StringVar(&presetOptions.schematicID, "schematic-id", "", "image factory schematic id (defaults to an empty schematic)")
		qemu.StringVar(&presetOptions.imageFactoryURL, "image-factory-url", constants.ImageFactoryURL, "image factory url")
		qemu.StringSliceVar(&presetOptions.presets, "presets", []string{preset.ISO{}.Name()}, "list of presets to apply")

		return qemu
	}

	var cmdDescription strings.Builder
	cmdDescription.WriteString("Create a local QEMU based Talos cluster\n")

	cmdDescription.WriteString("Available presets:\n")

	for _, p := range preset.Presets {
		cmdDescription.WriteString("  - " + p.Name() + ": " + p.Description() + "\n")
	}

	cmdDescription.WriteString("\n")
	cmdDescription.WriteString("Note: exactly one of 'iso', 'iso-secureboot', 'pxe' or 'disk-image' presets must be specified.\n")

	createQemuCmd := &cobra.Command{
		Use:   providers.QemuProviderName,
		Short: cmdDescription.String(),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cli.WithContext(context.Background(), func(ctx context.Context) error {
				provisioner, err := providers.Factory(ctx, providers.QemuProviderName)
				if err != nil {
					return err
				}

				return createQemuCluster(ctx, qOps, cOps, presetOptions, provisioner)
			})
		},
	}

	createQemuCmd.Flags().AddFlagSet(commonFlags)
	createQemuCmd.Flags().AddFlagSet(getQemuFlags())
	addOmniJoinTokenFlag(createQemuCmd, &cOps.OmniAPIEndpoint, configPatchFlagName, configPatchWorkerFlagName, configPatchControlPlaneFlagName)

	createCmd.AddCommand(createQemuCmd)
}
