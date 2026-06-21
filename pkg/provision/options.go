// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package provision

import (
	"io"
	"os"
	"runtime"

	"github.com/siderolabs/talos/pkg/machinery/client"
	clientconfig "github.com/siderolabs/talos/pkg/machinery/client/config"
)

// Option controls Provisioner.
type Option func(o *Options) error

// WithLogWriter sets logging destination.
func WithLogWriter(w io.Writer) Option {
	return func(o *Options) error {
		o.LogWriter = w

		return nil
	}
}

// WithKubernetesEndpoint specifies full external Kubernetes API endpoint to use when accessing Talos cluster.
func WithKubernetesEndpoint(endpoint string) Option {
	return func(o *Options) error {
		o.KubernetesEndpoint = endpoint

		return nil
	}
}

// WithTalosConfig specifies talosconfig to use when acessing Talos cluster.
func WithTalosConfig(talosConfig *clientconfig.Config) Option {
	return func(o *Options) error {
		o.TalosConfig = talosConfig

		return nil
	}
}

// WithTalosClient specifies client to use when acessing Talos cluster.
func WithTalosClient(client *client.Client) Option {
	return func(o *Options) error {
		o.TalosClient = client

		return nil
	}
}

// WithBootloader enables or disables bootloader (bootloader is enabled by default).
func WithBootloader(enabled bool) Option {
	return func(o *Options) error {
		o.BootloaderEnabled = enabled

		return nil
	}
}

// WithUEFI enables or disables UEFI boot on amd64 (default for amd64 is BIOS boot).
func WithUEFI(enabled bool) Option {
	return func(o *Options) error {
		o.UEFIEnabled = enabled

		return nil
	}
}

// WithTPM1_2 enables or disables TPM1.2 emulation.
func WithTPM1_2(enabled bool) Option {
	return func(o *Options) error {
		o.TPM1_2Enabled = enabled

		return nil
	}
}

// WithTPM2 enables or disables TPM2.0 emulation.
func WithTPM2(enabled bool) Option {
	return func(o *Options) error {
		o.TPM2Enabled = enabled

		return nil
	}
}

// WithIOMMU enables or disables IOMMU.
func WithIOMMU(enabled bool) Option {
	return func(o *Options) error {
		o.IOMMUEnabled = enabled

		return nil
	}
}

// WithExtraUEFISearchPaths configures additional search paths to look for UEFI firmware.
func WithExtraUEFISearchPaths(extraUEFISearchPaths []string) Option {
	return func(o *Options) error {
		o.ExtraUEFISearchPaths = extraUEFISearchPaths

		return nil
	}
}

// WithTargetArch specifies target architecture for the cluster.
func WithTargetArch(arch string) Option {
	return func(o *Options) error {
		o.TargetArch = arch

		return nil
	}
}

// WithDockerPorts allows docker provisioner to expose ports on workers.
func WithDockerPorts(ports []string) Option {
	return func(o *Options) error {
		o.DockerPorts = ports

		return nil
	}
}

// WithDockerPortsHostIP sets host IP for docker provisioner to expose ports on workers.
func WithDockerPortsHostIP(hostIP string) Option {
	return func(o *Options) error {
		o.DockerPortsHostIP = hostIP

		return nil
	}
}

// WithDeleteOnErr informs the provisioner to delete cluster state folder on error.
func WithDeleteOnErr(v bool) Option {
	return func(o *Options) error {
		o.DeleteStateOnErr = v

		return nil
	}
}

// WithSaveSupportArchivePath specifies path to save support archive on destroy.
func WithSaveSupportArchivePath(path string) Option {
	return func(o *Options) error {
		o.SaveSupportArchivePath = path

		return nil
	}
}

// WithSaveClusterLogsArchivePath specifies path to save cluster logs archive on destroy.
func WithSaveClusterLogsArchivePath(path string) Option {
	return func(o *Options) error {
		o.SaveClusterLogsArchivePath = path

		return nil
	}
}

// WithKMS inits KMS server in the provisioner.
func WithKMS(endpoint string) Option {
	return func(o *Options) error {
		o.KMSEndpoint = endpoint

		return nil
	}
}

// WithJSONLogs specifies endpoint to send logs in JSON format.
func WithJSONLogs(endpoint string) Option {
	return func(o *Options) error {
		o.JSONLogsEndpoint = endpoint

		return nil
	}
}

