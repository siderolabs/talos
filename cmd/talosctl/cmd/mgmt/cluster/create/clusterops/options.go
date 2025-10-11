// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package clusterops

import (
	"fmt"
	"path/filepath"
	"runtime"
	"time"

	"github.com/docker/cli/opts"

	clustercmd "github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster"
	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create/flags"
	"github.com/siderolabs/talos/cmd/talosctl/pkg/mgmt/helpers"
	"github.com/siderolabs/talos/pkg/bytesize"
	"github.com/siderolabs/talos/pkg/cli"
	"github.com/siderolabs/talos/pkg/images"
	"github.com/siderolabs/talos/pkg/machinery/config/bundle"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/version"
	"github.com/siderolabs/talos/pkg/provision"
)

// ClusterConfigs is the configuration needed to create a talos cluster via the provisioner interface.
type ClusterConfigs struct {
	ClusterRequest   provision.ClusterRequest
	ProvisionOptions []provision.Option
	ConfigBundle     *bundle.Bundle
}

// NodeResources represents CPU and Memory resources for a node.
type NodeResources struct {
	CPU    string
	Memory bytesize.ByteSize
}

// ParsedNodeResources represents parsed CPU and Memory resources for a node.
type ParsedNodeResources struct {
	NanoCPUs int64
	Memory   bytesize.ByteSize
}

// Common are the options that are not specific to a single provider.
type Common struct {
	// rootOps are the options from the root cluster command
	RootOps                   *clustercmd.CmdOps
	TalosconfigDestination    string
	RegistryMirrors           []string
	RegistryInsecure          []string
	KubernetesVersion         string
	ApplyConfigEnabled        bool
	ConfigDebug               bool
	NetworkCIDR               string
	NetworkMTU                int
	NetworkIPv4               bool
	DNSDomain                 string
	Workers                   int
	Controlplanes             int
	ControlplaneResources     NodeResources
	WorkerResources           NodeResources
	ClusterWait               bool
	ClusterWaitTimeout        time.Duration
	ForceInitNodeAsEndpoint   bool
	ForceEndpoint             string
	ControlPlanePort          int
	WithInitNode              bool
	CustomCNIUrl              string
	SkipKubeconfig            bool
	SkipInjectingConfig       bool
	TalosVersion              string
	EnableKubeSpan            bool
	EnableClusterDiscovery    bool
	ConfigPatch               []string
	ConfigPatchControlPlane   []string
	ConfigPatchWorker         []string
	KubePrismPort             int
	SkipK8sNodeReadinessCheck bool
	WithJSONLogs              bool
	WireguardCIDR             string
	WithUUIDHostnames         bool
	NetworkIPv6               bool
	OmniAPIEndpoint           string
}

// Docker are options specific to docker provisioner.
type Docker struct {
	HostIP      string
	DisableIPv6 bool
	MountOpts   opts.MountOpt
	Ports       string
	TalosImage  string
}

// Qemu are options specific to qemu provisioner.
type Qemu struct {
	NodeInstallImage          string
	NodeVmlinuzPath           string
	NodeInitramfsPath         string
	NodeISOPath               string
	NodeUSBPath               string
	NodeUKIPath               string
	NodeDiskImagePath         string
	NodeIPXEBootScript        string
	BootloaderEnabled         bool
	UefiEnabled               bool
	Tpm1_2Enabled             bool
	Tpm2Enabled               bool
	ExtraUEFISearchPaths      []string
	NetworkNoMasqueradeCIDRs  []string
	Nameservers               []string
	Disks                     flags.Disks
	DiskBlockSize             uint
	PreallocateDisks          bool
	ClusterUserVolumes        []string
	TargetArch                string
	CniBinPath                []string
	CniConfDir                string
	CniCacheDir               string
	CniBundleURL              string
	EncryptStatePartition     bool
	EncryptEphemeralPartition bool
	EncryptUserVolumes        bool
	UseVIP                    bool
	BadRTC                    bool
	ExtraBootKernelArgs       string
	DHCPSkipHostname          bool
	NetworkChaos              bool
	Jjitter                   time.Duration
	Latency                   time.Duration
	PacketLoss                float64
	PacketReorder             float64
	PacketCorrupt             float64
	Bandwidth                 int
	DiskEncryptionKeyTypes    []string
	WithFirewall              string
	WithSiderolinkAgent       flags.Agent
	DebugShellEnabled         bool
	WithIOMMU                 bool
	ConfigInjectionMethod     string
}

// GetCommon returns the default common options.
func GetCommon() Common {
	memory2GB := bytesize.WithDefaultUnit("MiB")
	cli.Should(memory2GB.Set("2.0GiB"))
	defaultResources := NodeResources{
		CPU:    "2.0",
		Memory: *memory2GB,
	}

	return Common{
		Controlplanes:         1,
		ControlplaneResources: defaultResources,
		Workers:               1,
		WorkerResources:       defaultResources,

		NetworkCIDR:            "10.5.0.0/24",
		KubernetesVersion:      constants.DefaultKubernetesVersion,
		NetworkMTU:             1500,
		ClusterWaitTimeout:     20 * time.Minute,
		ClusterWait:            true,
		DNSDomain:              "cluster.local",
		ControlPlanePort:       constants.DefaultControlPlanePort,
		RootOps:                &clustercmd.PersistentFlags, // TODO: move this elsewhere
		NetworkIPv4:            true,
		KubePrismPort:          constants.DefaultKubePrismPort,
		EnableClusterDiscovery: true,
		TalosVersion:           helpers.GetTag(),
	}
}

// GetQemu returns default QEMU options.
func GetQemu() Qemu {
	disks := flags.Disks{}
	cli.Should(disks.Set("virtio:10GiB,virtio:6GiB"))

	return Qemu{
		PreallocateDisks:  false,
		BootloaderEnabled: true,
		UefiEnabled:       true,
		Nameservers:       []string{"8.8.8.8", "1.1.1.1", "2001:4860:4860::8888", "2606:4700:4700::1111"},
		DiskBlockSize:     512,
		TargetArch:        runtime.GOARCH,
		CniBinPath:        []string{filepath.Join(clustercmd.DefaultCNIDir, "bin")},
		CniConfDir:        filepath.Join(clustercmd.DefaultCNIDir, "conf.d"),
		CniCacheDir:       filepath.Join(clustercmd.DefaultCNIDir, "cache"),
		CniBundleURL: fmt.Sprintf("https://github.com/%s/talos/releases/download/%s/talosctl-cni-bundle-%s.tar.gz",
			images.Username, version.Trim(version.Tag), constants.ArchVariable),
		Disks: disks,
	}
}

// GetDocker returns default Docker options.
func GetDocker() Docker {
	return Docker{
		HostIP:     "0.0.0.0",
		TalosImage: helpers.DefaultImage(images.DefaultTalosImageRepository),
	}
}
