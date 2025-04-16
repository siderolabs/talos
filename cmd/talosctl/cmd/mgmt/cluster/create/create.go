// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package create

import (
	"context"
	"fmt"
	"path/filepath"
	stdruntime "runtime"
	"time"

	"github.com/docker/cli/opts"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster"
	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create/clustermaker"
	"github.com/siderolabs/talos/cmd/talosctl/pkg/mgmt/helpers"
	"github.com/siderolabs/talos/pkg/cli"
	"github.com/siderolabs/talos/pkg/images"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/version"
	"github.com/siderolabs/talos/pkg/provision/providers"
)

// CommonOps are the options common across all the providers.
type commonOps = clustermaker.Options

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
	common *commonOps
	docker *dockerOps
	qemu   *qemuOps
}

type createFlags struct {
	common *pflag.FlagSet
	docker *pflag.FlagSet
	qemu   *pflag.FlagSet
}

func getCreateCommand() *cobra.Command {
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
		useVIPFlag                    = "use-vip"
		bootloaderEnabledFlag         = "with-bootloader"
		controlPlanePortFlag          = "control-plane-port"
		firewallFlag                  = "with-firewall"
		tpm2EnabledFlag               = "with-tpm2"
		withDebugShellFlag            = "with-debug-shell"
		withIOMMUFlag                 = "with-iommu"
		talosconfigFlag               = "talosconfig"
		applyConfigEnabledFlag        = "with-apply-config"
		wireguardCIDRFlag             = "wireguard-cidr"
		workersFlag                   = "workers"
		mastersFlag                   = "masters"
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
		enableKubeSpanFlag            = "with-kubespan"
		forceEndpointFlag             = "endpoint"
		kubePrismFlag                 = "kubeprism-port"
		diskEncryptionKeyTypesFlag    = "disk-encryption-key-types"
	)

	ops := createOps{
		common: &commonOps{},
		docker: &dockerOps{},
		qemu:   &qemuOps{},
	}
	ops.common.RootOps = &cluster.Flags

	getDockerFlags := func() *pflag.FlagSet {
		dockerFlags := pflag.NewFlagSet("", pflag.PanicOnError)
		dockerFlags.StringVar(&ops.docker.dockerHostIP, dockerHostIPFlag, "0.0.0.0", "Host IP to forward exposed ports to")
		dockerFlags.StringVar(&ops.docker.nodeImage, nodeImageFlag, helpers.DefaultImage(images.DefaultTalosImageRepository), "the image to use")
		dockerFlags.StringVarP(&ops.docker.ports, portsFlag, "p", "",
			"Comma-separated list of ports/protocols to expose on init node. Ex -p <hostPort>:<containerPort>/<protocol (tcp or udp)>")
		dockerFlags.BoolVar(&ops.docker.dockerDisableIPv6, dockerDisableIPv6Flag, false, "skip enabling IPv6 in containers")
		dockerFlags.Var(&ops.docker.mountOpts, mountOptsFlag, "attach a mount to the container")

		dockerFlags.VisitAll(func(f *pflag.Flag) {
			f.Usage = "(docker only) " + f.Usage
		})

		return dockerFlags
	}

	getQemuFlags := func() *pflag.FlagSet {
		qemuFlags := pflag.NewFlagSet("", pflag.PanicOnError)
		qemuFlags.StringVar(&ops.qemu.nodeInstallImage, nodeInstallImageFlag, helpers.DefaultImage(images.DefaultInstallerImageRepository), "the installer image to use")
		qemuFlags.StringVar(&ops.qemu.nodeVmlinuzPath, nodeVmlinuzPathFlag, helpers.ArtifactPath(constants.KernelAssetWithArch), "the compressed kernel image to use")
		qemuFlags.StringVar(&ops.qemu.nodeISOPath, nodeISOPathFlag, "", "the ISO path to use for the initial boot")
		qemuFlags.StringVar(&ops.qemu.nodeUSBPath, nodeUSBPathFlag, "", "the USB stick image path to use for the initial boot")
		qemuFlags.StringVar(&ops.qemu.nodeUKIPath, nodeUKIPathFlag, "", "the UKI image path to use for the initial boot")
		qemuFlags.StringVar(&ops.qemu.nodeInitramfsPath, nodeInitramfsPathFlag, helpers.ArtifactPath(constants.InitramfsAssetWithArch), "initramfs image to use")
		qemuFlags.StringVar(&ops.qemu.nodeDiskImagePath, nodeDiskImagePathFlag, "", "disk image to use")
		qemuFlags.StringVar(&ops.qemu.nodeIPXEBootScript, nodeIPXEBootScriptFlag, "", "iPXE boot script (URL) to use")
		qemuFlags.BoolVar(&ops.qemu.bootloaderEnabled, bootloaderEnabledFlag, true, "enable bootloader to load kernel and initramfs from disk image after install")
		qemuFlags.BoolVar(&ops.qemu.uefiEnabled, uefiEnabledFlag, true, "enable UEFI on x86_64 architecture")
		qemuFlags.BoolVar(&ops.qemu.tpm2Enabled, tpm2EnabledFlag, false, "enable TPM2 emulation support using swtpm")
		qemuFlags.BoolVar(&ops.qemu.debugShellEnabled, withDebugShellFlag, false, "drop talos into a maintenance shell on boot, this is for advanced debugging for developers only")
		qemuFlags.BoolVar(&ops.qemu.withIOMMU, withIOMMUFlag, false, "enable IOMMU support, this also add a new PCI root port and an interface attached to it)")
		qemuFlags.MarkHidden("with-debug-shell") //nolint:errcheck
		qemuFlags.StringSliceVar(&ops.qemu.extraUEFISearchPaths, extraUEFISearchPathsFlag, []string{}, "additional search paths for UEFI firmware (only applies when UEFI is enabled)")
		qemuFlags.StringSliceVar(&ops.qemu.networkNoMasqueradeCIDRs, networkNoMasqueradeCIDRsFlag, []string{}, "list of CIDRs to exclude from NAT")
		// This can be set to true only with qemu, but is still passed along with common ops due to the convenience of not having to separate the otherwise same logic
		qemuFlags.BoolVar(&ops.common.NetworkIPv6, networkIPv6Flag, false, "enable IPv6 network in the cluster")
		qemuFlags.StringSliceVar(&ops.qemu.nameservers, nameserversFlag, []string{"8.8.8.8", "1.1.1.1", "2001:4860:4860::8888", "2606:4700:4700::1111"}, "list of nameservers to use")
		qemuFlags.IntVar(&ops.qemu.clusterDiskSize, clusterDiskSizeFlag, 6*1024, "default limit on disk size in MB (each VM)")
		qemuFlags.UintVar(&ops.qemu.diskBlockSize, "disk-block-size", 512, "disk block size")
		qemuFlags.BoolVar(&ops.qemu.clusterDiskPreallocate, clusterDiskPreallocateFlag, true, "whether disk space should be preallocated")
		qemuFlags.StringSliceVar(&ops.qemu.clusterUserVolumes, clusterUserVolumesFlag, []string{}, "list of disks to create for each VM in format: <mount_point1>:<size1>:<mount_point2>:<size2>")
		qemuFlags.IntVar(&ops.qemu.extraDisks, extraDisksFlag, 0, "number of extra disks to create for each worker VM")
		qemuFlags.StringSliceVar(&ops.qemu.extraDisksDrivers, extraDisksDriversFlag, nil, "driver for each extra disk (virtio, ide, ahci, scsi, nvme, megaraid)")
		qemuFlags.IntVar(&ops.qemu.extraDiskSize, extraDiskSizeFlag, 5*1024, "default limit on disk size in MB (each VM)")
		qemuFlags.StringVar(&ops.qemu.targetArch, targetArchFlag, stdruntime.GOARCH, "cluster architecture")
		qemuFlags.StringSliceVar(&ops.qemu.cniBinPath, cniBinPathFlag, []string{filepath.Join(cluster.DefaultCNIDir, "bin")}, "search path for CNI binaries")
		qemuFlags.StringVar(&ops.qemu.cniConfDir, cniConfDirFlag, filepath.Join(cluster.DefaultCNIDir, "conf.d"), "CNI config directory path")
		qemuFlags.StringVar(&ops.qemu.cniCacheDir, cniCacheDirFlag, filepath.Join(cluster.DefaultCNIDir, "cache"), "CNI cache directory path")
		qemuFlags.StringVar(&ops.qemu.cniBundleURL, cniBundleURLFlag, fmt.Sprintf("https://github.com/%s/talos/releases/download/%s/talosctl-cni-bundle-%s.tar.gz",
			images.Username, version.Trim(version.Tag), constants.ArchVariable), "URL to download CNI bundle from")
		qemuFlags.BoolVar(&ops.qemu.encryptStatePartition, encryptStatePartitionFlag, false, "enable state partition encryption")
		qemuFlags.BoolVar(&ops.qemu.encryptEphemeralPartition, encryptEphemeralPartitionFlag, false, "enable ephemeral partition encryption")
		qemuFlags.StringArrayVar(&ops.qemu.diskEncryptionKeyTypes, diskEncryptionKeyTypesFlag, []string{"uuid"}, "encryption key types to use for disk encryption (uuid, kms)")
		qemuFlags.BoolVar(&ops.qemu.useVIP, useVIPFlag, false, "use a virtual IP for the controlplane endpoint instead of the loadbalancer")
		qemuFlags.BoolVar(&ops.qemu.badRTC, badRTCFlag, false, "launch VM with bad RTC state")
		qemuFlags.StringVar(&ops.qemu.extraBootKernelArgs, extraBootKernelArgsFlag, "", "add extra kernel args to the initial boot from vmlinuz and initramfs")
		qemuFlags.BoolVar(&ops.qemu.dhcpSkipHostname, dhcpSkipHostnameFlag, false, "skip announcing hostname via DHCP")
		qemuFlags.BoolVar(&ops.qemu.networkChaos, networkChaosFlag, false, "enable network chaos parameters when creating a qemu cluster")
		qemuFlags.DurationVar(&ops.qemu.jitter, jitterFlag, 0, "specify jitter on the bridge interface")
		qemuFlags.DurationVar(&ops.qemu.latency, latencyFlag, 0, "specify latency on the bridge interface")
		qemuFlags.Float64Var(&ops.qemu.packetLoss, packetLossFlag, 0.0, "specify percent of packet loss on the bridge interface. e.g. 50% = 0.50 (default: 0.0)")
		qemuFlags.Float64Var(&ops.qemu.packetReorder, packetReorderFlag, 0.0, "specify percent of reordered packets on the bridge interface. e.g. 50% = 0.50 (default: 0.0)")
		qemuFlags.Float64Var(&ops.qemu.packetCorrupt, packetCorruptFlag, 0.0, "specify percent of corrupt packets on the bridge interface. e.g. 50% = 0.50 (default: 0.0)")
		qemuFlags.IntVar(&ops.qemu.bandwidth, bandwidthFlag, 0, "specify bandwidth restriction (in kbps) on the bridge interface")
		qemuFlags.StringVar(&ops.qemu.withFirewall, firewallFlag, "", "inject firewall rules into the cluster, value is default policy - accept/block")
		qemuFlags.BoolVar(&ops.qemu.withUUIDHostnames, withUUIDHostnamesFlag, false, "use machine UUIDs as default hostnames")
		qemuFlags.Var(&ops.qemu.withSiderolinkAgent, withSiderolinkAgentFlag, "enables the use of siderolink agent as configuration apply mechanism. `true` or `wireguard` enables the agent, `tunnel` enables the agent with grpc tunneling") //nolint:lll
		qemuFlags.StringVar(&ops.qemu.configInjectionMethodFlagVal, configInjectionMethodFlag, "", "a method to inject machine config: default is HTTP server, 'metal-iso' to mount an ISO")

		qemuFlags.VisitAll(func(f *pflag.Flag) {
			f.Usage = "(QEMU only) " + f.Usage
		})

		return qemuFlags
	}

	getCommonFlags := func() *pflag.FlagSet {
		commonFlags := pflag.NewFlagSet("", pflag.PanicOnError)
		commonFlags.StringVar(&ops.common.Talosconfig, talosconfigFlag, "",
			fmt.Sprintf("The path to the Talos configuration file. Defaults to '%s' env variable if set, otherwise '%s' and '%s' in order.",
				constants.TalosConfigEnvVar,
				filepath.Join("$HOME", constants.TalosDir, constants.TalosconfigFilename),
				filepath.Join(constants.ServiceAccountMountPath, constants.TalosconfigFilename),
			),
		)
		commonFlags.BoolVar(&ops.common.ApplyConfigEnabled, applyConfigEnabledFlag, false, "enable apply config when the VM is starting in maintenance mode")
		commonFlags.StringSliceVar(&ops.common.RegistryMirrors, registryMirrorFlag, []string{}, "list of registry mirrors to use in format: <registry host>=<mirror URL>")
		commonFlags.StringSliceVar(&ops.common.RegistryInsecure, registryInsecureFlag, []string{}, "list of registry hostnames to skip TLS verification for")
		commonFlags.BoolVar(&ops.common.ConfigDebug, configDebugFlag, false, "enable debug in Talos config to send service logs to the console")
		commonFlags.IntVar(&ops.common.NetworkMTU, networkMTUFlag, 1500, "MTU of the cluster network")
		commonFlags.StringVar(&ops.common.NetworkCIDR, networkCIDRFlag, "10.5.0.0/24", "CIDR of the cluster network (IPv4, ULA network for IPv6 is derived in automated way)")
		commonFlags.IntVar(&ops.common.ControlPlanePort, controlPlanePortFlag, constants.DefaultControlPlanePort, "control plane port (load balancer and local API port)")
		commonFlags.BoolVar(&ops.common.NetworkIPv4, networkIPv4Flag, true, "enable IPv4 network in the cluster")
		commonFlags.StringVar(&ops.common.WireguardCIDR, wireguardCIDRFlag, "", "CIDR of the wireguard network")
		commonFlags.IntVar(&ops.common.Workers, workersFlag, 1, "the number of workers to create")
		commonFlags.IntVar(&ops.common.Controlplanes, mastersFlag, 1, "the number of masters to create")
		commonFlags.MarkDeprecated("commonOps.masters", "use --controlplanes instead") //nolint:errcheck
		commonFlags.IntVar(&ops.common.Controlplanes, controlplanesFlag, 1, "the number of controlplanes to create")
		commonFlags.StringVar(&ops.common.ControlPlaneCpus, controlPlaneCpusFlag, "2.0", "the share of CPUs as fraction (each control plane/VM)")
		commonFlags.StringVar(&ops.common.WorkersCpus, workersCpusFlag, "2.0", "the share of CPUs as fraction (each worker/VM)")
		commonFlags.IntVar(&ops.common.ControlPlaneMemory, controlPlaneMemoryFlag, 2048, "the limit on memory usage in MB (each control plane/VM)")
		commonFlags.IntVar(&ops.common.WorkersMemory, workersMemoryFlag, 2048, "the limit on memory usage in MB (each worker/VM)")
		commonFlags.BoolVar(&ops.common.ClusterWait, clusterWaitFlag, true, "wait for the cluster to be ready before returning")
		commonFlags.DurationVar(&ops.common.ClusterWaitTimeout, clusterWaitTimeoutFlag, 20*time.Minute, "timeout to wait for the cluster to be ready")
		commonFlags.BoolVar(&ops.common.ForceInitNodeAsEndpoint, forceInitNodeAsEndpointFlag, false, "use init node as endpoint instead of any load balancer endpoint")
		commonFlags.StringVar(&ops.common.ForceEndpoint, forceEndpointFlag, "", "use endpoint instead of provider defaults")
		commonFlags.StringVar(&ops.common.KubernetesVersion, kubernetesVersionFlag, constants.DefaultKubernetesVersion, "desired kubernetes version to run")
		commonFlags.StringVarP(&ops.common.InputDir, inputDirFlag, "i", "", "location of pre-generated config files")
		commonFlags.BoolVar(&ops.common.WithInitNode, withInitNodeFlag, false, "create the cluster with an init node")
		commonFlags.StringVar(&ops.common.CustomCNIUrl, customCNIUrlFlag, "", "install custom CNI from the URL (Talos cluster)")
		commonFlags.StringVar(&ops.common.DNSDomain, dnsDomainFlag, "cluster.local", "the dns domain to use for cluster")
		commonFlags.BoolVar(&ops.common.SkipKubeconfig, skipKubeconfigFlag, false, "skip merging kubeconfig from the created cluster")
		commonFlags.BoolVar(&ops.common.SkipInjectingConfig, skipInjectingConfigFlag, false, "skip injecting config from embedded metadata server, write config files to current directory")
		commonFlags.StringVar(&ops.common.TalosVersion, talosVersionFlag, "", "the desired Talos version to generate config for (if not set, defaults to image version)")
		commonFlags.BoolVar(&ops.common.EnableClusterDiscovery, withClusterDiscoveryFlag, true, "enable cluster discovery")
		commonFlags.BoolVar(&ops.common.EnableKubeSpan, enableKubeSpanFlag, false, "enable KubeSpan system")
		commonFlags.StringArrayVar(&ops.common.ConfigPatch, configPatchFlag, nil, "patch generated machineconfigs (applied to all node types), use @file to read a patch from file")
		commonFlags.StringArrayVar(&ops.common.ConfigPatchControlPlane, configPatchControlPlaneFlag, nil, "patch generated machineconfigs (applied to 'init' and 'controlplane' types)")
		commonFlags.StringArrayVar(&ops.common.ConfigPatchWorker, configPatchWorkerFlag, nil, "patch generated machineconfigs (applied to 'worker' type)")
		commonFlags.IntVar(&ops.common.KubePrismPort, kubePrismFlag, constants.DefaultKubePrismPort, "KubePrism port (set to 0 to disable)")
		commonFlags.BoolVar(&ops.common.SkipK8sNodeReadinessCheck, skipK8sNodeReadinessCheckFlag, false, "skip k8s node readiness checks")
		commonFlags.BoolVar(&ops.common.WithJSONLogs, withJSONLogsFlag, false, "enable JSON logs receiver and configure Talos to send logs there")

		return commonFlags
	}

	flags := createFlags{
		common: getCommonFlags(),
		qemu:   getQemuFlags(),
		docker: getDockerFlags(),
	}

	createQemuCmd := &cobra.Command{
		Use:   providers.QemuProviderName,
		Short: "Creates a local qemu based kubernetes cluster (linux only)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			provisionerFlag := cmd.Flag(cluster.ProvisionerFlag)
			if err := validateCmdProvisioner(provisionerFlag, providers.QemuProviderName); err != nil {
				return err
			}
			ops.common.RootOps.ProvisionerName = providers.QemuProviderName
			if err := validateProviderFlags(ops, flags); err != nil {
				return err
			}

			return cli.WithContext(context.Background(), func(ctx context.Context) error {
				return createQemuCluster(ctx, *ops.common, *ops.qemu)
			})
		},
	}

	createDockerCmd := &cobra.Command{
		Use:   providers.DockerProviderName,
		Short: "Creates a local docker based kubernetes cluster",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			provisionerFlag := cmd.Flag(cluster.ProvisionerFlag)
			if err := validateCmdProvisioner(provisionerFlag, providers.DockerProviderName); err != nil {
				return err
			}
			if err := validateProviderFlags(ops, flags); err != nil {
				return err
			}

			return cli.WithContext(context.Background(), func(ctx context.Context) error {
				return createDockerCluster(ctx, *ops.common, *ops.docker)
			})
		},
	}

	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Creates a local docker-based or QEMU-based kubernetes cluster",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := providers.IsValidProvider(ops.common.RootOps.ProvisionerName); err != nil {
				return err
			}
			if err := validateProviderFlags(ops, flags); err != nil {
				return err
			}

			return cli.WithContext(context.Background(), func(ctx context.Context) error {
				if ops.common.RootOps.ProvisionerName == providers.DockerProviderName {
					return createDockerCluster(ctx, *ops.common, *ops.docker)
				}

				return createQemuCluster(ctx, *ops.common, *ops.qemu)
			})
		},
	}

	createCmd.Flags().AddFlagSet(flags.common)
	createCmd.Flags().AddFlagSet(flags.docker)
	createCmd.Flags().AddFlagSet(flags.qemu)
	createDockerCmd.Flags().AddFlagSet(flags.common)
	createDockerCmd.Flags().AddFlagSet(flags.docker)
	createQemuCmd.Flags().AddFlagSet(flags.common)
	createQemuCmd.Flags().AddFlagSet(flags.qemu)

	// The individual flagsets are still sorted
	createCmd.Flags().SortFlags = false
	createDockerCmd.Flags().SortFlags = false
	createQemuCmd.Flags().SortFlags = false

	markInputDirFlagsExclusive := func(cmd *cobra.Command) {
		exclusiveFlags := []string{
			nodeInstallImageFlag,
			configDebugFlag,
			dnsDomainFlag,
			withClusterDiscoveryFlag,
			registryMirrorFlag,
			registryInsecureFlag,
			customCNIUrlFlag,
			talosVersionFlag,
			encryptStatePartitionFlag,
			encryptEphemeralPartitionFlag,
			enableKubeSpanFlag,
			forceEndpointFlag,
			kubePrismFlag,
			diskEncryptionKeyTypesFlag,
		}

		for _, f := range exclusiveFlags {
			if cmd.Flag(f) != nil {
				cmd.MarkFlagsMutuallyExclusive(inputDirFlag, f)
			}
		}
	}
	markInputDirFlagsExclusive(createCmd)
	markInputDirFlagsExclusive(createQemuCmd)
	markInputDirFlagsExclusive(createDockerCmd)

	createCmd.AddCommand(createDockerCmd)
	createCmd.AddCommand(createQemuCmd)

	return createCmd
}

