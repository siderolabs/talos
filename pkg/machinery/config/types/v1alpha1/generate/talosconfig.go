// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package generate

import (
	clientconfig "github.com/siderolabs/talos/pkg/machinery/client/config"
)

// Talosconfig returns the talos admin Talos config.
//
// Deprecated: use github.com/siderolabs/talos/pkg/machinery/config/generate.Talosconfig instead.
func Talosconfig(in *Input, opts ...GenOption) (*clientconfig.Config, error) {
	return in.Talosconfig()
}
