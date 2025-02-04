// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package docker

import (
	mounttypes "github.com/docker/docker/api/types/mount"

	"github.com/siderolabs/talos/pkg/provision"
)

// ClusterRequest is the docker cluster request.
type ClusterRequest struct {
	provision.ClusterRequestBase

	// Docker specific parameters.
	Image   string
	Network NetworkRequest
	Nodes   NodeRequests
}

// NodeRequests are the node requests.
type NodeRequests []NodeRequest

// NodeRequest is the docker specific node request.
type NodeRequest struct {
	provision.NodeRequestBase

	Mounts []mounttypes.Mount
	Ports  []string
}

// NetworkRequest is the docker specific network request.
type NetworkRequest struct {
	provision.NetworkRequestBase

	DockerDisableIPv6 bool
}
