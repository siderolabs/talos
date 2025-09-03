// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package create

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	clustercmd "github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster"
	"github.com/siderolabs/talos/pkg/cli"
	"github.com/siderolabs/talos/pkg/images"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/version"
	"github.com/siderolabs/talos/pkg/provision/providers"
)

const emptySchemanticID = "376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba"

type createQemuOps struct {
	schematicID     string
	imageFactoryURL string
}

func getDefaultQemuOptions() qemuOps {
	return qemuOps{
		preallocateDisks:  false,
		networkIPv6:       false,
		bootloaderEnabled: true,
		uefiEnabled:       true,
		nameservers:       []string{"8.8.8.8", "1.1.1.1", "2001:4860:4860::8888", "2606:4700:4700::1111"},
		diskBlockSize:     512,
		targetArch:        runtime.GOARCH,
		cniBinPath:        []string{filepath.Join(clustercmd.DefaultCNIDir, "bin")},
		cniConfDir:        filepath.Join(clustercmd.DefaultCNIDir, "conf.d"),
		cniCacheDir:       filepath.Join(clustercmd.DefaultCNIDir, "cache"),
		cniBundleURL: fmt.Sprintf("https://github.com/%s/talos/releases/download/%s/talosctl-cni-bundle-%s.tar.gz",
			images.Username, version.Trim(version.Tag), constants.ArchVariable),
		// asd
	}
}

func init() {
	cqOps := createQemuOps{}
	ops := &createOps{
		common: getDefaultCommonOptions(),
		qemu:   getDefaultQemuOptions(),
	}
	ops.common.skipInjectingConfig = true
	ops.common.applyConfigEnabled = true

	commonFlags := getCommonUserFacingFlags(&ops.common)
	addControlplanesFlag(commonFlags, &ops.common.controlplanes)
	addTalosVersionFlag(commonFlags, &ops.common.talosVersion, "the desired talos version")
	commonFlags.StringVar(&ops.common.networkCIDR, networkCIDRFlagName, "10.5.0.0/24", "CIDR of the cluster network")

	getQemuFlags := func() *pflag.FlagSet {
		qemu := pflag.NewFlagSet("qemu", pflag.PanicOnError)

		addDisksFlag(qemu, &ops.qemu.disks, []string{"virtio:10GB", "virtio:6GB"})
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

				data, err := getQemuClusterRequest(ctx, ops.common, ops.qemu, cqOps, provisioner)
				if err != nil {
					return err
				}

				cluster, err := provisioner.Create(ctx, data.clusterRequest, data.provisionOptions...)
				if err != nil {
					return err
				}

				err = postCreate(ctx, ops.common, data.talosconfig, cluster, data.provisionOptions, data.clusterRequest)
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