// WithBGP enables an embedded gobgp speaker acting as a fabric peer for testing native BGP.
func WithBGP(listenAddress, neighborRange, advertise string, localASN, peerASN uint32) Option {
	return func(o *Options) error {
		o.BGPEnabled = true
		o.BGPListenAddress = listenAddress
		o.BGPNeighborRange = neighborRange
		o.BGPAdvertise = advertise
		o.BGPLocalASN = localASN
		o.BGPPeerASN = peerASN

		return nil
	}
}

// WithBGPCLOS enables the fabric peer in full-CLOS mode: nodes have no net0, only dedicated fabric
// uplink(s). The peer peers unnumbered over every uplink, sends Router Advertisements, programs each
// node's learned loopback /32 into the host FIB (zebra), and IP-forwards + masquerades the node loopback
// CIDR so the nodes (reachable only via BGP) can reach the host services and the internet. The per-node
// uplink count comes from NetworkRequest.FabricUplinks. Linux-only (host FIB + NAT).
func WithBGPCLOS(advertise string, localASN, peerASN uint32, loopbackCIDR string) Option {
	return func(o *Options) error {
		o.BGPEnabled = true
		o.BGPCLOS = true
		o.BGPAdvertise = advertise
		o.BGPLocalASN = localASN
		o.BGPPeerASN = peerASN
		o.BGPLoopbackCIDR = loopbackCIDR

		return nil
	}
}

// WithSiderolinkAgent enables or disables siderolink agent.
func WithSiderolinkAgent(v bool) Option {
	return func(o *Options) error {
		o.SiderolinkEnabled = v

		return nil
	}
}

// WithSkipInjectingExtraCmdline prevents injecting extra kernel args into EFI vars.
func WithSkipInjectingExtraCmdline(v bool) Option {
	return func(o *Options) error {
		o.SkipInjectingExtraCmdline = v

		return nil
	}
}

// Options describes Provisioner parameters.
type Options struct {
	LogWriter          io.Writer
	TalosConfig        *clientconfig.Config
	TalosClient        *client.Client
	KubernetesEndpoint string
	TargetArch         string

	// Enable bootloader by booting from disk image after install.
	BootloaderEnabled bool

	// SkipInjectingExtraCmdline prevents injecting extra kernel args, e.g., console=ttyS0, into the EFI vars. Only applies when UEFI is enabled.
	SkipInjectingExtraCmdline bool

	// Enable UEFI (for amd64), arm64 can only boot UEFI
	UEFIEnabled bool
	// Enable TPM 1.2 emulation using swtpm.
	TPM1_2Enabled bool
	// Enable TPM 2.0 emulation using swtpm.
	TPM2Enabled bool
	// Enable IOMMU for VMs and add a new PCI root controller and network interface.
	IOMMUEnabled bool
	// Configure additional search paths to look for UEFI firmware.
	ExtraUEFISearchPaths []string

	// Expose ports to worker machines in docker provisioner
	DockerPorts                []string
	DockerPortsHostIP          string
	SaveSupportArchivePath     string
	SaveClusterLogsArchivePath string
	DeleteStateOnErr           bool

	KMSEndpoint      string
	JSONLogsEndpoint string

	SiderolinkEnabled bool

	// BGP test fabric peer (embedded gobgp), enabled by --with-bgp.
	BGPEnabled       bool
	BGPListenAddress string
	BGPNeighborRange string
	BGPAdvertise     string
	BGPLocalASN      uint32
	BGPPeerASN       uint32
	// BGPCLOS runs the fabric peer in full-CLOS mode: unnumbered over every node's dedicated fabric
	// uplink(s) + Router Advertisements + host FIB programming (zebra) + NAT; enabled by --with-bgp-clos.
	BGPCLOS bool
	// BGPLoopbackCIDR is the node loopback identity CIDR the full-CLOS fabric peer IP-forwards and
	// masquerades so the (BGP-only) nodes can reach the host services and the internet.
	BGPLoopbackCIDR string
}

// DefaultOptions returns default options.
func DefaultOptions() Options {
	return Options{
		BootloaderEnabled: true,
		TargetArch:        runtime.GOARCH,
		LogWriter:         os.Stderr,
		DockerPortsHostIP: "0.0.0.0",
	}
}
