// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration
// +build integration

package base

import (
	"github.com/talos-systems/talos/pkg/cluster"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
)

type infoWrapper struct {
	masterNodes []string
	workerNodes []string

	nodeInfos       []cluster.NodeInfo
	nodeInfosByType map[machine.Type][]cluster.NodeInfo
}

func (wrapper *infoWrapper) Nodes() []cluster.NodeInfo {
	return wrapper.nodeInfos
}

func (wrapper *infoWrapper) NodesByType(t machine.Type) []cluster.NodeInfo {
	return wrapper.nodeInfosByType[t]
}
