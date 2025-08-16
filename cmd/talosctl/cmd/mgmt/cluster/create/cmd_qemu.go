// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package create

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"runtime"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	clustercmd "github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster"
	"github.com/siderolabs/talos/pkg/cli"
	"github.com/siderolabs/talos/pkg/images"
	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/version"
	"github.com/siderolabs/talos/pkg/provision/providers"
)

const emptySchemanticID = "376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba"

func init() {
	var (
		schematicID     string
		imageFactoryURL string
	)

	ops := &createOps{
		common: getDefaultCommonOptions(),
		qemu: qemuOps{
			preallocateDisks: false,
			uefiEnabled:      true,
			nameservers:      []string{"8.8.8.8", "1.1.1.1", "2001:4860:4860::8888", "2606:4700:4700::1111"},
			diskBlockSize:    512,
			targetArch:       runtime.GOARCH,
			cniBinPath:       []string{filepath.Join(clustercmd.DefaultCNIDir, "bin")},
			cniConfDir:       filepath.Join(clustercmd.DefaultCNIDir, "conf.d"),
			cniCacheDir:      filepath.Join(clustercmd.DefaultCNIDir, "cache"),
			cniBundleURL: fmt.Sprintf("https://github.com/%s/talos/releases/download/%s/talosctl-cni-bundle-%s.tar.gz",
				images.Username, version.Trim(version.Tag), constants.ArchVariable),
		},
	}
	ops.common.skipInjectingConfig = true
	ops.common.applyConfigEnabled = true

	commonFlags := getCommonUserFacingFlags(&ops.common)
	addControlplanesFlag(commonFlags, &ops.common.controlplanes)
	addTalosVersionFlag(commonFlags, &ops.common.talosVersion, "the desired talos version")
	commonFlags.StringVar(&ops.common.networkCIDR, networkCIDRFlagName, "10.5.0.0/24", "CIDR of the cluster network")

	getQemuFlags := func() *pflag.FlagSet {
		qemu := pflag.NewFlagSet("qemu", pflag.PanicOnError)

		addDisksFlag(qemu, &ops.qemu.disks, []string{"virtio:" + strconv.Itoa(10*1024), "virtio:" + strconv.Itoa(6*1024)})
		qemu.StringVar(&schematicID, "schematic-id", "", "image factory schematic id (defaults to an empty schematic)")
		qemu.StringVar(&imageFactoryURL, "image-factory-url", "https://factory.talos.dev/", "image factory url")

		return qemu
	}

	createQemuCmd := &cobra.Command{
		Use:   providers.QemuProviderName,
		Short: "Create a local QEMU based Talos cluster",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cli.WithContext(context.Background(), func(ctx context.Context) error {
				if ops.common.talosVersion == "" || ops.common.talosVersion[0] != 'v' {
					return fmt.Errorf("failed to parse talos version: version string must start with a 'v'")
				}

				_, err := config.ParseContractFromVersion(ops.common.talosVersion)
				if err != nil {
					return fmt.Errorf("failed to parse talos version: %s", err)
				}

				if schematicID == "" {
					schematicID = emptySchemanticID
				}

				factoryURL, err := url.Parse(imageFactoryURL)
				if err != nil {
					return fmt.Errorf("malformed Image Factory URL: %q: %w", imageFactoryURL, err)
				}

				if factoryURL.Scheme == "" || factoryURL.Host == "" {
					return fmt.Errorf("image Factory URL must include scheme and host: %q", imageFactoryURL)
				}

				ops.qemu.nodeISOPath, err = url.JoinPath(factoryURL.String(), "image", schematicID, ops.common.talosVersion, "metal-"+ops.qemu.targetArch+".iso")
				cli.Should(err)
				ops.qemu.nodeInstallImage, err = url.JoinPath(factoryURL.Host, "metal-installer", schematicID+":"+ops.common.talosVersion)
				cli.Should(err)

				return create(ctx, *ops)
			})
		},
	}

	createQemuCmd.Flags().AddFlagSet(commonFlags)
	createQemuCmd.Flags().AddFlagSet(getQemuFlags())

	createCmd.AddCommand(createQemuCmd)
}
