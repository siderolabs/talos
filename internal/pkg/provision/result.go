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
	Name string
	CIDR net.IPNet
}

// NodeInfo describes a node.
type NodeInfo struct {
	ID   string
	Name string
	Type machine.Type

	PublicIP  net.IP
	PrivateIP net.IP
}
