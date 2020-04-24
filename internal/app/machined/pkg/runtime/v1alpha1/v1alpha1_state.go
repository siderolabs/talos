// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"errors"
	"os"
	"sync"

	"github.com/talos-systems/talos/pkg/blockdevice/probe"
	"github.com/talos-systems/talos/pkg/universe"
)

var once sync.Once

// NewState initializes and returns the v1alpha1 state.
func NewState() (state *State, err error) {
	var dev *probe.ProbedBlockDevice

	dev, err = probe.GetDevWithFileSystemLabel(universe.EphemeralPartitionLabel)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}

	state = &State{
		disk: dev,
	}

	return state, nil
}

// State implements the State interface.
type State struct {
	disk   *probe.ProbedBlockDevice
	config []byte
}

// Disk implements the state interface.
func (s *State) Disk() *probe.ProbedBlockDevice {
	return s.disk
}

// Installed implements the state interface/
func (s *State) Installed() bool {
	return s.disk == nil
}

// Config implements the state interface/
func (s *State) Config() []byte {
	return s.config
}

// SetConfig implements the state interface/
func (s *State) SetConfig(b []byte) {
	once.Do(func() {
		s.config = b
	})
}
