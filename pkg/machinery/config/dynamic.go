// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import (
	"context"
	"net"
)

// DynamicConfigProvider provides additional configuration which is overlaid on top of existing configuration.
type DynamicConfigProvider interface {
	Hostname(context.Context) ([]byte, error)
	ExternalIPs(context.Context) ([]net.IP, error)
}
