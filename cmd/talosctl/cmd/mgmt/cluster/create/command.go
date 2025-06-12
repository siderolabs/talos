// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package create

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"time"

	"github.com/docker/cli/opts"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	clustercmd "github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster"
	"github.com/siderolabs/talos/cmd/talosctl/pkg/mgmt/helpers"
	"github.com/siderolabs/talos/pkg/cli"
	"github.com/siderolabs/talos/pkg/images"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/version"
	"github.com/siderolabs/talos/pkg/provision/providers"
)

// commonOps are the options common between all the providers.
type commonOps struct {
	// RootOps are the options from the root cluster command
	rootOps                   *clustercmd.CmdOps
	talosconfig               string
	registryMirrors           []string
	registryInsecure          []string
	kubernetesVersion         string
	applyConfigEnabled        bool
	configDebug               bool
	networkCIDR               string
	networkMTU                int
	networkIPv4               bool
	dnsDomain                 string
	workers                   int
	controlplanes             int
	controlPlaneCpus          string
	workersCpus               string
	controlPlaneMemory        int
	workersMemory             int
	clusterWait               bool
	clusterWaitTimeout        time.Duration
	forceInitNodeAsEndpoint   bool
	forceEndpoint             string
	inputDir                  string
	controlPlanePort          int
	withInitNode              bool
	customCNIUrl              string
	skipKubeconfig            bool
	skipInjectingConfig       bool
	talosVersion              string
	enableKubeSpan            bool
	enableClusterDiscovery    bool
	configPatch               []string
	configPatchControlPlane   []string
	configPatchWorker         []string
	kubePrismPort             int
	skipK8sNodeReadinessCheck bool
	withJSONLogs              bool
	wireguardCIDR             string

	// IPv6 networking is suported only on qemu, but it doesn't make sense to separate the logic
	networkIPv6 bool
}

type qemuOps struct {
	nodeInstallImage             string
	nodeVmlinuzPath              string
	nodeInitramfsPath            string
	nodeISOPath                  string
	nodeUSBPath                  string
	nodeUKIPath                  string
	nodeDiskImagePath            string
	nodeIPXEBootScript           string
	bootloaderEnabled            bool
	uefiEnabled                  bool
	tpm1_2Enabled                bool
	tpm2Enabled                  bool
	extraUEFISearchPaths         []string
	networkNoMasqueradeCIDRs     []string
	nameservers                  []string
	clusterDiskSize              int
	diskBlockSize                uint
	clusterDiskPreallocate       bool
	clusterUserVolumes           []string
	extraDisks                   int
	extraDiskSize                int
	extraDisksDrivers            []string
	targetArch                   string
	cniBinPath                   []string
	cniConfDir                   string
	cniCacheDir                  string
	cniBundleURL                 string
	encryptStatePartition        bool
	encryptEphemeralPartition    bool
	encryptUserVolumes           bool
	useVIP                       bool
	badRTC                       bool
	extraBootKernelArgs          string
	dhcpSkipHostname             bool
	networkChaos                 bool
	jitter                       time.Duration
	latency                      time.Duration
	packetLoss                   float64
	packetReorder                float64
	packetCorrupt                float64
	bandwidth                    int
	diskEncryptionKeyTypes       []string
	withFirewall                 string
	withUUIDHostnames            bool
	withSiderolinkAgent          agentFlag
	debugShellEnabled            bool
	withIOMMU                    bool
	configInjectionMethodFlagVal string
}

type dockerOps struct {
	dockerHostIP      string
	dockerDisableIPv6 bool
	mountOpts         opts.MountOpt
	ports             string
	nodeImage         string
}

type createOps struct {
	common commonOps
	docker dockerOps
	qemu   qemuOps
}

type createFlags struct {
	common *pflag.FlagSet
	docker *pflag.FlagSet
	qemu   *pflag.FlagSet
}

