// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"github.com/talos-systems/talos/pkg/blockdevice/probe"
	"github.com/talos-systems/talos/pkg/machinery/config"
)

// State defines the state.
type State interface {
	Platform() Platform
	Machine() MachineState
	Cluster() ClusterState
}

// Machine defines the runtime parameters.
type Machine interface {
	State() MachineState
	Config() config.MachineConfig
}

// MachineState defines the machined state.
type MachineState interface {
	Disk() *probe.ProbedBlockDevice
	Close() error
	Installed() bool
}

// ClusterState defines the cluster state.
type ClusterState interface{}
