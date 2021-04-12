// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/registry"
	"github.com/talos-systems/go-blockdevice/blockdevice/probe"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/disk"
	"github.com/talos-systems/talos/pkg/machinery/config"
)

// State defines the state.
type State interface {
	Platform() Platform
	Machine() MachineState
	Cluster() ClusterState
	V1Alpha2() V1Alpha2State
}

// Machine defines the runtime parameters.
type Machine interface {
	State() MachineState
	Config() config.MachineConfig
}

// MachineState defines the machined state.
type MachineState interface {
	Disk(options ...disk.Option) *probe.ProbedBlockDevice
	Close() error
	Installed() bool
	IsInstallStaged() bool
	StagedInstallImageRef() string
	StagedInstallOptions() []byte
}

// ClusterState defines the cluster state.
type ClusterState interface{}

// V1Alpha2State defines the next generation (v2) interface binding into v1 runtime.
type V1Alpha2State interface {
	Resources() state.State

	NamespaceRegistry() *registry.NamespaceRegistry
	ResourceRegistry() *registry.ResourceRegistry

	SetConfig(config.Provider) error
}
