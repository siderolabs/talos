// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package provision

import (
	"errors"
	"net/netip"
	"slices"
	"time"

	mounttypes "github.com/docker/docker/api/types/mount"
	"github.com/google/uuid"
	"github.com/siderolabs/go-procfs/procfs"

	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
)

// ClusterRequest is the root object describing cluster to be provisioned.
type ClusterRequest struct {
	Name string

	Network NetworkRequest
	Nodes   NodeRequests

	// Docker specific parameters.
	Image string

	// Boot options (QEMU).
	KernelPath     string
	InitramfsPath  string
	ISOPath        string
	USBPath        string
	DiskImagePath  string
	IPXEBootScript string

	// Encryption
	KMSEndpoint string

	// Path to talosctl executable to re-execute itself as needed.
	SelfExecutable string

	// Path to root of state directory (~/.talos/clusters by default).
	StateDirectory string

	SiderolinkRequest SiderolinkRequest
}

// CNIConfig describes CNI part of NetworkRequest.
type CNIConfig struct {
	BinPath  []string
	ConfDir  string
	CacheDir string

	BundleURL string
}

// NetworkRequest describes cluster network.
type NetworkRequest struct {
	Name              string
	CIDRs             []netip.Prefix
	NoMasqueradeCIDRs []netip.Prefix
	GatewayAddrs      []netip.Addr
	MTU               int
	Nameservers       []netip.Addr

	LoadBalancerPorts []int

	// CNI-specific parameters.
	CNI CNIConfig

	// DHCP options
	DHCPSkipHostname bool

	// Docker-specific parameters.
	DockerDisableIPv6 bool

	// Network chaos parameters.
	NetworkChaos  bool
	Jitter        time.Duration
	Latency       time.Duration
	PacketLoss    float64
	PacketReorder float64
	PacketCorrupt float64
	Bandwidth     int
}

// NodeRequests is a list of NodeRequest.
type NodeRequests []NodeRequest

// FindInitNode looks up init node, it returns an error if no init node is present or if it's duplicate.
func (reqs NodeRequests) FindInitNode() (req NodeRequest, err error) {
	found := false

	for i := range reqs {
		if reqs[i].Config == nil {
			continue
		}

		if reqs[i].Config.Machine().Type() == machine.TypeInit {
			if found {
				err = errors.New("duplicate init node in requests")

				return
			}

			req = reqs[i]
			found = true
		}
	}

	if !found {
		err = errors.New("no init node found in requests")
	}

	return
}

// ControlPlaneNodes returns subset of nodes which are Init/ControlPlane type.
func (reqs NodeRequests) ControlPlaneNodes() (nodes []NodeRequest) {
	for i := range reqs {
		if reqs[i].Type == machine.TypeInit || reqs[i].Type == machine.TypeControlPlane {
			nodes = append(nodes, reqs[i])
		}
	}

	return
}

// WorkerNodes returns subset of nodes which are Init/ControlPlane type.
func (reqs NodeRequests) WorkerNodes() (nodes []NodeRequest) {
	for i := range reqs {
		if reqs[i].Type == machine.TypeWorker {
			nodes = append(nodes, reqs[i])
		}
	}

	return
}

// PXENodes returns subset of nodes which are PXE booted.
func (reqs NodeRequests) PXENodes() (nodes []NodeRequest) {
	for i := range reqs {
		if reqs[i].PXEBooted {
			nodes = append(nodes, reqs[i])
		}
	}

	return
}

// Disk represents a disk size and name in NodeRequest.
type Disk struct {
	// Size in bytes.
	Size uint64
	// Whether to skip preallocating the disk space.
	SkipPreallocate bool
	// Partitions represents the list of partitions.
	Partitions []*v1alpha1.DiskPartition
	// Driver for the disk.
	//
	// Supported types: "virtio", "ide", "ahci", "scsi", "nvme", "megaraid".
	Driver string
	// Block size for the disk, defaults to 512 if not set.
	BlockSize uint
}

// ConfigInjectionMethod describes how to inject configuration into the node.
type ConfigInjectionMethod int

const (
	// ConfigInjectionMethodHTTP injects configuration via HTTP.
	ConfigInjectionMethodHTTP ConfigInjectionMethod = iota
	// ConfigInjectionMethodMetalISO injects configuration via Metal ISO.
	ConfigInjectionMethodMetalISO
)

// NodeRequest describes a request for a node.
type NodeRequest struct {
	Name string
	IPs  []netip.Addr
	Type machine.Type

	Config                config.Provider
	ConfigInjectionMethod ConfigInjectionMethod

	// Share of CPUs, in 1e-9 fractions
	NanoCPUs int64
	// Memory limit in bytes
	Memory int64
	// Disks (volumes), if applicable (VM only)
	Disks []*Disk
	// Mounts (containers only)
	Mounts []mounttypes.Mount
	// Ports
	Ports []string
	// SkipInjectingConfig disables reading configuration from http server
	SkipInjectingConfig bool
	// DefaultBootOrder overrides default boot order "cn" (disk, then network boot).
	//
	// BootOrder can be forced to be "nc" (PXE boot) via the API in QEMU provisioner.
	DefaultBootOrder string

	// ExtraKernelArgs passes additional kernel args
	// to the initial boot from initramfs and vmlinuz.
	//
	// This doesn't apply to boots from ISO or from the disk image.
	ExtraKernelArgs *procfs.Cmdline

	// UUID allows to specify the UUID of the node (VMs only).
	//
	// If not specified, a random UUID will be generated.
	UUID *uuid.UUID

	// Testing features

	// BadRTC resets RTC to well known time in the past (QEMU provisioner).
	BadRTC bool

	// PXE-booted VMs
	PXEBooted        bool
	TFTPServer       string
	IPXEBootFilename string
}

// SiderolinkRequest describes a request for SideroLink agent.
type SiderolinkRequest struct {
	WireguardEndpoint string
	APIEndpoint       string
	APICertificate    []byte
	APIKey            []byte
	SinkEndpoint      string
	LogEndpoint       string
	SiderolinkBind    []SiderolinkBind
}

// GetAddr returns the address for the given UUID.
func (sr *SiderolinkRequest) GetAddr(u *uuid.UUID) (netip.Addr, bool) {
	if idx := slices.IndexFunc(sr.SiderolinkBind, func(sb SiderolinkBind) bool { return sb.UUID == *u }); idx != -1 {
		return sr.SiderolinkBind[idx].Addr, true
	}

	return netip.Addr{}, false
}

// SiderolinkBind describes a pair of prebinded UUID->Addr for SideroLink agent.
type SiderolinkBind struct {
	UUID uuid.UUID
	Addr netip.Addr
}
