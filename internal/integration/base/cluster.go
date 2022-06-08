// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration
// +build integration

package base

import (
	"fmt"

	"github.com/talos-systems/talos/pkg/cluster"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
)

type infoWrapper struct {
	masterNodes []string
	workerNodes []string
}

func (wrapper *infoWrapper) Nodes() ([]cluster.NodeInfo, error) {
	return cluster.IPsToNodeInfos(append(wrapper.masterNodes, wrapper.workerNodes...))
}

func (wrapper *infoWrapper) NodesByType(t machine.Type) ([]cluster.NodeInfo, error) {
	switch t {
	case machine.TypeInit:
		return nil, nil
	case machine.TypeControlPlane:
		return cluster.IPsToNodeInfos(wrapper.masterNodes)
	case machine.TypeWorker:
		return cluster.IPsToNodeInfos(wrapper.workerNodes)
	case machine.TypeUnknown:
		fallthrough
	default:
		panic(fmt.Sprintf("unexpected machine type %v", t))
	}
}
