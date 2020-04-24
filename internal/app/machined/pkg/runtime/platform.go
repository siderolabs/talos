// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"net"

	"github.com/talos-systems/go-procfs/procfs"
)

// Platform defines the requirements for a platform.
type Platform interface {
	Name() string
	Configuration() ([]byte, error)
	Hostname() ([]byte, error)
	Mode() Mode
	ExternalIPs() ([]net.IP, error)
	KernelArgs() procfs.Parameters
}
