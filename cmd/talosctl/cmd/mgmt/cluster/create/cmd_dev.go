// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package create

import (
	"context"
	"fmt"
	"runtime"
	"slices"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	clustercmd "github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster"
	"github.com/siderolabs/talos/cmd/talosctl/pkg/mgmt/helpers"
	"github.com/siderolabs/talos/pkg/cli"
	"github.com/siderolabs/talos/pkg/images"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

type qemuOps struct {
	nodeInstallImage          string
	nodeVmlinuzPath           string
	nodeInitramfsPath         string
	nodeISOPath               string
	nodeUSBPath               string
	nodeUKIPath               string
	nodeDiskImagePath         string
	nodeIPXEBootScript        string
	bootloaderEnabled         bool
	uefiEnabled               bool
	tpm1_2Enabled             bool
	tpm2Enabled               bool
	extraUEFISearchPaths      []string
	networkNoMasqueradeCIDRs  []string
	nameservers               []string
	disks                     []string
	diskBlockSize             uint
	preallocateDisks          bool
	clusterUserVolumes        []string
	targetArch                string
	cniBinPath                []string
	cniConfDir                string
	cniCacheDir               string
	cniBundleURL              string
	encryptStatePartition     bool
	encryptEphemeralPartition bool
	encryptUserVolumes        bool
	useVIP                    bool
	badRTC                    bool
	extraBootKernelArgs       string
	dhcpSkipHostname          bool
	networkChaos              bool
	jitter                    time.Duration
	latency                   time.Duration
	packetLoss                float64
	packetReorder             float64
	packetCorrupt             float64
	bandwidth                 int
	diskEncryptionKeyTypes    []string
	withFirewall              string
	withSiderolinkAgent       agentFlag
	debugShellEnabled         bool
	withIOMMU                 bool
	configInjectionMethod     string
	networkIPv6               bool
}

type legacyOps struct {
	clusterDiskSize   int
	extraDisks        int
	extraDiskSize     int
	extraDisksDrivers []string
}

type createOps struct {
	common commonOps
	docker dockerOps
	qemu   qemuOps
}

var createCmd = getCreateCmd()

//nolint:gocyclo
func getCreateCmd() *cobra.Command {
	const (
		inputDirFlag                  = "input-dir"
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

		// The following might work but need testing first.
		configInjectionMethodFlag,
	}

	ops := &createOps{
		common: getDefaultCommonOptions(),
		qemu:   getDefaultQemuOptions(),
	}
	legacyOps := legacyOps{}

	getCommonFlags := func() *pflag.FlagSet {
		common := pflag.NewFlagSet("common", pflag.PanicOnError)

		addControlplaneCpusFlag(common, &ops.common.controlplaneResources.cpu, controlPlaneCpusFlag)
		addWorkersCpusFlag(common, &ops.common.workerResources.cpu, workersCpusFlag)
		addControlPlaneMemoryFlag(common, &ops.common.controlplaneResources.memory, controlPlaneMemoryFlag)
		addWorkersMemoryFlag(common, &ops.common.workerResources.memory, workersMemoryFlag)

		addWorkersFlag(common, &ops.common.workers)
		addControlplanesFlag(common, &ops.common.controlplanes)
		addKubernetesVersionFlag(common, &ops.common.kubernetesVersion)
		addTalosconfigDestinationFlag(common, &ops.common.talosconfigDestination, talosconfigFlag)
		addConfigPatchFlag(common, &ops.common.configPatch, configPatchFlag)
		addConfigPatchControlPlaneFlag(common, &ops.common.configPatchControlPlane, configPatchControlPlaneFlag)
		addConfigPatchWorkerFlag(common, &ops.common.configPatchWorker, configPatchWorkerFlag)
		addRegistryMirrorFlag(common, &ops.common.registryMirrors)
		addNetworkMTUFlag(common, &ops.common.networkMTU)
		addTalosVersionFlag(common, &ops.common.talosVersion, "the desired Talos version to generate config for")

		common.StringVar(&ops.common.networkCIDR, networkCIDRFlagName, ops.common.networkCIDR, "CIDR of the cluster network (IPv4, ULA network for IPv6 is derived in automated way)")
		common.StringVar(&ops.common.wireguardCIDR, wireguardCIDRFlag, ops.common.wireguardCIDR, "CIDR of the wireguard network")
		common.BoolVar(&ops.common.applyConfigEnabled, applyConfigEnabledFlag, ops.common.applyConfigEnabled, "enable apply config when the VM is starting in maintenance mode")
		common.StringSliceVar(&ops.common.registryInsecure, registryInsecureFlag, ops.common.registryInsecure, "list of registry hostnames to skip TLS verification for")
		common.IntVar(&ops.common.controlPlanePort, controlPlanePortFlag, ops.common.controlPlanePort, "control plane port (load balancer and local API port)")
		common.BoolVar(&ops.common.configDebug, configDebugFlag, ops.common.configDebug, "enable debug in Talos config to send service logs to the console")
		common.BoolVar(&ops.common.networkIPv4, networkIPv4Flag, ops.common.networkIPv4, "enable IPv4 network in the cluster")
		common.BoolVar(&ops.common.clusterWait, clusterWaitFlag, ops.common.clusterWait, "wait for the cluster to be ready before returning")
		common.DurationVar(&ops.common.clusterWaitTimeout, clusterWaitTimeoutFlag, ops.common.clusterWaitTimeout, "timeout to wait for the cluster to be ready")
		common.BoolVar(&ops.common.forceInitNodeAsEndpoint, forceInitNodeAsEndpointFlag, ops.common.forceInitNodeAsEndpoint, "use init node as endpoint instead of any load balancer endpoint")
		common.StringVar(&ops.common.forceEndpoint, forceEndpointFlag, ops.common.forceEndpoint, "use endpoint instead of provider defaults")
		common.StringVarP(&ops.common.inputDir, inputDirFlag, "i", ops.common.inputDir, "location of pre-generated config files")
		common.BoolVar(&ops.common.withInitNode, withInitNodeFlag, ops.common.withInitNode, "create the cluster with an init node")
		common.StringVar(&ops.common.customCNIUrl, customCNIUrlFlag, ops.common.customCNIUrl, "install custom CNI from the URL (Talos cluster)")
		common.StringVar(&ops.common.dnsDomain, dnsDomainFlag, ops.common.dnsDomain, "the dns domain to use for cluster")
		common.BoolVar(&ops.common.skipKubeconfig, skipKubeconfigFlag, ops.common.skipKubeconfig, "skip merging kubeconfig from the created cluster")
		common.BoolVar(&ops.common.skipInjectingConfig, skipInjectingConfigFlag, ops.common.skipInjectingConfig,
			"skip injecting config from embedded metadata server, write config files to current directory")
		common.BoolVar(&ops.common.enableClusterDiscovery, withClusterDiscoveryFlag, ops.common.enableClusterDiscovery, "enable cluster discovery")
		common.BoolVar(&ops.common.enableKubeSpan, enableKubeSpanFlag, ops.common.enableKubeSpan, "enable KubeSpan system")
		common.IntVar(&ops.common.kubePrismPort, kubePrismFlag, ops.common.kubePrismPort, "KubePrism port (set to 0 to disable)")
		common.BoolVar(&ops.common.skipK8sNodeReadinessCheck, skipK8sNodeReadinessCheckFlag, ops.common.skipK8sNodeReadinessCheck, "skip k8s node readiness checks")
		common.BoolVar(&ops.common.withJSONLogs, withJSONLogsFlag, ops.common.withJSONLogs, "enable JSON logs receiver and configure Talos to send logs there")
		common.BoolVar(&ops.common.withUUIDHostnames, withUUIDHostnamesFlag, ops.common.withUUIDHostnames, "use machine UUIDs as default hostnames")

		return common
	}

	getQemuFlags := func() *pflag.FlagSet {
		qemu := pflag.NewFlagSet("qemu", pflag.PanicOnError)

		qemu.BoolVar(&ops.qemu.preallocateDisks, preallocateDisksFlag, true, "whether disk space should be preallocated")
		qemu.StringSliceVar(&ops.qemu.clusterUserVolumes, clusterUserVolumesFlag, ops.qemu.clusterUserVolumes, "list of user volumes to create for each VM in format: <name1>:<size1>:<name2>:<size2>")
		qemu.StringVar(&ops.qemu.nodeInstallImage, nodeInstallImageFlag, helpers.DefaultImage(images.DefaultInstallerImageRepository), "the installer image to use")
		qemu.StringVar(&ops.qemu.nodeVmlinuzPath, nodeVmlinuzPathFlag, helpers.ArtifactPath(constants.KernelAssetWithArch), "the compressed kernel image to use")
		qemu.StringVar(&ops.qemu.nodeISOPath, nodeISOPathFlag, ops.qemu.nodeISOPath, "the ISO path to use for the initial boot")
		qemu.StringVar(&ops.qemu.nodeUSBPath, nodeUSBPathFlag, ops.qemu.nodeUSBPath, "the USB stick image path to use for the initial boot")
		qemu.StringVar(&ops.qemu.nodeUKIPath, nodeUKIPathFlag, ops.qemu.nodeUKIPath, "the UKI image path to use for the initial boot")
		qemu.StringVar(&ops.qemu.nodeInitramfsPath, nodeInitramfsPathFlag, helpers.ArtifactPath(constants.InitramfsAssetWithArch), "initramfs image to use")
		qemu.StringVar(&ops.qemu.nodeDiskImagePath, nodeDiskImagePathFlag, ops.qemu.nodeDiskImagePath, "disk image to use")
		qemu.StringVar(&ops.qemu.nodeIPXEBootScript, nodeIPXEBootScriptFlag, ops.qemu.nodeIPXEBootScript, "iPXE boot script (URL) to use")
		qemu.BoolVar(&ops.qemu.bootloaderEnabled, bootloaderEnabledFlag, ops.qemu.bootloaderEnabled, "enable bootloader to load kernel and initramfs from disk image after install")
		qemu.BoolVar(&ops.qemu.uefiEnabled, uefiEnabledFlag, ops.qemu.uefiEnabled, "enable UEFI on x86_64 architecture")
		qemu.BoolVar(&ops.qemu.tpm1_2Enabled, tpmEnabledFlag, ops.qemu.tpm1_2Enabled, "enable TPM 1.2 emulation support using swtpm")
		qemu.BoolVar(&ops.qemu.tpm2Enabled, tpm2EnabledFlag, ops.qemu.tpm2Enabled, "enable TPM 2.0 emulation support using swtpm")
		qemu.BoolVar(&ops.qemu.debugShellEnabled, withDebugShellFlag, ops.qemu.debugShellEnabled, "drop talos into a maintenance shell on boot, this is for advanced debugging for developers only")
		qemu.BoolVar(&ops.qemu.withIOMMU, withIOMMUFlag, ops.qemu.withIOMMU, "enable IOMMU support, this also add a new PCI root port and an interface attached to it")
		qemu.MarkHidden("with-debug-shell") //nolint:errcheck
		qemu.StringSliceVar(&ops.qemu.extraUEFISearchPaths, extraUEFISearchPathsFlag, ops.qemu.extraUEFISearchPaths, "additional search paths for UEFI firmware (only applies when UEFI is enabled)")
		qemu.StringSliceVar(&ops.qemu.networkNoMasqueradeCIDRs, networkNoMasqueradeCIDRsFlag, ops.qemu.networkNoMasqueradeCIDRs, "list of CIDRs to exclude from NAT")
		qemu.StringSliceVar(&ops.qemu.nameservers, nameserversFlag, ops.qemu.nameservers, "list of nameservers to use")
		qemu.IntVar(&legacyOps.clusterDiskSize, clusterDiskSizeFlag, 6*1024, "default limit on disk size in MB (each VM)")
		qemu.UintVar(&ops.qemu.diskBlockSize, diskBlockSizeFlag, ops.qemu.diskBlockSize, "disk block size")
		qemu.IntVar(&legacyOps.extraDisks, extraDisksFlag, 0, "number of extra disks to create for each worker VM")
		qemu.StringSliceVar(&legacyOps.extraDisksDrivers, "extra-disks-drivers", nil, "driver for each extra disk (virtio, ide, ahci, scsi, nvme, megaraid)")
		qemu.IntVar(&legacyOps.extraDiskSize, extraDiskSizeFlag, 5*1024, "default limit on disk size in MB (each VM)")
		qemu.StringVar(&ops.qemu.targetArch, targetArchFlag, ops.qemu.targetArch, "cluster architecture")
		qemu.StringSliceVar(&ops.qemu.cniBinPath, cniBinPathFlag, ops.qemu.cniBinPath, "search path for CNI binaries")
		qemu.StringVar(&ops.qemu.cniConfDir, cniConfDirFlag, ops.qemu.cniConfDir, "CNI config directory path")
		qemu.StringVar(&ops.qemu.cniCacheDir, cniCacheDirFlag, ops.qemu.cniCacheDir, "CNI cache directory path")
		qemu.StringVar(&ops.qemu.cniBundleURL, cniBundleURLFlag, ops.qemu.cniBundleURL, "URL to download CNI bundle from")
		qemu.BoolVar(&ops.qemu.encryptStatePartition, encryptStatePartitionFlag, ops.qemu.encryptStatePartition, "enable state partition encryption")
		qemu.BoolVar(&ops.qemu.encryptEphemeralPartition, encryptEphemeralPartitionFlag, ops.qemu.encryptEphemeralPartition, "enable ephemeral partition encryption")
		qemu.BoolVar(&ops.qemu.encryptUserVolumes, encryptUserVolumeFlag, ops.qemu.encryptUserVolumes, "enable ephemeral partition encryption")
		qemu.StringArrayVar(&ops.qemu.diskEncryptionKeyTypes, diskEncryptionKeyTypesFlag, []string{"uuid"}, "encryption key types to use for disk encryption (uuid, kms)")
		qemu.BoolVar(&ops.qemu.networkIPv6, networkIPv6Flag, ops.qemu.networkIPv6, "enable IPv6 network in the cluster")
		qemu.BoolVar(&ops.qemu.useVIP, useVIPFlag, ops.qemu.useVIP, "use a virtual IP for the controlplane endpoint instead of the loadbalancer")
		qemu.BoolVar(&ops.qemu.badRTC, badRTCFlag, ops.qemu.badRTC, "launch VM with bad RTC state")
		qemu.StringVar(&ops.qemu.extraBootKernelArgs, extraBootKernelArgsFlag, ops.qemu.extraBootKernelArgs, "add extra kernel args to the initial boot from vmlinuz and initramfs")
		qemu.BoolVar(&ops.qemu.dhcpSkipHostname, dhcpSkipHostnameFlag, ops.qemu.dhcpSkipHostname, "skip announcing hostname via DHCP")
		qemu.BoolVar(&ops.qemu.networkChaos, networkChaosFlag, ops.qemu.networkChaos, "enable to use network chaos parameters")
		qemu.DurationVar(&ops.qemu.jitter, jitterFlag, ops.qemu.jitter, "specify jitter on the bridge interface")
		qemu.DurationVar(&ops.qemu.latency, latencyFlag, ops.qemu.latency, "specify latency on the bridge interface")
		qemu.Float64Var(&ops.qemu.packetLoss, packetLossFlag, ops.qemu.packetLoss,
			"specify percent of packet loss on the bridge interface. e.g. 50% = 0.50 (default: 0.0)")
		qemu.Float64Var(&ops.qemu.packetReorder, packetReorderFlag, ops.qemu.packetReorder,
			"specify percent of reordered packets on the bridge interface. e.g. 50% = 0.50 (default: 0.0)")
		qemu.Float64Var(&ops.qemu.packetCorrupt, packetCorruptFlag, ops.qemu.packetCorrupt,
			"specify percent of corrupt packets on the bridge interface. e.g. 50% = 0.50 (default: 0.0)")
		qemu.IntVar(&ops.qemu.bandwidth, bandwidthFlag, ops.qemu.bandwidth, "specify bandwidth restriction (in kbps) on the bridge interface")
		qemu.StringVar(&ops.qemu.withFirewall, firewallFlag, ops.qemu.withFirewall, "inject firewall rules into the cluster, value is default policy - accept/block")
		qemu.Var(&ops.qemu.withSiderolinkAgent, withSiderolinkAgentFlag,
			"enables the use of siderolink agent as configuration apply mechanism. `true` or `wireguard` enables the agent, `tunnel` enables the agent with grpc tunneling")
		qemu.StringVar(&ops.qemu.configInjectionMethod,
			configInjectionMethodFlag, ops.qemu.configInjectionMethod, "a method to inject machine config: default is HTTP server, 'metal-iso' to mount an ISO")

		return qemu
	}

	ops.common.rootOps = &clustercmd.PersistentFlags

	// createCmd is the developer oriented create command.
	createCmd := &cobra.Command{
		Use:    "create",
		Hidden: false, // todo: hide once user-facing commands are implemented
		Short:  "Creates a local qemu based cluster for Talos development",
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cli.WithContext(context.Background(), func(ctx context.Context) error {
				if err := validateQemuFlags(cmd.Flags(), unImplementedFlagsDarwin); err != nil {
					return err
				}

				ops.qemu.disks = append(ops.qemu.disks, fmt.Sprintf("virtio:%d", legacyOps.clusterDiskSize))

				for i := range legacyOps.extraDisks {
					driver := "ide"

					// ide driver is not supported on arm64
					if ops.qemu.targetArch == "arm64" {
						driver = "virtio"
					}

					if i < len(legacyOps.extraDisksDrivers) {
						driver = legacyOps.extraDisksDrivers[i]
					}

					ops.qemu.disks = append(ops.qemu.disks, fmt.Sprintf("%s:%d", driver, legacyOps.extraDiskSize))
				}

				return create(ctx, *ops)
			})
		},
	}

	clustercmd.AddProvisionerFlag(createCmd)
	cli.Should(createCmd.Flags().MarkHidden(clustercmd.ProvisionerFlag))

	createCmd.Flags().AddFlagSet(getCommonFlags())
	createCmd.Flags().AddFlagSet(getQemuFlags())

	createCmd.MarkFlagsMutuallyExclusive(inputDirFlag, nodeInstallImageFlag)
	createCmd.MarkFlagsMutuallyExclusive(inputDirFlag, configDebugFlag)
	createCmd.MarkFlagsMutuallyExclusive(inputDirFlag, dnsDomainFlag)
	createCmd.MarkFlagsMutuallyExclusive(inputDirFlag, withClusterDiscoveryFlag)
	createCmd.MarkFlagsMutuallyExclusive(inputDirFlag, registryMirrorFlagName)
	createCmd.MarkFlagsMutuallyExclusive(inputDirFlag, registryInsecureFlag)
	createCmd.MarkFlagsMutuallyExclusive(inputDirFlag, customCNIUrlFlag)
	createCmd.MarkFlagsMutuallyExclusive(inputDirFlag, talosVersionFlagName)
	createCmd.MarkFlagsMutuallyExclusive(inputDirFlag, encryptStatePartitionFlag)
	createCmd.MarkFlagsMutuallyExclusive(inputDirFlag, encryptEphemeralPartitionFlag)
	createCmd.MarkFlagsMutuallyExclusive(inputDirFlag, encryptUserVolumeFlag)
	createCmd.MarkFlagsMutuallyExclusive(inputDirFlag, enableKubeSpanFlag)
	createCmd.MarkFlagsMutuallyExclusive(inputDirFlag, forceEndpointFlag)
	createCmd.MarkFlagsMutuallyExclusive(inputDirFlag, kubePrismFlag)
	createCmd.MarkFlagsMutuallyExclusive(inputDirFlag, diskEncryptionKeyTypesFlag)

	createCmd.MarkFlagsMutuallyExclusive(tpmEnabledFlag, tpm2EnabledFlag)

	hideUnimplementedQemuFlags(createCmd, unImplementedFlagsDarwin)

	return createCmd
}

func init() {
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