func init() {
	const (
		dockerHostIPFlag              = "docker-host-ip"
		nodeImageFlag                 = "image"
		portsFlag                     = "exposed-ports"
		dockerDisableIPv6Flag         = "docker-disable-ipv6"
		mountOptsFlag                 = "mount"
		inputDirFlag                  = "input-dir"
		networkIPv4Flag               = "ipv4"
		networkIPv6Flag               = "ipv6"
		networkMTUFlag                = "mtu"
		networkCIDRFlag               = "cidr"
		networkNoMasqueradeCIDRsFlag  = "no-masquerade-cidrs"
		nameserversFlag               = "nameservers"
		clusterDiskPreallocateFlag    = "disk-preallocate"
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
		workersFlag                   = "workers"
		controlplanesFlag             = "controlplanes"
		controlPlaneCpusFlag          = "cpus"
		workersCpusFlag               = "cpus-workers"
		controlPlaneMemoryFlag        = "memory"
		workersMemoryFlag             = "memory-workers"
		clusterWaitFlag               = "wait"
		clusterWaitTimeoutFlag        = "wait-timeout"
		forceInitNodeAsEndpointFlag   = "init-node-as-endpoint"
		kubernetesVersionFlag         = "kubernetes-version"
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
		registryMirrorFlag            = "registry-mirror"
		registryInsecureFlag          = "registry-insecure-skip-verify"
		customCNIUrlFlag              = "custom-cni-url"
		talosVersionFlag              = "talos-version"
		encryptStatePartitionFlag     = "encrypt-state"
		encryptEphemeralPartitionFlag = "encrypt-ephemeral"
		encryptUserVolumeFlag         = "encrypt-user-volumes"
		enableKubeSpanFlag            = "with-kubespan"
		forceEndpointFlag             = "endpoint"
		kubePrismFlag                 = "kubeprism-port"
		diskEncryptionKeyTypesFlag    = "disk-encryption-key-types"
	)

	unImplementedQemuFlagsDarwin := []string{
		tpmEnabledFlag,
		tpm2EnabledFlag,
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
		common: commonOps{},
		docker: dockerOps{},
		qemu:   qemuOps{},
	}

	flags := createFlags{
		common: pflag.NewFlagSet("common", pflag.PanicOnError),
		qemu:   pflag.NewFlagSet("qemu", pflag.PanicOnError),
		docker: pflag.NewFlagSet("docker", pflag.PanicOnError),
	}

	ops.common.rootOps = &clustercmd.Flags

	// createCmd represents the cluster up command.
	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Creates a local docker-based or QEMU-based kubernetes cluster",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cli.WithContext(context.Background(), func(ctx context.Context) error {
				if err := providers.IsValidProvider(ops.common.rootOps.ProvisionerName); err != nil {
					return err
				}
				if err := validateProviderFlags(*ops, flags, unImplementedQemuFlagsDarwin); err != nil {
					return err
				}

				return create(ctx, *ops)
			})
		},
	}

	// common options
	flags.common.StringVar(&ops.common.talosconfig, "talosconfig", "",
		fmt.Sprintf("The path to the Talos configuration file. Defaults to '%s' env variable if set, otherwise '%s' and '%s' in order.",
			constants.TalosConfigEnvVar,
			filepath.Join("$HOME", constants.TalosDir, constants.TalosconfigFilename),
			filepath.Join(constants.ServiceAccountMountPath, constants.TalosconfigFilename),
		),
	)
	flags.common.BoolVar(&ops.common.applyConfigEnabled, applyConfigEnabledFlag, false, "enable apply config when the VM is starting in maintenance mode")
	flags.common.StringSliceVar(&ops.common.registryMirrors, registryMirrorFlag, []string{}, "list of registry mirrors to use in format: <registry host>=<mirror URL>")
	flags.common.StringSliceVar(&ops.common.registryInsecure, registryInsecureFlag, []string{}, "list of registry hostnames to skip TLS verification for")
	flags.common.BoolVar(&ops.common.configDebug, configDebugFlag, false, "enable debug in Talos config to send service logs to the console")
	flags.common.IntVar(&ops.common.networkMTU, networkMTUFlag, 1500, "MTU of the cluster network")
	flags.common.StringVar(&ops.common.networkCIDR, networkCIDRFlag, "10.5.0.0/24", "CIDR of the cluster network (IPv4, ULA network for IPv6 is derived in automated way)")
	flags.common.BoolVar(&ops.common.networkIPv4, networkIPv4Flag, true, "enable IPv4 network in the cluster")
	flags.common.StringVar(&ops.common.wireguardCIDR, wireguardCIDRFlag, "", "CIDR of the wireguard network")
	flags.common.IntVar(&ops.common.workers, workersFlag, 1, "the number of workers to create")
	flags.common.IntVar(&ops.common.controlplanes, controlplanesFlag, 1, "the number of controlplanes to create")
	flags.common.StringVar(&ops.common.controlPlaneCpus, controlPlaneCpusFlag, "2.0", "the share of CPUs as fraction (each control plane/VM)")
	flags.common.StringVar(&ops.common.workersCpus, workersCpusFlag, "2.0", "the share of CPUs as fraction (each worker/VM)")
	flags.common.IntVar(&ops.common.controlPlaneMemory, controlPlaneMemoryFlag, 2048, "the limit on memory usage in MB (each control plane/VM)")
	flags.common.IntVar(&ops.common.workersMemory, workersMemoryFlag, 2048, "the limit on memory usage in MB (each worker/VM)")
	flags.common.BoolVar(&ops.common.clusterWait, clusterWaitFlag, true, "wait for the cluster to be ready before returning")
	flags.common.DurationVar(&ops.common.clusterWaitTimeout, clusterWaitTimeoutFlag, 20*time.Minute, "timeout to wait for the cluster to be ready")
	flags.common.BoolVar(&ops.common.forceInitNodeAsEndpoint, forceInitNodeAsEndpointFlag, false, "use init node as endpoint instead of any load balancer endpoint")
	flags.common.StringVar(&ops.common.forceEndpoint, forceEndpointFlag, "", "use endpoint instead of provider defaults")
	flags.common.StringVar(&ops.common.kubernetesVersion, kubernetesVersionFlag, constants.DefaultKubernetesVersion, "desired kubernetes version to run")
	flags.common.StringVarP(&ops.common.inputDir, inputDirFlag, "i", "", "location of pre-generated config files")
	flags.common.BoolVar(&ops.common.withInitNode, withInitNodeFlag, false, "create the cluster with an init node")
	flags.common.StringVar(&ops.common.customCNIUrl, customCNIUrlFlag, "", "install custom CNI from the URL (Talos cluster)")
	flags.common.StringVar(&ops.common.dnsDomain, dnsDomainFlag, "cluster.local", "the dns domain to use for cluster")
	flags.common.BoolVar(&ops.common.skipKubeconfig, skipKubeconfigFlag, false, "skip merging kubeconfig from the created cluster")
	flags.common.BoolVar(&ops.common.skipInjectingConfig, skipInjectingConfigFlag, false, "skip injecting config from embedded metadata server, write config files to current directory")
	flags.common.StringVar(&ops.common.talosVersion, talosVersionFlag, "", "the desired Talos version to generate config for (if not set, defaults to image version)")
	flags.common.BoolVar(&ops.common.enableClusterDiscovery, withClusterDiscoveryFlag, true, "enable cluster discovery")
	flags.common.BoolVar(&ops.common.enableKubeSpan, enableKubeSpanFlag, false, "enable KubeSpan system")
	flags.common.StringArrayVar(&ops.common.configPatch, configPatchFlag, nil, "patch generated machineconfigs (applied to all node types), use @file to read a patch from file")
	flags.common.StringArrayVar(&ops.common.configPatchControlPlane, configPatchControlPlaneFlag, nil, "patch generated machineconfigs (applied to 'init' and 'controlplane' types)")
	flags.common.StringArrayVar(&ops.common.configPatchWorker, configPatchWorkerFlag, nil, "patch generated machineconfigs (applied to 'worker' type)")
	flags.common.IntVar(&ops.common.controlPlanePort, controlPlanePortFlag, constants.DefaultControlPlanePort, "control plane port (load balancer and local API port)")
	flags.common.IntVar(&ops.common.kubePrismPort, kubePrismFlag, constants.DefaultKubePrismPort, "KubePrism port (set to 0 to disable)")
	flags.common.BoolVar(&ops.common.skipK8sNodeReadinessCheck, skipK8sNodeReadinessCheckFlag, false, "skip k8s node readiness checks")
	flags.common.BoolVar(&ops.common.withJSONLogs, withJSONLogsFlag, false, "enable JSON logs receiver and configure Talos to send logs there")

	// qemu options
	flags.qemu.StringVar(&ops.qemu.nodeInstallImage, nodeInstallImageFlag, helpers.DefaultImage(images.DefaultInstallerImageRepository), "the installer image to use")
	flags.qemu.StringVar(&ops.qemu.nodeVmlinuzPath, nodeVmlinuzPathFlag, helpers.ArtifactPath(constants.KernelAssetWithArch), "the compressed kernel image to use")
	flags.qemu.StringVar(&ops.qemu.nodeISOPath, nodeISOPathFlag, "", "the ISO path to use for the initial boot")
	flags.qemu.StringVar(&ops.qemu.nodeUSBPath, nodeUSBPathFlag, "", "the USB stick image path to use for the initial boot")
	flags.qemu.StringVar(&ops.qemu.nodeUKIPath, nodeUKIPathFlag, "", "the UKI image path to use for the initial boot")
	flags.qemu.StringVar(&ops.qemu.nodeInitramfsPath, nodeInitramfsPathFlag, helpers.ArtifactPath(constants.InitramfsAssetWithArch), "initramfs image to use")
	flags.qemu.StringVar(&ops.qemu.nodeDiskImagePath, nodeDiskImagePathFlag, "", "disk image to use")
	flags.qemu.StringVar(&ops.qemu.nodeIPXEBootScript, nodeIPXEBootScriptFlag, "", "iPXE boot script (URL) to use")
	flags.qemu.BoolVar(&ops.qemu.bootloaderEnabled, bootloaderEnabledFlag, true, "enable bootloader to load kernel and initramfs from disk image after install")
	flags.qemu.BoolVar(&ops.qemu.uefiEnabled, uefiEnabledFlag, true, "enable UEFI on x86_64 architecture")
	flags.qemu.BoolVar(&ops.qemu.tpm1_2Enabled, tpmEnabledFlag, false, "enable TPM 1.2 emulation support using swtpm")
	flags.qemu.BoolVar(&ops.qemu.tpm2Enabled, tpm2EnabledFlag, false, "enable TPM 2.0 emulation support using swtpm")
	flags.qemu.BoolVar(&ops.qemu.debugShellEnabled, withDebugShellFlag, false, "drop talos into a maintenance shell on boot, this is for advanced debugging for developers only")
	flags.qemu.BoolVar(&ops.qemu.withIOMMU, withIOMMUFlag, false, "enable IOMMU support, this also add a new PCI root port and an interface attached to it")
	flags.qemu.MarkHidden("with-debug-shell") //nolint:errcheck
	flags.qemu.StringSliceVar(&ops.qemu.extraUEFISearchPaths, extraUEFISearchPathsFlag, []string{}, "additional search paths for UEFI firmware (only applies when UEFI is enabled)")
	flags.qemu.StringSliceVar(&ops.qemu.networkNoMasqueradeCIDRs, networkNoMasqueradeCIDRsFlag, []string{}, "list of CIDRs to exclude from NAT")
	flags.qemu.StringSliceVar(&ops.qemu.nameservers, nameserversFlag, []string{"8.8.8.8", "1.1.1.1", "2001:4860:4860::8888", "2606:4700:4700::1111"}, "list of nameservers to use")
	flags.qemu.IntVar(&ops.qemu.clusterDiskSize, clusterDiskSizeFlag, 6*1024, "default limit on disk size in MB (each VM)")
	flags.qemu.UintVar(&ops.qemu.diskBlockSize, diskBlockSizeFlag, 512, "disk block size")
	flags.qemu.BoolVar(&ops.qemu.clusterDiskPreallocate, clusterDiskPreallocateFlag, true, "whether disk space should be preallocated")
	flags.qemu.StringSliceVar(&ops.qemu.clusterUserVolumes, clusterUserVolumesFlag, []string{}, "list of user volumes to create for each VM in format: <name1>:<size1>:<name2>:<size2>")
	flags.qemu.IntVar(&ops.qemu.extraDisks, extraDisksFlag, 0, "number of extra disks to create for each worker VM")
	flags.qemu.StringSliceVar(&ops.qemu.extraDisksDrivers, "extra-disks-drivers", nil, "driver for each extra disk (virtio, ide, ahci, scsi, nvme, megaraid)")
	flags.qemu.IntVar(&ops.qemu.extraDiskSize, extraDiskSizeFlag, 5*1024, "default limit on disk size in MB (each VM)")
	flags.qemu.StringVar(&ops.qemu.targetArch, targetArchFlag, runtime.GOARCH, "cluster architecture")
	flags.qemu.StringSliceVar(&ops.qemu.cniBinPath, cniBinPathFlag, []string{filepath.Join(clustercmd.DefaultCNIDir, "bin")}, "search path for CNI binaries")
	flags.qemu.StringVar(&ops.qemu.cniConfDir, cniConfDirFlag, filepath.Join(clustercmd.DefaultCNIDir, "conf.d"), "CNI config directory path")
	flags.qemu.StringVar(&ops.qemu.cniCacheDir, cniCacheDirFlag, filepath.Join(clustercmd.DefaultCNIDir, "cache"), "CNI cache directory path")
	flags.qemu.StringVar(&ops.qemu.cniBundleURL, cniBundleURLFlag, fmt.Sprintf("https://github.com/%s/talos/releases/download/%s/talosctl-cni-bundle-%s.tar.gz",
		images.Username, version.Trim(version.Tag), constants.ArchVariable), "URL to download CNI bundle from")
	flags.qemu.BoolVar(&ops.qemu.encryptStatePartition, encryptStatePartitionFlag, false, "enable state partition encryption")
	flags.qemu.BoolVar(&ops.qemu.encryptEphemeralPartition, encryptEphemeralPartitionFlag, false, "enable ephemeral partition encryption")
	flags.qemu.BoolVar(&ops.qemu.encryptUserVolumes, encryptUserVolumeFlag, false, "enable ephemeral partition encryption")
	flags.qemu.StringArrayVar(&ops.qemu.diskEncryptionKeyTypes, diskEncryptionKeyTypesFlag, []string{"uuid"}, "encryption key types to use for disk encryption (uuid, kms)")
	// This flag is currently only supported on qemu, but the internal logic still assumes the possibility of ipv6 on other providers.
	flags.qemu.BoolVar(&ops.common.networkIPv6, networkIPv6Flag, false, "enable IPv6 network in the cluster")
	flags.qemu.BoolVar(&ops.qemu.useVIP, useVIPFlag, false, "use a virtual IP for the controlplane endpoint instead of the loadbalancer")
	flags.qemu.BoolVar(&ops.qemu.badRTC, badRTCFlag, false, "launch VM with bad RTC state")
	flags.qemu.StringVar(&ops.qemu.extraBootKernelArgs, extraBootKernelArgsFlag, "", "add extra kernel args to the initial boot from vmlinuz and initramfs")
	flags.qemu.BoolVar(&ops.qemu.dhcpSkipHostname, dhcpSkipHostnameFlag, false, "skip announcing hostname via DHCP")
	flags.qemu.BoolVar(&ops.qemu.networkChaos, networkChaosFlag, false, "enable to use network chaos parameters")
	flags.qemu.DurationVar(&ops.qemu.jitter, jitterFlag, 0, "specify jitter on the bridge interface")
	flags.qemu.DurationVar(&ops.qemu.latency, latencyFlag, 0, "specify latency on the bridge interface")
	flags.qemu.Float64Var(&ops.qemu.packetLoss, packetLossFlag, 0.0,
		"specify percent of packet loss on the bridge interface. e.g. 50% = 0.50 (default: 0.0)")
	flags.qemu.Float64Var(&ops.qemu.packetReorder, packetReorderFlag, 0.0,
		"specify percent of reordered packets on the bridge interface. e.g. 50% = 0.50 (default: 0.0)")
	flags.qemu.Float64Var(&ops.qemu.packetCorrupt, packetCorruptFlag, 0.0,
		"specify percent of corrupt packets on the bridge interface. e.g. 50% = 0.50 (default: 0.0)")
	flags.qemu.IntVar(&ops.qemu.bandwidth, bandwidthFlag, 0, "specify bandwidth restriction (in kbps) on the bridge interface")
	flags.qemu.StringVar(&ops.qemu.withFirewall, firewallFlag, "", "inject firewall rules into the cluster, value is default policy - accept/block")
	flags.qemu.BoolVar(&ops.qemu.withUUIDHostnames, withUUIDHostnamesFlag, false, "use machine UUIDs as default hostnames")
	flags.qemu.Var(&ops.qemu.withSiderolinkAgent, withSiderolinkAgentFlag,
		"enables the use of siderolink agent as configuration apply mechanism. `true` or `wireguard` enables the agent, `tunnel` enables the agent with grpc tunneling")
	flags.qemu.StringVar(&ops.qemu.configInjectionMethodFlagVal,
		configInjectionMethodFlag, "", "a method to inject machine config: default is HTTP server, 'metal-iso' to mount an ISO")

	flags.qemu.VisitAll(func(f *pflag.Flag) {
		f.Usage = "(qemu) " + f.Usage
	})

	// docker options
	flags.docker.StringVarP(&ops.docker.ports, portsFlag, "p", "",
		"Comma-separated list of ports/protocols to expose on init node. Ex -p <hostPort>:<containerPort>/<protocol (tcp or udp)>")
	flags.docker.StringVar(&ops.docker.nodeImage, nodeImageFlag, helpers.DefaultImage(images.DefaultTalosImageRepository), "the image to use")
	flags.docker.StringVar(&ops.docker.dockerHostIP, dockerHostIPFlag, "0.0.0.0", "Host IP to forward exposed ports to")
	flags.docker.BoolVar(&ops.docker.dockerDisableIPv6, dockerDisableIPv6Flag, false, "skip enabling IPv6 in containers")
	flags.docker.Var(&ops.docker.mountOpts, mountOptsFlag, "attach a mount to the container")

	flags.docker.VisitAll(func(f *pflag.Flag) {
		f.Usage = "(docker) " + f.Usage
	})

	createCmd.Flags().AddFlagSet(flags.common)
	createCmd.Flags().AddFlagSet(flags.qemu)
	createCmd.Flags().AddFlagSet(flags.docker)

	// disable top-level flag sorting.
	// The flags within flagsets are still sorted.
	// This results in the flags being in the order the flagsets were added, but still sorted within the flagset groups.
	createCmd.Flags().SortFlags = false

	createCmd.MarkFlagsMutuallyExclusive(inputDirFlag, nodeInstallImageFlag)
	createCmd.MarkFlagsMutuallyExclusive(inputDirFlag, configDebugFlag)
	createCmd.MarkFlagsMutuallyExclusive(inputDirFlag, dnsDomainFlag)
	createCmd.MarkFlagsMutuallyExclusive(inputDirFlag, withClusterDiscoveryFlag)
	createCmd.MarkFlagsMutuallyExclusive(inputDirFlag, registryMirrorFlag)
	createCmd.MarkFlagsMutuallyExclusive(inputDirFlag, registryInsecureFlag)
	createCmd.MarkFlagsMutuallyExclusive(inputDirFlag, customCNIUrlFlag)
	createCmd.MarkFlagsMutuallyExclusive(inputDirFlag, talosVersionFlag)
	createCmd.MarkFlagsMutuallyExclusive(inputDirFlag, encryptStatePartitionFlag)
	createCmd.MarkFlagsMutuallyExclusive(inputDirFlag, encryptEphemeralPartitionFlag)
	createCmd.MarkFlagsMutuallyExclusive(inputDirFlag, encryptUserVolumeFlag)
	createCmd.MarkFlagsMutuallyExclusive(inputDirFlag, enableKubeSpanFlag)
	createCmd.MarkFlagsMutuallyExclusive(inputDirFlag, forceEndpointFlag)
	createCmd.MarkFlagsMutuallyExclusive(inputDirFlag, kubePrismFlag)
	createCmd.MarkFlagsMutuallyExclusive(inputDirFlag, diskEncryptionKeyTypesFlag)

	createCmd.MarkFlagsMutuallyExclusive(tpmEnabledFlag, tpm2EnabledFlag)

	hideUnimplementedQemuFlags(createCmd, unImplementedQemuFlagsDarwin)

	clustercmd.Cmd.AddCommand(createCmd)
}

// validateProviderFlags checks if flags not applicable for the given provisioner are passed.
func validateProviderFlags(ops createOps, flags createFlags, unImplementedQemuFlagsDarwin []string) error {
	var invalidFlags *pflag.FlagSet

	errMsg := ""

	switch ops.common.rootOps.ProvisionerName {
	case providers.DockerProviderName:
		invalidFlags = flags.qemu
	case providers.QemuProviderName:
		invalidFlags = flags.docker

		if runtime.GOOS == "darwin" {
			flags.qemu.VisitAll(func(f *pflag.Flag) {
				for _, unimplemented := range unImplementedQemuFlagsDarwin {
					if f.Changed && f.Name == unimplemented {
						errMsg += fmt.Sprintf("%s flag is not supported on macos\n", f.Name)

						return
					}
				}
			})
		}
	}

	invalidFlags.VisitAll(func(invalidFlag *pflag.Flag) {
		if invalidFlag.Changed {
			errMsg += fmt.Sprintf("%s flag has been set but has no effect with the %s provisioner\n", invalidFlag.Name, ops.common.rootOps.ProvisionerName)
		}
	})

	if errMsg != "" {
		fmt.Println()

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
