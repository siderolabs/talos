// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package provision

import (
	"fmt"
	"net"

	"github.com/talos-systems/go-procfs/procfs"

	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
)

// ClusterRequest is the root object describing cluster to be provisioned.
type ClusterRequest struct {
	Name string

	Network NetworkRequest
	Nodes   NodeRequests

	Image         string
	KernelPath    string
	InitramfsPath string
	ISOPath       string
	DiskImagePath string

	// Path to talosctl executable to re-execute itself as needed.
	SelfExecutable string

	// Path to root of state directory (~/.talos/clusters by default).
	StateDirectory string
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
	Name         string
	CIDRs        []net.IPNet
	GatewayAddrs []net.IP
	MTU          int
	Nameservers  []net.IP

	// CNI-specific parameters.
	CNI CNIConfig
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
				err = fmt.Errorf("duplicate init node in requests")

				return
			}

			req = reqs[i]
			found = true
		}
	}

	if !found {
		err = fmt.Errorf("no init node found in requests")
	}

	return
}

// MasterNodes returns subset of nodes which are Init/ControlPlane type.
func (reqs NodeRequests) MasterNodes() (nodes []NodeRequest) {
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
	// Partitions represents the list of partitions.
	Partitions []*v1alpha1.DiskPartition
}

// NodeRequest describes a request for a node.
type NodeRequest struct {
	Name   string
	IPs    []net.IP
	Config config.Provider
	Type   machine.Type

	// Share of CPUs, in 1e-9 fractions
	NanoCPUs int64
	// Memory limit in bytes
	Memory int64
	// Disks (volumes), if applicable
	Disks []*Disk
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

	// Testing features

	// BadRTC resets RTC to well known time in the past (QEMU provisioner).
	BadRTC bool

	// PXE-booted VMs
	PXEBooted        bool
	TFTPServer       string
	IPXEBootFilename string
}
