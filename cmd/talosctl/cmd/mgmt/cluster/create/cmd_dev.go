// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package create

import (
	"context"
	"fmt"
	"runtime"
	"slices"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	clustercmd "github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster"
	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create/clusterops"
	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create/flags"
	"github.com/siderolabs/talos/cmd/talosctl/pkg/mgmt/helpers"
	"github.com/siderolabs/talos/pkg/cli"
	"github.com/siderolabs/talos/pkg/images"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

type legacyOps struct {
	clusterDiskSize   int
	extraDisks        int
	extraDiskSize     int
	extraDisksDrivers []string
}

var (
	createCmd    = getCreateCmd("create", true)
	createDevCmd = getCreateCmd("dev", false)
)

//nolint:gocyclo
func getCreateCmd(cmdName string, hidden bool) *cobra.Command {
	const (
		networkIPv4Flag               = "ipv4"
		networkIPv6Flag               = "ipv6"
		networkNoMasqueradeCIDRsFlag  = "no-masquerade-cidrs"
		nameserversFlag               = "nameservers"
		preallocateDisksFlag          = "disk-preallocate"
		clusterUserVolumesFlag        = "user-volumes"
		clusterDiskSizeFlag           = "disk"
		diskBlockSizeFlag             = "disk-block-size"
		useVIPFlag                    = "use-vip"
		bootloaderEnabledFlag         = "with-bootloader"
		controlPlanePortFlag          = "control-plane-port"
		firewallFlag                  = "with-firewall"
		tpmEnabledFlag                = "with-tpm1_2"
		tpm2EnabledFlag               = "with-tpm2"
		withDebugShellFlag            = "with-debug-shell"
		withIOMMUFlag                 = "with-iommu"
		talosconfigFlag               = "talosconfig"
		applyConfigEnabledFlag        = "with-apply-config"
		wireguardCIDRFlag             = "wireguard-cidr"
		controlPlaneCpusFlag          = "cpus"
		workersCpusFlag               = "cpus-workers"
		controlPlaneMemoryFlag        = "memory"
		workersMemoryFlag             = "memory-workers"
		clusterWaitFlag               = "wait"
		clusterWaitTimeoutFlag        = "wait-timeout"
		forceInitNodeAsEndpointFlag   = "init-node-as-endpoint"
		withInitNodeFlag              = "with-init-node"
		skipKubeconfigFlag            = "skip-kubeconfig"
		skipInjectingConfigFlag       = "skip-injecting-config"
		configPatchFlag               = "config-patch"
		configPatchControlPlaneFlag   = "config-patch-control-plane"
		configPatchWorkerFlag         = "config-patch-worker"
		skipK8sNodeReadinessCheckFlag = "skip-k8s-node-readiness-check"
		withJSONLogsFlag              = "with-json-logs"
		nodeVmlinuzPathFlag           = "vmlinuz-path"
		nodeISOPathFlag               = "iso-path"
		nodeUSBPathFlag               = "usb-path"
		nodeUKIPathFlag               = "uki-path"
		nodeInitramfsPathFlag         = "initrd-path"
		nodeDiskImagePathFlag         = "disk-image-path"
		nodeIPXEBootScriptFlag        = "ipxe-boot-script"
		uefiEnabledFlag               = "with-uefi"
		extraUEFISearchPathsFlag      = "extra-uefi-search-paths"
		extraDisksFlag                = "extra-disks"
		extraDisksDriversFlag         = "extra-disks-drivers"
		extraDiskSizeFlag             = "extra-disks-size"
		targetArchFlag                = "arch"
		cniBinPathFlag                = "cni-bin-path"
		cniConfDirFlag                = "cni-conf-dir"
		cniCacheDirFlag               = "cni-cache-dir"
		cniBundleURLFlag              = "cni-bundle-url"
		badRTCFlag                    = "bad-rtc"
		extraBootKernelArgsFlag       = "extra-boot-kernel-args"
		dhcpSkipHostnameFlag          = "disable-dhcp-hostname"
		networkChaosFlag              = "with-network-chaos"
		jitterFlag                    = "with-network-jitter"
		latencyFlag                   = "with-network-latency"
		packetLossFlag                = "with-network-packet-loss"
		packetReorderFlag             = "with-network-packet-reorder"
		packetCorruptFlag             = "with-network-packet-corrupt"
		bandwidthFlag                 = "with-network-bandwidth"
		withUUIDHostnamesFlag         = "with-uuid-hostnames"
		withSiderolinkAgentFlag       = "with-siderolink"
		configInjectionMethodFlag     = "config-injection-method"
		airgappedFlag                 = "airgapped"
		imageCachePathFlag            = "image-cache-path"
		imageCacheTLSCertFileFlag     = "image-cache-tls-cert-file"
		imageCacheTLSKeyFileFlag      = "image-cache-tls-key-file"
		imageCachePortFlag            = "image-cache-port"

		// The following flags are the gen options - the options that are only used in machine configuration (i.e., not during the qemu/docker provisioning).
		// They are not applicable when no machine configuration is generated, hence mutually exclusive with the --input-dir flag.

		nodeInstallImageFlag          = "install-image"
		configDebugFlag               = "with-debug"
		dnsDomainFlag                 = "dns-domain"
		withClusterDiscoveryFlag      = "with-cluster-discovery"
		registryInsecureFlag          = "registry-insecure-skip-verify"
		customCNIUrlFlag              = "custom-cni-url"
		encryptStatePartitionFlag     = "encrypt-state"
		encryptEphemeralPartitionFlag = "encrypt-ephemeral"
		encryptUserVolumeFlag         = "encrypt-user-volumes"
		enableKubeSpanFlag            = "with-kubespan"
		forceEndpointFlag             = "endpoint"
		kubePrismFlag                 = "kubeprism-port"
		diskEncryptionKeyTypesFlag    = "disk-encryption-key-types"
	)

	unImplementedFlagsDarwin := []string{
		networkNoMasqueradeCIDRsFlag,
		cniBinPathFlag,
		cniConfDirFlag,
		cniCacheDirFlag,
		cniBundleURLFlag,
		badRTCFlag,
		networkChaosFlag,
		jitterFlag,
		latencyFlag,
		packetLossFlag,
		packetReorderFlag,
		packetCorruptFlag,
		bandwidthFlag,
		airgappedFlag,

		// The following might work but need testing first.
		configInjectionMethodFlag,
	}

	qOps := clusterops.GetQemu()
	cOps := clusterops.GetCommon()
	legacyOps := legacyOps{}

	getCommonFlags := func() *pflag.FlagSet {
		common := pflag.NewFlagSet("common", pflag.PanicOnError)

		addControlplaneCpusFlag(common, &cOps.ControlplaneResources.CPU, controlPlaneCpusFlag)
		addWorkersCpusFlag(common, &cOps.WorkerResources.CPU, workersCpusFlag)
		addControlPlaneMemoryFlag(common, &cOps.ControlplaneResources.Memory, controlPlaneMemoryFlag)
		addWorkersMemoryFlag(common, &cOps.WorkerResources.Memory, workersMemoryFlag)

		addWorkersFlag(common, &cOps.Workers)
		addControlplanesFlag(common, &cOps.Controlplanes)
		addKubernetesVersionFlag(common, &cOps.KubernetesVersion)
		addTalosconfigDestinationFlag(common, &cOps.TalosconfigDestination, talosconfigFlag)
		addConfigPatchFlag(common, &cOps.ConfigPatch, configPatchFlag)
		addConfigPatchControlPlaneFlag(common, &cOps.ConfigPatchControlPlane, configPatchControlPlaneFlag)
		addConfigPatchWorkerFlag(common, &cOps.ConfigPatchWorker, configPatchWorkerFlag)
		addRegistryMirrorFlag(common, &cOps.RegistryMirrors)
		addNetworkMTUFlag(common, &cOps.NetworkMTU)
		addTalosVersionFlag(common, &cOps.TalosVersion, "the desired Talos version to generate config for")

		common.StringVar(&cOps.NetworkCIDR, networkCIDRFlagName, cOps.NetworkCIDR, "CIDR of the cluster network (IPv4, ULA network for IPv6 is derived in automated way)")
		common.StringVar(&cOps.WireguardCIDR, wireguardCIDRFlag, cOps.WireguardCIDR, "CIDR of the wireguard network")
		common.BoolVar(&cOps.ApplyConfigEnabled, applyConfigEnabledFlag, cOps.ApplyConfigEnabled, "enable apply config when the VM is starting in maintenance mode")
		common.StringSliceVar(&cOps.RegistryInsecure, registryInsecureFlag, cOps.RegistryInsecure, "list of registry hostnames to skip TLS verification for")
		common.IntVar(&cOps.ControlPlanePort, controlPlanePortFlag, cOps.ControlPlanePort, "control plane port (load balancer and local API port)")
		common.BoolVar(&cOps.ConfigDebug, configDebugFlag, cOps.ConfigDebug, "enable debug in Talos config to send service logs to the console")
		common.BoolVar(&cOps.NetworkIPv4, networkIPv4Flag, cOps.NetworkIPv4, "enable IPv4 network in the cluster")
		common.BoolVar(&cOps.ClusterWait, clusterWaitFlag, cOps.ClusterWait, "wait for the cluster to be ready before returning")
		common.DurationVar(&cOps.ClusterWaitTimeout, clusterWaitTimeoutFlag, cOps.ClusterWaitTimeout, "timeout to wait for the cluster to be ready")
		common.BoolVar(&cOps.ForceInitNodeAsEndpoint, forceInitNodeAsEndpointFlag, cOps.ForceInitNodeAsEndpoint, "use init node as endpoint instead of any load balancer endpoint")
		common.StringVar(&cOps.ForceEndpoint, forceEndpointFlag, cOps.ForceEndpoint, "use endpoint instead of provider defaults")
		common.BoolVar(&cOps.WithInitNode, withInitNodeFlag, cOps.WithInitNode, "create the cluster with an init node")
		common.StringVar(&cOps.CustomCNIUrl, customCNIUrlFlag, cOps.CustomCNIUrl, "install custom CNI from the URL (Talos cluster)")
		common.StringVar(&cOps.DNSDomain, dnsDomainFlag, cOps.DNSDomain, "the dns domain to use for cluster")
		common.BoolVar(&cOps.SkipKubeconfig, skipKubeconfigFlag, cOps.SkipKubeconfig, "skip merging kubeconfig from the created cluster")
		common.BoolVar(&cOps.SkipInjectingConfig, skipInjectingConfigFlag, cOps.SkipInjectingConfig,
			"skip injecting config from embedded metadata server, write config files to current directory")
		common.BoolVar(&cOps.EnableClusterDiscovery, withClusterDiscoveryFlag, cOps.EnableClusterDiscovery, "enable cluster discovery")
		common.BoolVar(&cOps.EnableKubeSpan, enableKubeSpanFlag, cOps.EnableKubeSpan, "enable KubeSpan system")
		common.IntVar(&cOps.KubePrismPort, kubePrismFlag, cOps.KubePrismPort, "KubePrism port (set to 0 to disable)")
		common.BoolVar(&cOps.SkipK8sNodeReadinessCheck, skipK8sNodeReadinessCheckFlag, cOps.SkipK8sNodeReadinessCheck, "skip k8s node readiness checks")
		common.BoolVar(&cOps.WithJSONLogs, withJSONLogsFlag, cOps.WithJSONLogs, "enable JSON logs receiver and configure Talos to send logs there")
		common.BoolVar(&cOps.WithUUIDHostnames, withUUIDHostnamesFlag, cOps.WithUUIDHostnames, "use machine UUIDs as default hostnames")
		common.BoolVar(&cOps.NetworkIPv6, networkIPv6Flag, cOps.NetworkIPv6, "enable IPv6 network in the cluster")

		return common
	}

	getQemuFlags := func() *pflag.FlagSet {
		qemu := pflag.NewFlagSet("qemu", pflag.PanicOnError)

		qemu.BoolVar(&qOps.PreallocateDisks, preallocateDisksFlag, true, "whether disk space should be preallocated")
		qemu.StringSliceVar(&qOps.ClusterUserVolumes, clusterUserVolumesFlag, qOps.ClusterUserVolumes, "list of user volumes to create for each VM in format: <name1>:<size1>:<name2>:<size2>")
		qemu.StringVar(&qOps.NodeInstallImage, nodeInstallImageFlag, helpers.DefaultImage(images.DefaultInstallerImageRepository), "the installer image to use")
		qemu.StringVar(&qOps.NodeVmlinuzPath, nodeVmlinuzPathFlag, helpers.ArtifactPath(constants.KernelAssetWithArch), "the compressed kernel image to use")
		qemu.StringVar(&qOps.NodeISOPath, nodeISOPathFlag, qOps.NodeISOPath, "the ISO path to use for the initial boot")
		qemu.StringVar(&qOps.NodeUSBPath, nodeUSBPathFlag, qOps.NodeUSBPath, "the USB stick image path to use for the initial boot")
		qemu.StringVar(&qOps.NodeUKIPath, nodeUKIPathFlag, qOps.NodeUKIPath, "the UKI image path to use for the initial boot")
		qemu.StringVar(&qOps.NodeInitramfsPath, nodeInitramfsPathFlag, helpers.ArtifactPath(constants.InitramfsAssetWithArch), "initramfs image to use")
		qemu.StringVar(&qOps.NodeDiskImagePath, nodeDiskImagePathFlag, qOps.NodeDiskImagePath, "disk image to use")
		qemu.StringVar(&qOps.NodeIPXEBootScript, nodeIPXEBootScriptFlag, qOps.NodeIPXEBootScript, "iPXE boot script (URL) to use")
		qemu.BoolVar(&qOps.BootloaderEnabled, bootloaderEnabledFlag, qOps.BootloaderEnabled, "enable bootloader to load kernel and initramfs from disk image after install")
		qemu.BoolVar(&qOps.UefiEnabled, uefiEnabledFlag, qOps.UefiEnabled, "enable UEFI on x86_64 architecture")
		qemu.BoolVar(&qOps.Tpm1_2Enabled, tpmEnabledFlag, qOps.Tpm1_2Enabled, "enable TPM 1.2 emulation support using swtpm")
		qemu.BoolVar(&qOps.Tpm2Enabled, tpm2EnabledFlag, qOps.Tpm2Enabled, "enable TPM 2.0 emulation support using swtpm")
		qemu.BoolVar(&qOps.DebugShellEnabled, withDebugShellFlag, qOps.DebugShellEnabled, "drop talos into a maintenance shell on boot, this is for advanced debugging for developers only")
		qemu.BoolVar(&qOps.WithIOMMU, withIOMMUFlag, qOps.WithIOMMU, "enable IOMMU support, this also add a new PCI root port and an interface attached to it")
		qemu.MarkHidden("with-debug-shell") //nolint:errcheck
		qemu.StringSliceVar(&qOps.ExtraUEFISearchPaths, extraUEFISearchPathsFlag, qOps.ExtraUEFISearchPaths, "additional search paths for UEFI firmware (only applies when UEFI is enabled)")
		qemu.StringSliceVar(&qOps.NetworkNoMasqueradeCIDRs, networkNoMasqueradeCIDRsFlag, qOps.NetworkNoMasqueradeCIDRs, "list of CIDRs to exclude from NAT")
		qemu.StringSliceVar(&qOps.Nameservers, nameserversFlag, qOps.Nameservers, "list of nameservers to use, by default use embedded DNS forwarder")
		qemu.UintVar(&qOps.DiskBlockSize, diskBlockSizeFlag, qOps.DiskBlockSize, "disk block size")
		qemu.StringVar(&qOps.TargetArch, targetArchFlag, qOps.TargetArch, "cluster architecture")
		qemu.StringSliceVar(&qOps.CniBinPath, cniBinPathFlag, qOps.CniBinPath, "search path for CNI binaries")
		qemu.StringVar(&qOps.CniConfDir, cniConfDirFlag, qOps.CniConfDir, "CNI config directory path")
		qemu.StringVar(&qOps.CniCacheDir, cniCacheDirFlag, qOps.CniCacheDir, "CNI cache directory path")
		qemu.StringVar(&qOps.CniBundleURL, cniBundleURLFlag, qOps.CniBundleURL, "URL to download CNI bundle from")
		qemu.BoolVar(&qOps.EncryptStatePartition, encryptStatePartitionFlag, qOps.EncryptStatePartition, "enable state partition encryption")
		qemu.BoolVar(&qOps.EncryptEphemeralPartition, encryptEphemeralPartitionFlag, qOps.EncryptEphemeralPartition, "enable ephemeral partition encryption")
		qemu.BoolVar(&qOps.EncryptUserVolumes, encryptUserVolumeFlag, qOps.EncryptUserVolumes, "enable ephemeral partition encryption")
		qemu.StringArrayVar(&qOps.DiskEncryptionKeyTypes, diskEncryptionKeyTypesFlag, []string{"uuid"}, "encryption key types to use for disk encryption (uuid, kms)")
		qemu.BoolVar(&qOps.UseVIP, useVIPFlag, qOps.UseVIP, "use a virtual IP for the controlplane endpoint instead of the loadbalancer")
		qemu.BoolVar(&qOps.BadRTC, badRTCFlag, qOps.BadRTC, "launch VM with bad RTC state")
		qemu.StringVar(&qOps.ExtraBootKernelArgs, extraBootKernelArgsFlag, qOps.ExtraBootKernelArgs, "add extra kernel args to the initial boot from vmlinuz and initramfs")
		qemu.BoolVar(&qOps.DHCPSkipHostname, dhcpSkipHostnameFlag, qOps.DHCPSkipHostname, "skip announcing hostname via DHCP")
		qemu.BoolVar(&qOps.NetworkChaos, networkChaosFlag, qOps.NetworkChaos, "enable to use network chaos parameters")
		qemu.DurationVar(&qOps.Jjitter, jitterFlag, qOps.Jjitter, "specify jitter on the bridge interface")
		qemu.DurationVar(&qOps.Latency, latencyFlag, qOps.Latency, "specify latency on the bridge interface")
		qemu.Float64Var(&qOps.PacketLoss, packetLossFlag, qOps.PacketLoss,
			"specify percent of packet loss on the bridge interface. e.g. 50% = 0.50 (default: 0.0)")
		qemu.Float64Var(&qOps.PacketReorder, packetReorderFlag, qOps.PacketReorder,
			"specify percent of reordered packets on the bridge interface. e.g. 50% = 0.50 (default: 0.0)")
		qemu.Float64Var(&qOps.PacketCorrupt, packetCorruptFlag, qOps.PacketCorrupt,
			"specify percent of corrupt packets on the bridge interface. e.g. 50% = 0.50 (default: 0.0)")
		qemu.IntVar(&qOps.Bandwidth, bandwidthFlag, qOps.Bandwidth, "specify bandwidth restriction (in kbps) on the bridge interface")
		qemu.StringVar(&qOps.WithFirewall, firewallFlag, qOps.WithFirewall, "inject firewall rules into the cluster, value is default policy - accept/block")
		qemu.Var(&qOps.WithSiderolinkAgent, withSiderolinkAgentFlag,
			"enables the use of siderolink agent as configuration apply mechanism. `true` or `wireguard` enables the agent, `tunnel` enables the agent with grpc tunneling")
		qemu.StringVar(&qOps.ConfigInjectionMethod,
			configInjectionMethodFlag, qOps.ConfigInjectionMethod, "a method to inject machine config: default is HTTP server, 'metal-iso' to mount an ISO")
		qemu.BoolVar(&qOps.Airgapped, airgappedFlag, qOps.Airgapped, "limit VM network access to the provisioning network only")
		qemu.StringVar(&qOps.ImageCachePath, imageCachePathFlag, qOps.ImageCachePath, "path to image cache")
		qemu.StringVar(&qOps.ImageCacheTLSCertFile, imageCacheTLSCertFileFlag, qOps.ImageCacheTLSCertFile, "path to image cache TLS cert")
		qemu.StringVar(&qOps.ImageCacheTLSKeyFile, imageCacheTLSKeyFileFlag, qOps.ImageCacheTLSKeyFile, "path to image cache TLS key")
		qemu.Uint16Var(&qOps.ImageCachePort, imageCachePortFlag, qOps.ImageCachePort, "port on which to serve image cache")

		return qemu
	}

	// createCmd is the developer oriented create command.
	createCmd := &cobra.Command{
		Use:    cmdName,
		Hidden: hidden,
		Short:  "Creates a local qemu based cluster for Talos development",
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cli.WithContext(context.Background(), func(ctx context.Context) error {
				if cmdName == "create" {
					cli.Warning("the developer oriented 'cluster create' command has been moved to 'cluster create dev'")
				}

				if err := validateQemuFlags(cmd.Flags(), unImplementedFlagsDarwin); err != nil {
					return err
				}

				disks := fmt.Sprintf("virtio:%d", legacyOps.clusterDiskSize)

				for i := range legacyOps.extraDisks {
					driver := "ide"

					// ide driver is not supported on arm64
					if qOps.TargetArch == "arm64" {
						driver = "virtio"
					}

					if i < len(legacyOps.extraDisksDrivers) {
						driver = legacyOps.extraDisksDrivers[i]
					}

					disks += fmt.Sprintf(",%s:%d", driver, legacyOps.extraDiskSize)
				}

				qOps.Disks = flags.Disks{}

				if err := qOps.Disks.Set(disks); err != nil {
					return err
				}

				return createDevCluster(ctx, cOps, qOps)
			})
		},
	}
	createCmd.Flags().IntVar(&legacyOps.clusterDiskSize, clusterDiskSizeFlag, 6*1024, "default limit on disk size in MB (each VM)")
	createCmd.Flags().IntVar(&legacyOps.extraDisks, extraDisksFlag, 0, "number of extra disks to create for each worker VM")
	createCmd.Flags().StringSliceVar(&legacyOps.extraDisksDrivers, "extra-disks-drivers", nil, "driver for each extra disk (virtio, ide, ahci, scsi, nvme, megaraid)")
	createCmd.Flags().IntVar(&legacyOps.extraDiskSize, extraDiskSizeFlag, 5*1024, "default limit on disk size in MB (each VM)")

	clustercmd.AddProvisionerFlag(createCmd)
	cli.Should(createCmd.Flags().MarkHidden(clustercmd.ProvisionerFlagName))

	createCmd.Flags().AddFlagSet(getCommonFlags())
	createCmd.Flags().AddFlagSet(getQemuFlags())
	addOmniJoinTokenFlag(createCmd, &cOps.OmniAPIEndpoint, configPatchFlag, configPatchWorkerFlag, configPatchControlPlaneFlag)

	createCmd.MarkFlagsMutuallyExclusive(tpmEnabledFlag, tpm2EnabledFlag)

	hideUnimplementedQemuFlags(createCmd, unImplementedFlagsDarwin)

	return createCmd
}

func init() {
	createCmd.AddCommand(createDevCmd)
	clustercmd.Cmd.AddCommand(createCmd)
}

func validateQemuFlags(allCmdFlags *pflag.FlagSet, unImplementedQemuFlagsDarwin []string) error {
	errMsg := ""

	allCmdFlags.VisitAll(func(f *pflag.Flag) {
		if f.Changed {
			if runtime.GOOS == "darwin" && slices.Contains(unImplementedQemuFlagsDarwin, f.Name) {
				errMsg += fmt.Sprintf("the \"%s\" flag is not supported on macos\n", f.Name)
			}
		}
	})

	if errMsg != "" {
		return fmt.Errorf("%sinvalid flags found", errMsg)
	}

	return nil
}

func hideUnimplementedQemuFlags(cmd *cobra.Command, unImplementedQemuFlagsDarwin []string) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if runtime.GOOS != "darwin" {
			return
		}

		for _, unimplemented := range unImplementedQemuFlagsDarwin {
			if f.Name == unimplemented {
				f.Hidden = true
			}
		}
	})
}
