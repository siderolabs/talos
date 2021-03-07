// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration

package base

import "github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"

type infoWrapper struct {
	masterNodes []string
	workerNodes []string
}

func (wrapper *infoWrapper) Nodes() []string {
	return append([]string(nil), append(wrapper.masterNodes, wrapper.workerNodes...)...)
}

func (wrapper *infoWrapper) NodesByType(t machine.Type) []string {
	switch t {
	case machine.TypeInit:
		return nil
	case machine.TypeControlPlane:
		return append([]string(nil), wrapper.masterNodes...)
	case machine.TypeJoin:
		return append([]string(nil), wrapper.workerNodes...)
	case machine.TypeUnknown:
		fallthrough
	default:
		panic("unreachable")
	}
}
