// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"net"

	"github.com/talos-systems/go-procfs/procfs"
)

// Platform defines the requirements for a platform.
type Platform interface {
	Name() string
	Configuration(context.Context) ([]byte, error)
	Hostname(context.Context) ([]byte, error)
	Mode() Mode
	ExternalIPs(context.Context) ([]net.IP, error)
	KernelArgs() procfs.Parameters
}
