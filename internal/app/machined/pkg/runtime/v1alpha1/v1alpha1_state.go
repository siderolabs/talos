// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"context"
	"log"
	"os"
	"sync"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha2"
	"github.com/siderolabs/talos/internal/pkg/meta"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	metaconsts "github.com/siderolabs/talos/pkg/machinery/meta"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// State implements the state interface.
type State struct {
	platform runtime.Platform
	machine  *MachineState
	cluster  *ClusterState
	v2       runtime.V1Alpha2State
}

// MachineState represents the machine's state.
type MachineState struct {
	platform  runtime.Platform
	resources state.State

	meta     *meta.Meta
	metaOnce sync.Once

	stagedInstall         bool
	stagedInstallImageRef string
	stagedInstallOptions  []byte

	kexecPrepared bool

	dbus DBusState
}

// ClusterState represents the cluster's state.
type ClusterState struct{}

// NewState initializes and returns the v1alpha1 state.
func NewState() (s *State, err error) {
	p, err := platform.CurrentPlatform()
	if err != nil {
		return nil, err
	}

	v2State, err := v1alpha2.NewState()
	if err != nil {
		return nil, err
	}

	machine := &MachineState{
		platform:  p,
		resources: v2State.Resources(),
	}

	cluster := &ClusterState{}

	s = &State{
		platform: p,
		cluster:  cluster,
		machine:  machine,
		v2:       v2State,
	}

	return s, nil
}

// Platform implements the state interface.
func (s *State) Platform() runtime.Platform {
	return s.platform
}

// Machine implements the state interface.
func (s *State) Machine() runtime.MachineState {
	return s.machine
}

// Cluster implements the state interface.
func (s *State) Cluster() runtime.ClusterState {
	return s.cluster
}

// V1Alpha2 implements the state interface.
func (s *State) V1Alpha2() runtime.V1Alpha2State {
	return s.v2
}

// Meta implements the runtime.MachineState interface.
func (s *MachineState) Meta() runtime.Meta {
	// no META in container mode
	if s.platform.Mode() == runtime.ModeContainer {
		return s
	}

	var (
		justLoaded bool
		loadErr    error
	)

	s.metaOnce.Do(func() {
		s.meta, loadErr = meta.New(context.Background(), s.resources)
		if loadErr != nil {
			if !os.IsNotExist(loadErr) {
				log.Printf("META: failed to load: %s", loadErr)
			}
		} else {
			s.probeMeta()
		}

		justLoaded = true
	})

	return metaWrapper{
		MachineState: s,
		justLoaded:   justLoaded,
		loadErr:      loadErr,
	}
}

// ReadTag implements the runtime.Meta interface.
func (s *MachineState) ReadTag(t uint8) (val string, ok bool) {
	if s.platform.Mode() == runtime.ModeContainer {
		return "", false
	}

	return s.meta.ReadTag(t)
}

// ReadTagBytes implements the runtime.Meta interface.
func (s *MachineState) ReadTagBytes(t uint8) (val []byte, ok bool) {
	if s.platform.Mode() == runtime.ModeContainer {
		return nil, false
	}

	return s.meta.ReadTagBytes(t)
}

// SetTag implements the runtime.Meta interface.
func (s *MachineState) SetTag(ctx context.Context, t uint8, val string) (bool, error) {
	if s.platform.Mode() == runtime.ModeContainer {
		return false, nil
	}

	return s.meta.SetTag(ctx, t, val)
}

// SetTagBytes implements the runtime.Meta interface.
func (s *MachineState) SetTagBytes(ctx context.Context, t uint8, val []byte) (bool, error) {
	if s.platform.Mode() == runtime.ModeContainer {
		return false, nil
	}

	return s.meta.SetTagBytes(ctx, t, val)
}

// DeleteTag implements the runtime.Meta interface.
func (s *MachineState) DeleteTag(ctx context.Context, t uint8) (bool, error) {
	if s.platform.Mode() == runtime.ModeContainer {
		return false, nil
	}

	return s.meta.DeleteTag(ctx, t)
}

// Reload implements the runtime.Meta interface.
func (s *MachineState) Reload(ctx context.Context) error {
	if s.platform.Mode() == runtime.ModeContainer {
		return nil
	}

	err := s.meta.Reload(ctx)
	if err == nil {
		s.probeMeta()
	}

	return err
}

// Flush implements the runtime.Meta interface.
func (s *MachineState) Flush() error {
	if s.platform.Mode() == runtime.ModeContainer {
		return nil
	}

	return s.meta.Flush()
}

func (s *MachineState) probeMeta() {
	stagedInstallImageRef, ok1 := s.meta.ReadTag(metaconsts.StagedUpgradeImageRef)
	stagedInstallOptions, ok2 := s.meta.ReadTag(metaconsts.StagedUpgradeInstallOptions)

	s.stagedInstall = ok1 && ok2

	if s.stagedInstall {
		// clear the staged install flags
		_, err1 := s.meta.DeleteTag(context.Background(), metaconsts.StagedUpgradeImageRef)
		_, err2 := s.meta.DeleteTag(context.Background(), metaconsts.StagedUpgradeInstallOptions)

		if err := s.meta.Flush(); err != nil || err1 != nil || err2 != nil {
			// failed to delete staged install tags, clear the stagedInstall to prevent boot looping
			s.stagedInstall = false
		}

		s.stagedInstallImageRef = stagedInstallImageRef
		s.stagedInstallOptions = []byte(stagedInstallOptions)
	}
}

// Installed implements the machine state interface.
func (s *MachineState) Installed() bool {
	// undefined in container mode
	if s.platform.Mode() == runtime.ModeContainer {
		return true
	}

	// legacy flow, no context available
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	metaStatus, err := safe.StateWatchFor[*block.VolumeStatus](
		ctx,
		s.resources,
		block.NewVolumeStatus(block.NamespaceName, constants.MetaPartitionLabel).Metadata(),
		state.WithEventTypes(state.Created, state.Updated),
		state.WithCondition(func(r resource.Resource) (bool, error) {
			vs, ok := r.(*block.VolumeStatus)
			if !ok {
				return false, nil
			}

			switch vs.TypedSpec().Phase { //nolint:exhaustive
			case block.VolumePhaseMissing:
				// no META, talos is not installed
				return true, nil
			case block.VolumePhaseReady:
				// META found
				return true, nil
			default:
				return false, nil
			}
		}),
	)
	if err != nil {
		return false
	}

	return metaStatus.TypedSpec().Phase == block.VolumePhaseReady
}

// IsInstallStaged implements the machine state interface.
func (s *MachineState) IsInstallStaged() bool {
	return s.stagedInstall
}

// StagedInstallImageRef implements the machine state interface.
func (s *MachineState) StagedInstallImageRef() string {
	return s.stagedInstallImageRef
}

// StagedInstallOptions implements the machine state interface.
func (s *MachineState) StagedInstallOptions() []byte {
	return s.stagedInstallOptions
}

// KexecPrepared implements the machine state interface.
func (s *MachineState) KexecPrepared(prepared bool) {
	s.kexecPrepared = prepared
}

// IsKexecPrepared implements the machine state interface.
func (s *MachineState) IsKexecPrepared() bool {
	return s.kexecPrepared
}

// DBus implements the machine state interface.
func (s *MachineState) DBus() runtime.DBusState {
	return &s.dbus
}

type metaWrapper struct {
	*MachineState
	justLoaded bool
	loadErr    error
}

func (m metaWrapper) Reload(ctx context.Context) error {
	if m.justLoaded {
		return m.loadErr
	}

	return m.MachineState.Reload(ctx)
}
