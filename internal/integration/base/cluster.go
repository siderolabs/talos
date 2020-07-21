// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration

package base

import "github.com/talos-systems/talos/internal/app/machined/pkg/runtime"

type infoWrapper struct {
	masterNodes []string
	workerNodes []string
}

func (wrapper *infoWrapper) Nodes() []string {
	return append(wrapper.masterNodes, wrapper.workerNodes...)
}

func (wrapper *infoWrapper) NodesByType(t runtime.MachineType) []string {
	switch t {
	case runtime.MachineTypeInit:
		return nil
	case runtime.MachineTypeControlPlane:
		return wrapper.masterNodes
	case runtime.MachineTypeJoin:
		return wrapper.workerNodes
	default:
		panic("unreachable")
	}
}
