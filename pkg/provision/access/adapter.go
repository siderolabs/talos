// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package access

import (
	"github.com/siderolabs/talos/pkg/cluster"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/provision"
)

// Adapter provides cluster access via provision.Cluster.
type Adapter struct {
	cluster.ConfigClientProvider
	cluster.KubernetesClient
	cluster.APICrashDumper
	cluster.APIBootstrapper
	cluster.Info
	cluster.ApplyConfigClient
}

type infoWrapper struct {
	clusterInfo provision.ClusterInfo
	nodes       []cluster.NodeInfo
	nodesByType map[machine.Type][]cluster.NodeInfo
}

func (wrapper *infoWrapper) Nodes() []cluster.NodeInfo {
	return wrapper.nodes
}

func (wrapper *infoWrapper) NodesByType(t machine.Type) []cluster.NodeInfo {
	return wrapper.nodesByType[t]
}

// NewAdapter returns ClusterAccess object from Cluster.
func NewAdapter(clusterInfo provision.Cluster, opts ...provision.Option) *Adapter {
	options := provision.DefaultOptions()

	for _, opt := range opts {
		if err := opt(&options); err != nil {
			panic(err)
		}
	}

	c := clusterInfo.Info()

	nodeInfos, err := cluster.MapProvisionNodeInfosToClusterNodeInfos(c.Nodes)
	if err != nil {
		panic(err)
	}

	nodeInfosByType, err := cluster.MapProvisionNodeInfosToNodeInfosByType(c.Nodes)
	if err != nil {
		panic(err)
	}

	infoW := &infoWrapper{
		clusterInfo: c,
		nodes:       nodeInfos,
		nodesByType: nodeInfosByType,
	}

	configProvider := cluster.ConfigClientProvider{
		DefaultClient: options.TalosClient,
		TalosConfig:   options.TalosConfig,
	}

	return &Adapter{
		ConfigClientProvider: configProvider,
		KubernetesClient: cluster.KubernetesClient{
			ClientProvider: &configProvider,
			ForceEndpoint:  options.KubernetesEndpoint,
		},
		APICrashDumper: cluster.APICrashDumper{
			ClientProvider: &configProvider,
			Info:           infoW,
		},
		APIBootstrapper: cluster.APIBootstrapper{
			ClientProvider: &configProvider,
			Info:           infoW,
		},
		Info: infoW,
	}
}
