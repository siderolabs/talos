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

	clustercmd "github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster"
	"github.com/siderolabs/talos/cmd/talosctl/pkg/mgmt/helpers"
	"github.com/siderolabs/talos/pkg/cli"
	"github.com/siderolabs/talos/pkg/images"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/version"
)

const (
	inputDirFlag                 = "input-dir"
	networkIPv4Flag              = "ipv4"
	networkIPv6Flag              = "ipv6"
	networkMTUFlag               = "mtu"
	networkCIDRFlag              = "cidr"
	networkNoMasqueradeCIDRsFlag = "no-masquerade-cidrs"
	nameserversFlag              = "nameservers"
	clusterDiskPreallocateFlag   = "disk-preallocate"
	clusterUserVolumesFlag       = "user-volumes"
	clusterDiskSizeFlag          = "disk"
	useVIPFlag                   = "use-vip"
	bootloaderEnabledFlag        = "with-bootloader"
	controlPlanePortFlag         = "control-plane-port"
	firewallFlag                 = "with-firewall"
	tpm2EnabledFlag              = "with-tpm2"
	withDebugShellFlag           = "with-debug-shell"
	withIOMMUFlag                = "with-iommu"

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

func init() {
	ops := &createOps{
		common: commonOps{},
		docker: dockerOps{},
		qemu:   qemuOps{},
	}

	ops.common.rootOps = &clustercmd.Flags

	// createCmd represents the cluster up command.
	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Creates a local docker-based or QEMU-based kubernetes cluster",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cli.WithContext(context.Background(), func(ctx context.Context) error {
				return create(ctx, *ops)
			})
		},
	}

	// common options
	createCmd.Flags().StringVar(&ops.common.talosconfig, "talosconfig", "",
		fmt.Sprintf("The path to the Talos configuration file. Defaults to '%s' env variable if set, otherwise '%s' and '%s' in order.",
			constants.TalosConfigEnvVar,
			filepath.Join("$HOME", constants.TalosDir, constants.TalosconfigFilename),
			filepath.Join(constants.ServiceAccountMountPath, constants.TalosconfigFilename),
		),
	)
	createCmd.Flags().BoolVar(&ops.common.applyConfigEnabled, "with-apply-config", false, "enable apply config when the VM is starting in maintenance mode")
	createCmd.Flags().StringSliceVar(&ops.common.registryMirrors, registryMirrorFlag, []string{}, "list of registry mirrors to use in format: <registry host>=<mirror URL>")
	createCmd.Flags().StringSliceVar(&ops.common.registryInsecure, registryInsecureFlag, []string{}, "list of registry hostnames to skip TLS verification for")
	createCmd.Flags().BoolVar(&ops.common.configDebug, configDebugFlag, false, "enable debug in Talos config to send service logs to the console")
	createCmd.Flags().IntVar(&ops.common.networkMTU, networkMTUFlag, 1500, "MTU of the cluster network")
	createCmd.Flags().StringVar(&ops.common.networkCIDR, networkCIDRFlag, "10.5.0.0/24", "CIDR of the cluster network (IPv4, ULA network for IPv6 is derived in automated way)")
	createCmd.Flags().BoolVar(&ops.common.networkIPv4, networkIPv4Flag, true, "enable IPv4 network in the cluster")
	createCmd.Flags().BoolVar(&ops.common.networkIPv6, networkIPv6Flag, false, "enable IPv6 network in the cluster (QEMU provisioner only)")
	createCmd.Flags().StringVar(&ops.common.wireguardCIDR, "wireguard-cidr", "", "CIDR of the wireguard network")
	createCmd.Flags().IntVar(&ops.common.workers, "workers", 1, "the number of workers to create")
	createCmd.Flags().IntVar(&ops.common.controlplanes, "masters", 1, "the number of masters to create")
	createCmd.Flags().MarkDeprecated("masters", "use --controlplanes instead") //nolint:errcheck
	createCmd.Flags().IntVar(&ops.common.controlplanes, "controlplanes", 1, "the number of controlplanes to create")
	createCmd.Flags().StringVar(&ops.common.controlPlaneCpus, "cpus", "2.0", "the share of CPUs as fraction (each control plane/VM)")
	createCmd.Flags().StringVar(&ops.common.workersCpus, "cpus-workers", "2.0", "the share of CPUs as fraction (each worker/VM)")
	createCmd.Flags().IntVar(&ops.common.controlPlaneMemory, "memory", 2048, "the limit on memory usage in MB (each control plane/VM)")
	createCmd.Flags().IntVar(&ops.common.workersMemory, "memory-workers", 2048, "the limit on memory usage in MB (each worker/VM)")
	createCmd.Flags().BoolVar(&ops.common.clusterWait, "wait", true, "wait for the cluster to be ready before returning")
	createCmd.Flags().DurationVar(&ops.common.clusterWaitTimeout, "wait-timeout", 20*time.Minute, "timeout to wait for the cluster to be ready")
	createCmd.Flags().BoolVar(&ops.common.forceInitNodeAsEndpoint, "init-node-as-endpoint", false, "use init node as endpoint instead of any load balancer endpoint")
	createCmd.Flags().StringVar(&ops.common.forceEndpoint, forceEndpointFlag, "", "use endpoint instead of provider defaults")
	createCmd.Flags().StringVar(&ops.common.kubernetesVersion, "kubernetes-version", constants.DefaultKubernetesVersion, "desired kubernetes version to run")
	createCmd.Flags().StringVarP(&ops.common.inputDir, inputDirFlag, "i", "", "location of pre-generated config files")
	createCmd.Flags().BoolVar(&ops.common.withInitNode, "with-init-node", false, "create the cluster with an init node")
	createCmd.Flags().StringVar(&ops.common.customCNIUrl, customCNIUrlFlag, "", "install custom CNI from the URL (Talos cluster)")
	createCmd.Flags().StringVar(&ops.common.dnsDomain, dnsDomainFlag, "cluster.local", "the dns domain to use for cluster")
	createCmd.Flags().BoolVar(&ops.common.skipKubeconfig, "skip-kubeconfig", false, "skip merging kubeconfig from the created cluster")
	createCmd.Flags().BoolVar(&ops.common.skipInjectingConfig, "skip-injecting-config", false, "skip injecting config from embedded metadata server, write config files to current directory")
	createCmd.Flags().StringVar(&ops.common.talosVersion, talosVersionFlag, "", "the desired Talos version to generate config for (if not set, defaults to image version)")
	createCmd.Flags().BoolVar(&ops.common.enableClusterDiscovery, withClusterDiscoveryFlag, true, "enable cluster discovery")
	createCmd.Flags().BoolVar(&ops.common.enableKubeSpan, enableKubeSpanFlag, false, "enable KubeSpan system")
	createCmd.Flags().StringArrayVar(&ops.common.configPatch, "config-patch", nil, "patch generated machineconfigs (applied to all node types), use @file to read a patch from file")
	createCmd.Flags().StringArrayVar(&ops.common.configPatchControlPlane, "config-patch-control-plane", nil, "patch generated machineconfigs (applied to 'init' and 'controlplane' types)")
	createCmd.Flags().StringArrayVar(&ops.common.configPatchWorker, "config-patch-worker", nil, "patch generated machineconfigs (applied to 'worker' type)")
	createCmd.Flags().IntVar(&ops.common.controlPlanePort, controlPlanePortFlag, constants.DefaultControlPlanePort, "control plane port (load balancer and local API port, QEMU only)")
	createCmd.Flags().IntVar(&ops.common.kubePrismPort, kubePrismFlag, constants.DefaultKubePrismPort, "KubePrism port (set to 0 to disable)")
	createCmd.Flags().BoolVar(&ops.common.skipK8sNodeReadinessCheck, "skip-k8s-node-readiness-check", false, "skip k8s node readiness checks")
	createCmd.Flags().BoolVar(&ops.common.withJSONLogs, "with-json-logs", false, "enable JSON logs receiver and configure Talos to send logs there")

	// qemu options
	createCmd.Flags().StringVar(&ops.qemu.nodeInstallImage, nodeInstallImageFlag, helpers.DefaultImage(images.DefaultInstallerImageRepository), "the installer image to use")
	createCmd.Flags().StringVar(&ops.qemu.nodeVmlinuzPath, "vmlinuz-path", helpers.ArtifactPath(constants.KernelAssetWithArch), "the compressed kernel image to use")
	createCmd.Flags().StringVar(&ops.qemu.nodeISOPath, "iso-path", "", "the ISO path to use for the initial boot (VM only)")
	createCmd.Flags().StringVar(&ops.qemu.nodeUSBPath, "usb-path", "", "the USB stick image path to use for the initial boot (VM only)")
	createCmd.Flags().StringVar(&ops.qemu.nodeUKIPath, "uki-path", "", "the UKI image path to use for the initial boot (VM only)")
	createCmd.Flags().StringVar(&ops.qemu.nodeInitramfsPath, "initrd-path", helpers.ArtifactPath(constants.InitramfsAssetWithArch), "initramfs image to use")
	createCmd.Flags().StringVar(&ops.qemu.nodeDiskImagePath, "disk-image-path", "", "disk image to use")
	createCmd.Flags().StringVar(&ops.qemu.nodeIPXEBootScript, "ipxe-boot-script", "", "iPXE boot script (URL) to use")
	createCmd.Flags().BoolVar(&ops.qemu.bootloaderEnabled, bootloaderEnabledFlag, true, "enable bootloader to load kernel and initramfs from disk image after install")
	createCmd.Flags().BoolVar(&ops.qemu.uefiEnabled, "with-uefi", true, "enable UEFI on x86_64 architecture")
	createCmd.Flags().BoolVar(&ops.qemu.tpm2Enabled, tpm2EnabledFlag, false, "enable TPM2 emulation support using swtpm")
	createCmd.Flags().BoolVar(&ops.qemu.debugShellEnabled, withDebugShellFlag, false, "drop talos into a maintenance shell on boot, this is for advanced debugging for developers only")
	createCmd.Flags().BoolVar(&ops.qemu.withIOMMU, withIOMMUFlag, false, "enable IOMMU support, this also add a new PCI root port and an interface attached to it (qemu only)")
	createCmd.Flags().MarkHidden("with-debug-shell") //nolint:errcheck
	createCmd.Flags().StringSliceVar(&ops.qemu.extraUEFISearchPaths, "extra-uefi-search-paths", []string{}, "additional search paths for UEFI firmware (only applies when UEFI is enabled)")
	createCmd.Flags().StringSliceVar(&ops.qemu.networkNoMasqueradeCIDRs, networkNoMasqueradeCIDRsFlag, []string{}, "list of CIDRs to exclude from NAT (QEMU provisioner only)")
	createCmd.Flags().StringSliceVar(&ops.qemu.nameservers, nameserversFlag, []string{"8.8.8.8", "1.1.1.1", "2001:4860:4860::8888", "2606:4700:4700::1111"}, "list of nameservers to use")
	createCmd.Flags().IntVar(&ops.qemu.clusterDiskSize, clusterDiskSizeFlag, 6*1024, "default limit on disk size in MB (each VM)")
	createCmd.Flags().UintVar(&ops.qemu.diskBlockSize, "disk-block-size", 512, "disk block size (VM only)")
	createCmd.Flags().BoolVar(&ops.qemu.clusterDiskPreallocate, clusterDiskPreallocateFlag, true, "whether disk space should be preallocated")
	createCmd.Flags().StringSliceVar(&ops.qemu.clusterUserVolumes, clusterUserVolumesFlag, []string{}, "list of user volumes to create for each VM in format: <name1>:<size1>:<name2>:<size2>")
	createCmd.Flags().IntVar(&ops.qemu.extraDisks, "extra-disks", 0, "number of extra disks to create for each worker VM")
	createCmd.Flags().StringSliceVar(&ops.qemu.extraDisksDrivers, "extra-disks-drivers", nil, "driver for each extra disk (virtio, ide, ahci, scsi, nvme, megaraid)")
	createCmd.Flags().IntVar(&ops.qemu.extraDiskSize, "extra-disks-size", 5*1024, "default limit on disk size in MB (each VM)")
	createCmd.Flags().StringVar(&ops.qemu.targetArch, "arch", stdruntime.GOARCH, "cluster architecture")
	createCmd.Flags().StringSliceVar(&ops.qemu.cniBinPath, "cni-bin-path", []string{filepath.Join(clustercmd.DefaultCNIDir, "bin")}, "search path for CNI binaries (VM only)")
	createCmd.Flags().StringVar(&ops.qemu.cniConfDir, "cni-conf-dir", filepath.Join(clustercmd.DefaultCNIDir, "conf.d"), "CNI config directory path (VM only)")
	createCmd.Flags().StringVar(&ops.qemu.cniCacheDir, "cni-cache-dir", filepath.Join(clustercmd.DefaultCNIDir, "cache"), "CNI cache directory path (VM only)")
	createCmd.Flags().StringVar(&ops.qemu.cniBundleURL, "cni-bundle-url", fmt.Sprintf("https://github.com/%s/talos/releases/download/%s/talosctl-cni-bundle-%s.tar.gz",
		images.Username, version.Trim(version.Tag), constants.ArchVariable), "URL to download CNI bundle from (VM only)")
	createCmd.Flags().BoolVar(&ops.qemu.encryptStatePartition, encryptStatePartitionFlag, false, "enable state partition encryption")
	createCmd.Flags().BoolVar(&ops.qemu.encryptEphemeralPartition, encryptEphemeralPartitionFlag, false, "enable ephemeral partition encryption")
	createCmd.Flags().BoolVar(&ops.qemu.encryptUserVolumes, encryptUserVolumeFlag, false, "enable ephemeral partition encryption")
	createCmd.Flags().StringArrayVar(&ops.qemu.diskEncryptionKeyTypes, diskEncryptionKeyTypesFlag, []string{"uuid"}, "encryption key types to use for disk encryption (uuid, kms)")
	createCmd.Flags().BoolVar(&ops.qemu.useVIP, useVIPFlag, false, "use a virtual IP for the controlplane endpoint instead of the loadbalancer")
	createCmd.Flags().BoolVar(&ops.qemu.badRTC, "bad-rtc", false, "launch VM with bad RTC state (QEMU only)")
	createCmd.Flags().StringVar(&ops.qemu.extraBootKernelArgs, "extra-boot-kernel-args", "", "add extra kernel args to the initial boot from vmlinuz and initramfs (QEMU only)")
	createCmd.Flags().BoolVar(&ops.qemu.dhcpSkipHostname, "disable-dhcp-hostname", false, "skip announcing hostname via DHCP (QEMU only)")
	createCmd.Flags().BoolVar(&ops.qemu.networkChaos, "with-network-chaos", false, "enable to use network chaos parameters when creating a qemu cluster")
	createCmd.Flags().DurationVar(&ops.qemu.jitter, "with-network-jitter", 0, "specify jitter on the bridge interface when creating a qemu cluster")
	createCmd.Flags().DurationVar(&ops.qemu.latency, "with-network-latency", 0, "specify latency on the bridge interface when creating a qemu cluster")
	createCmd.Flags().Float64Var(&ops.qemu.packetLoss, "with-network-packet-loss", 0.0,
		"specify percent of packet loss on the bridge interface when creating a qemu cluster. e.g. 50% = 0.50 (default: 0.0)")
	createCmd.Flags().Float64Var(&ops.qemu.packetReorder, "with-network-packet-reorder", 0.0,
		"specify percent of reordered packets on the bridge interface when creating a qemu cluster. e.g. 50% = 0.50 (default: 0.0)")
	createCmd.Flags().Float64Var(&ops.qemu.packetCorrupt, "with-network-packet-corrupt", 0.0,
		"specify percent of corrupt packets on the bridge interface when creating a qemu cluster. e.g. 50% = 0.50 (default: 0.0)")
	createCmd.Flags().IntVar(&ops.qemu.bandwidth, "with-network-bandwidth", 0, "specify bandwidth restriction (in kbps) on the bridge interface when creating a qemu cluster")
	createCmd.Flags().StringVar(&ops.qemu.withFirewall, firewallFlag, "", "inject firewall rules into the cluster, value is default policy - accept/block (QEMU only)")
	createCmd.Flags().BoolVar(&ops.qemu.withUUIDHostnames, "with-uuid-hostnames", false, "use machine UUIDs as default hostnames (QEMU only)")
	createCmd.Flags().Var(&ops.qemu.withSiderolinkAgent, "with-siderolink", "enables the use of siderolink agent as configuration apply mechanism. `true` or `wireguard` enables the agent, `tunnel` enables the agent with grpc tunneling") //nolint:lll
	createCmd.Flags().StringVar(&ops.qemu.configInjectionMethodFlagVal,
		"config-injection-method", "", "a method to inject machine config: default is HTTP server, 'metal-iso' to mount an ISO (QEMU only)")

	// docker options
	createCmd.Flags().StringVarP(&ops.docker.ports, "exposed-ports", "p", "",
		"Comma-separated list of ports/protocols to expose on init node. Ex -p <hostPort>:<containerPort>/<protocol (tcp or udp)> (Docker provisioner only)")
	createCmd.Flags().StringVar(&ops.docker.nodeImage, "image", helpers.DefaultImage(images.DefaultTalosImageRepository), "the image to use")
	createCmd.Flags().StringVar(&ops.docker.dockerHostIP, "docker-host-ip", "0.0.0.0", "Host IP to forward exposed ports to (Docker provisioner only)")
	createCmd.Flags().BoolVar(&ops.docker.dockerDisableIPv6, "docker-disable-ipv6", false, "skip enabling IPv6 in containers (Docker only)")
	createCmd.Flags().Var(&ops.docker.mountOpts, "mount", "attach a mount to the container (Docker only)")

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

	clustercmd.Cmd.AddCommand(createCmd)
}
