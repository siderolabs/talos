// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package provision

import (
	"net"

	"github.com/talos-systems/talos/pkg/config/machine"
)

// Cluster describes the provisioned Cluster.
type Cluster interface {
	// Provisioner returns name of the provisioner used to build the cluster.
	Provisioner() string
	// StatePath returns path to the state directory of the cluster.
	StatePath() (string, error)
	// Info returns running cluster information.
	Info() ClusterInfo
}

// ClusterInfo describes the cluster.
type ClusterInfo struct {
	ClusterName string

	Network NetworkInfo
	Nodes   []NodeInfo
}

// NetworkInfo describes cluster network.
type NetworkInfo struct {
	Name        string
	CIDR        net.IPNet
	GatewayAddr net.IP
	MTU         int
}

// NodeInfo describes a node.
type NodeInfo struct {
	ID   string
	Name string
	Type machine.Type

	// Share of CPUs, in 1e-9 fractions
	NanoCPUs int64
	// Memory limit in bytes
	Memory int64
	// Disk (volume) size in bytes, if applicable
	DiskSize int64

	PublicIP  net.IP
	PrivateIP net.IP
}
