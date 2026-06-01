// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package system

import (
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/conditions"
)

func NewServices(runtime runtime.Runtime) *singleton { //nolint:revive
	return newServices(runtime)
}

func WaitForServiceWithInstance(instance *singleton, event StateEvent, service string) conditions.Condition {
	return waitForService(instance, event, service)
}
