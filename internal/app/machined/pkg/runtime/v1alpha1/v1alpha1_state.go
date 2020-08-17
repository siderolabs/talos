// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"errors"
	"os"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform"
	"github.com/talos-systems/talos/pkg/blockdevice/probe"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// State implements the state interface.
type State struct {
	platform runtime.Platform
	machine  *MachineState
	cluster  *ClusterState
}

// MachineState represents the machine's state.
type MachineState struct {
	disk *probe.ProbedBlockDevice
}

// ClusterState represents the cluster's state.
type ClusterState struct {
	disk *probe.ProbedBlockDevice
}

// NewState initializes and returns the v1alpha1 state.
func NewState() (s *State, err error) {
	var dev *probe.ProbedBlockDevice

	dev, err = probe.GetDevWithFileSystemLabel(constants.EphemeralPartitionLabel)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}

	p, err := platform.CurrentPlatform()
	if err != nil {
		return nil, err
	}

	machine := &MachineState{
		disk: dev,
	}

	cluster := &ClusterState{
		disk: dev,
	}

	s = &State{
		platform: p,
		cluster:  cluster,
		machine:  machine,
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

// Disk implements the machine state interface.
func (s *MachineState) Disk() *probe.ProbedBlockDevice {
	if s.disk == nil {
		var dev *probe.ProbedBlockDevice

		dev, err := probe.GetDevWithFileSystemLabel(constants.EphemeralPartitionLabel)
		if err == nil {
			s.disk = dev
		}
	}

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
	if s.disk == nil {
		var dev *probe.ProbedBlockDevice

		dev, err := probe.GetDevWithFileSystemLabel(constants.EphemeralPartitionLabel)
		if err == nil {
			s.disk = dev
		}
	}

	return s.disk != nil
}
