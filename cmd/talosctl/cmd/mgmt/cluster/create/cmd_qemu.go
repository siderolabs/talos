// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package create

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	clustercmd "github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster"
	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create/clusterops"
	"github.com/siderolabs/talos/pkg/cli"
	"github.com/siderolabs/talos/pkg/provision/providers"
)

const emptySchemanticID = "376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba"

type createQemuOps struct {
	schematicID     string
	imageFactoryURL string
}

func init() {
	cqOps := createQemuOps{}
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
		qemu.StringVar(&cqOps.schematicID, "schematic-id", "", "image factory schematic id (defaults to an empty schematic)")
		qemu.StringVar(&cqOps.imageFactoryURL, "image-factory-url", "https://factory.talos.dev/", "image factory url")

		return qemu
	}

	createQemuCmd := &cobra.Command{
		Use:   providers.QemuProviderName,
		Short: "Create a local QEMU based Talos cluster",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cli.WithContext(context.Background(), func(ctx context.Context) error {
				provisioner, err := providers.Factory(ctx, providers.QemuProviderName)
				if err != nil {
					return err
				}

				data, err := getQemuClusterRequest(ctx, qOps, cOps, cqOps, provisioner)
				if err != nil {
					return err
				}

				cluster, err := provisioner.Create(ctx, data.ClusterRequest, data.ProvisionOptions...)
				if err != nil {
					return err
				}

				err = postCreate(ctx, cOps, data.ConfigBundle.TalosCfg, cluster, data.ProvisionOptions, data.ClusterRequest)
				if err != nil {
					return err
				}

				return clustercmd.ShowCluster(cluster)
			})
		},
	}

	createQemuCmd.Flags().AddFlagSet(commonFlags)
	createQemuCmd.Flags().AddFlagSet(getQemuFlags())

	createCmd.AddCommand(createQemuCmd)
}
