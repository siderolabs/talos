// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package client

import (
	"github.com/talos-systems/talos/pkg/machinery/client/resolver"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

var talosListResolverScheme string

func init() {
	talosListResolverScheme = resolver.RegisterRoundRobinResolver(constants.ApidPort)
}
