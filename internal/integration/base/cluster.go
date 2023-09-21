// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration

package base

import (
	"slices"

	"github.com/siderolabs/talos/pkg/cluster"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
)

type infoWrapper struct {
	nodeInfos       []cluster.NodeInfo
	nodeInfosByType map[machine.Type][]cluster.NodeInfo
}

func newNodeInfo(masterNodes, workerNodes []string) (*infoWrapper, error) {
	controlPlaneNodeInfos, err := cluster.IPsToNodeInfos(masterNodes)
	if err != nil {
		return nil, err
	}

	workerNodeInfos, err := cluster.IPsToNodeInfos(workerNodes)
	if err != nil {
		return nil, err
	}

	return &infoWrapper{
		nodeInfos: append(slices.Clone(controlPlaneNodeInfos), workerNodeInfos...),
		nodeInfosByType: map[machine.Type][]cluster.NodeInfo{
			machine.TypeControlPlane: controlPlaneNodeInfos,
			machine.TypeWorker:       workerNodeInfos,
		},
	}, nil
}

func (wrapper *infoWrapper) Nodes() []cluster.NodeInfo {
	return wrapper.nodeInfos
}

func (wrapper *infoWrapper) NodesByType(t machine.Type) []cluster.NodeInfo {
	return wrapper.nodeInfosByType[t]
}
