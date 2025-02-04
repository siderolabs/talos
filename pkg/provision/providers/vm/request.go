// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vm

import (
	"net/netip"
	"time"

	"github.com/google/uuid"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-procfs/procfs"

	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
	"github.com/siderolabs/talos/pkg/provision"
)

// ClusterRequest is the vm cluster request.
type ClusterRequest struct {
	provision.ClusterRequestBase

	Network NetworkRequest
	Nodes   NodeRequests

	// Boot options
	KernelPath     string
	InitramfsPath  string
	ISOPath        string
	USBPath        string
	UKIPath        string
	DiskImagePath  string
	IPXEBootScript string

	// Encryption
	KMSEndpoint       string
	SiderolinkRequest provision.SiderolinkRequest
}

// ConfigInjectionMethod describes how to inject configuration into the node.
type ConfigInjectionMethod int

const (
	// ConfigInjectionMethodHTTP injects configuration via HTTP.
	ConfigInjectionMethodHTTP ConfigInjectionMethod = iota
	// ConfigInjectionMethodMetalISO injects configuration via Metal ISO.
	ConfigInjectionMethodMetalISO
)

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
	// Supported types: "virtio", "ide", "ahci", "scsi", "nvme".
	Driver string
	// Block size for the disk, defaults to 512 if not set.
	BlockSize uint
}

// GetBase returns the base node list.
func (n NodeRequests) GetBase() provision.BaseNodeRequests {
	base := xslices.Map(n, func(n NodeRequest) provision.NodeRequestBase {
		return n.NodeRequestBase
	})

	return provision.BaseNodeRequests(base)
}

// PXENodes returns subset of nodes which are PXE booted.
func (n NodeRequests) PXENodes() (nodes NodeRequests) {
	for i := range n {
		if n[i].PXEBooted {
			nodes = append(nodes, n[i])
		}
	}

	return
}

// NodeRequests are the node requests.
type NodeRequests []NodeRequest

// NodeRequest is the qemu specific node request.
type NodeRequest struct {
	provision.NodeRequestBase

	ConfigInjectionMethod ConfigInjectionMethod
	// Disks (volumes), if applicable (VM only)
	Disks []*Disk

	// DefaultBootOrder overrides default boot order "cn" (disk, then network boot).
	//
	// BootOrder can be forced to be "nc" (PXE boot) via the API in QEMU provisioner.
	DefaultBootOrder string

	// ExtraKernelArgs passes additional kernel args
	// to the initial boot from initramfs and vmlinuz.
	//
	// This doesn't apply to boots from ISO or from the disk image.
	ExtraKernelArgs *procfs.Cmdline

	Quirks quirks.Quirks

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

// NetworkRequest describes qemu cluster network.
type NetworkRequest struct {
	provision.NetworkRequestBase
	MTU               int
	Nameservers       []netip.Addr
	NoMasqueradeCIDRs []netip.Prefix
	LoadBalancerPorts []int

	// CNI-specific parameters.
	CNI CNIConfig

	// DHCP options
	DHCPSkipHostname bool

	// Network chaos parameters.
	NetworkChaos  bool
	Jitter        time.Duration
	Latency       time.Duration
	PacketLoss    float64
	PacketReorder float64
	PacketCorrupt float64
	Bandwidth     int
}

// CNIConfig describes CNI part of NetworkRequest.
type CNIConfig struct {
	BinPath  []string
	ConfDir  string
	CacheDir string

	BundleURL string
}
