// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"errors"
	"os"

	"github.com/talos-systems/go-blockdevice/blockdevice/probe"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/adv"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha2"
	"github.com/talos-systems/talos/pkg/machinery/constants"
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
	disk *probe.ProbedBlockDevice

	stagedInstall         bool
	stagedInstallImageRef string
	stagedInstallOptions  []byte
}

// ClusterState represents the cluster's state.
type ClusterState struct {
}

// NewState initializes and returns the v1alpha1 state.
func NewState() (s *State, err error) {
	p, err := platform.CurrentPlatform()
	if err != nil {
		return nil, err
	}

	machine := &MachineState{}

	err = machine.probeDisk()
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}

	cluster := &ClusterState{}

	v2State, err := v1alpha2.NewState()
	if err != nil {
		return nil, err
	}

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

func (s *MachineState) probeDisk() error {
	if s.disk != nil {
		return nil
	}

	var dev *probe.ProbedBlockDevice

	dev, err := probe.GetDevWithFileSystemLabel(constants.EphemeralPartitionLabel)
	if err == nil {
		s.disk = dev
	}

	if err != nil {
		return err
	}

	meta, err := bootloader.NewMeta()
	if err != nil {
		// ignore missing meta
		return nil
	}

	defer meta.Close() //nolint: errcheck

	stagedInstallImageRef, ok1 := meta.ADV.ReadTag(adv.StagedUpgradeImageRef)
	stagedInstallOptions, ok2 := meta.ADV.ReadTag(adv.StagedUpgradeInstallOptions)

	s.stagedInstall = ok1 && ok2

	if s.stagedInstall {
		// clear the staged install flags
		meta.ADV.DeleteTag(adv.StagedUpgradeImageRef)
		meta.ADV.DeleteTag(adv.StagedUpgradeInstallOptions)

		if err = meta.Write(); err != nil {
			// failed to delete staged install tags, clear the stagedInstall to prevent boot looping
			s.stagedInstall = false
		}

		s.stagedInstallImageRef = stagedInstallImageRef
		s.stagedInstallOptions = []byte(stagedInstallOptions)
	}

	return nil
}

// Disk implements the machine state interface.
func (s *MachineState) Disk() *probe.ProbedBlockDevice {
	s.probeDisk() //nolint: errcheck

	return s.disk
}

// Close implements the machine state interface.
func (s *MachineState) Close() error {
	if s.disk != nil {
		return s.disk.Close()
	}

	return nil
}

// Installed implements the machine state interface.
func (s *MachineState) Installed() bool {
	s.probeDisk() //nolint: errcheck

	return s.disk != nil
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
