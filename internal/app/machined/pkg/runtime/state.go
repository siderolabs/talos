// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/registry"

	configcore "github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/config"
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
	Installed() bool
	IsInstallStaged() bool
	StagedInstallImageRef() string
	StagedInstallOptions() []byte
	KexecPrepared(bool)
	IsKexecPrepared() bool
	DBus() DBusState
	Meta() Meta
}

// Meta defines the access to META partition.
type Meta interface {
	ReadTag(t uint8) (val string, ok bool)
	ReadTagBytes(t uint8) (val []byte, ok bool)
	SetTag(ctx context.Context, t uint8, val string) (bool, error)
	SetTagBytes(ctx context.Context, t uint8, val []byte) (bool, error)
	DeleteTag(ctx context.Context, t uint8) (bool, error)
	Reload(ctx context.Context) error
	Flush() error
}

// ClusterState defines the cluster state.
type ClusterState any

// V1Alpha2State defines the next generation (v2) interface binding into v1 runtime.
type V1Alpha2State interface {
	Resources() state.State

	NamespaceRegistry() *registry.NamespaceRegistry
	ResourceRegistry() *registry.ResourceRegistry

	SetConfig(configcore.Provider) error
}

// DBusState defines the D-Bus logind mock.
type DBusState interface {
	Start() error
	Stop() error
	WaitShutdown(ctx context.Context) error
}
