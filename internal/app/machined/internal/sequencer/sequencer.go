/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package sequencer

import (
	"github.com/talos-systems/talos/internal/app/machined/internal/sequencer/v1alpha1"
	"github.com/talos-systems/talos/internal/app/machined/proto"
)

// Sequencer describes the boot, shutdown, and upgrade events.
type Sequencer interface {
	Boot() error
	Shutdown() error
	Upgrade(*proto.UpgradeRequest) error
}

// Version represents the sequencer version.
type Version int

const (
	// V1Alpha1 is the v1alpha1 sequencer.
	V1Alpha1 = iota
)

// New initializes and returns a sequencer based on the specified version.
func New(v Version) Sequencer {
	switch v {
	case V1Alpha1:
		return &v1alpha1.Sequencer{}
	default:
		return nil
	}
}
