// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import "github.com/talos-systems/talos/pkg/blockdevice/probe"

// State defines the state interface.
type State interface {
	Disk() *probe.ProbedBlockDevice
	Shutdown() error
	Config() []byte
	SetConfig([]byte)
	Installed() bool
}