// validateProviderFlags checks if flags not applicable for the given provisioner are passed.
func validateProviderFlags(ops createOps, flags createFlags) error {
	var invalidFlags *pflag.FlagSet

	switch ops.common.RootOps.ProvisionerName {
	case providers.DockerProviderName:
		invalidFlags = flags.qemu
	case providers.QemuProviderName:
		invalidFlags = flags.docker
	}

	errMsg := ""

	invalidFlags.VisitAll(func(invalidFlag *pflag.Flag) {
		if invalidFlag.Changed {
			errMsg += fmt.Sprintf("%s flag has been set but has no effect with the %s provisioner\n", invalidFlag.Name, ops.common.RootOps.ProvisionerName)
		}
	})

	if errMsg != "" {
		fmt.Println()

		return fmt.Errorf(errMsg, "invalid provisioner flags found")
	}

	return nil
}

func init() {
	createCmd := getCreateCommand()

	cluster.Cmd.AddCommand(createCmd)
}

// validateCmdProvisioner checks if the passed provisionerFlag matches the command provisioner.
func validateCmdProvisioner(provisionerFlag *pflag.Flag, provisioner string) error {
	if !provisionerFlag.Changed ||
		provisionerFlag.Value.String() == "" ||
		provisionerFlag.Value.String() == provisioner {
		return nil
	}

	return fmt.Errorf(`invalid provisioner: "%s"
--provisioner must be omitted or has to be "%s" when using cluster create %s`, provisionerFlag.Value.String(), provisioner, provisioner)
}

type agentFlag uint8

func (a *agentFlag) String() string {
	switch *a {
	case 1:
		return "wireguard"
	case 2:
		return "grpc-tunnel"
	case 3:
		return "wireguard+tls"
	case 4:
		return "grpc-tunnel+tls"
	default:
		return "none"
	}
}

func (a *agentFlag) Set(s string) error {
	switch s {
	case "true", "wireguard":
		*a = 1
	case "tunnel":
		*a = 2
	case "wireguard+tls":
		*a = 3
	case "grpc-tunnel+tls":
		*a = 4
	default:
		return fmt.Errorf("unknown type: %s, possible values: 'true', 'wireguard' for the usual WG; 'tunnel' for WG over GRPC, add '+tls' to enable TLS for API", s)
	}

	return nil
}

func (a *agentFlag) Type() string    { return "agent" }
func (a *agentFlag) IsEnabled() bool { return *a != 0 }
func (a *agentFlag) IsTunnel() bool  { return *a == 2 || *a == 4 }
func (a *agentFlag) IsTLS() bool     { return *a == 3 || *a == 4 }
