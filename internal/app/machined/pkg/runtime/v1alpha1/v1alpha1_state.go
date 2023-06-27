// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"context"
	"errors"
	"log"
	"os"
	"sync"

	"github.com/cosi-project/runtime/pkg/state"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/siderolabs/go-blockdevice/blockdevice/probe"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/disk"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha2"
	"github.com/siderolabs/talos/internal/pkg/meta"
	"github.com/siderolabs/talos/pkg/machinery/constants"
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

	disks map[string]*probe.ProbedBlockDevice

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

	err = machine.probeDisks()
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
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

func (s *MachineState) probeDisks(labels ...string) error {
	if s.platform.Mode() == runtime.ModeContainer {
		return os.ErrNotExist
	}

	if len(labels) == 0 {
		labels = []string{constants.EphemeralPartitionLabel, constants.BootPartitionLabel, constants.EFIPartitionLabel, constants.StatePartitionLabel}
	}

	if s.disks == nil {
		s.disks = map[string]*probe.ProbedBlockDevice{}
	}

	for _, label := range labels {
		if _, ok := s.disks[label]; ok {
			continue
		}

		var dev *probe.ProbedBlockDevice

		dev, err := probe.GetDevWithPartitionName(label)
		if err != nil {
			return err
		}

		s.disks[label] = dev
	}

	return nil
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
	stagedInstallImageRef, ok1 := s.meta.ReadTag(meta.StagedUpgradeImageRef)
	stagedInstallOptions, ok2 := s.meta.ReadTag(meta.StagedUpgradeInstallOptions)

	s.stagedInstall = ok1 && ok2

	if s.stagedInstall {
		// clear the staged install flags
		_, err1 := s.meta.DeleteTag(context.Background(), meta.StagedUpgradeImageRef)
		_, err2 := s.meta.DeleteTag(context.Background(), meta.StagedUpgradeInstallOptions)

		if err := s.meta.Flush(); err != nil || err1 != nil || err2 != nil {
			// failed to delete staged install tags, clear the stagedInstall to prevent boot looping
			s.stagedInstall = false
		}

		s.stagedInstallImageRef = stagedInstallImageRef
		s.stagedInstallOptions = []byte(stagedInstallOptions)
	}
}

// Disk implements the machine state interface.
func (s *MachineState) Disk(options ...disk.Option) *probe.ProbedBlockDevice {
	opts := &disk.Options{
		Label: constants.EphemeralPartitionLabel,
	}

	for _, opt := range options {
		opt(opts)
	}

	s.probeDisks(opts.Label) //nolint:errcheck

	return s.disks[opts.Label]
}

// Close implements the machine state interface.
func (s *MachineState) Close() error {
	var result *multierror.Error

	for label, disk := range s.disks {
		if err := disk.Close(); err != nil {
			e := multierror.Append(result, err)
			if e != nil {
				return e
			}
		}

		delete(s.disks, label)
	}

	return result.ErrorOrNil()
}

// Installed implements the machine state interface.
func (s *MachineState) Installed() bool {
	return s.Disk(
		disk.WithPartitionLabel(constants.EphemeralPartitionLabel),
	) != nil
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
