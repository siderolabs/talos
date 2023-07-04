// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package maintenance

import "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"

var runtimeController runtime.Controller

// InjectController is used to pass the controller into the maintenance service.
func InjectController(c runtime.Controller) {
	runtimeController = c
}
