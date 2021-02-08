// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package access

import (
	"github.com/talos-systems/talos/pkg/cluster"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/provision"
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
}

func (wrapper *infoWrapper) Nodes() []string {
	nodes := make([]string, len(wrapper.clusterInfo.Nodes))

	for i := range nodes {
		nodes[i] = wrapper.clusterInfo.Nodes[i].IPs[0].String()
	}

	return nodes
}

func (wrapper *infoWrapper) NodesByType(t machine.Type) []string {
	var nodes []string

	for _, node := range wrapper.clusterInfo.Nodes {
		if node.Type == t {
			nodes = append(nodes, node.IPs[0].String())
		}
	}

	return nodes
}

// NewAdapter returns ClusterAccess object from Cluster.
func NewAdapter(clusterInfo provision.Cluster, opts ...provision.Option) *Adapter {
	options := provision.DefaultOptions()

	for _, opt := range opts {
		if err := opt(&options); err != nil {
			panic(err)
		}
	}

	info := &infoWrapper{clusterInfo: clusterInfo.Info()}

	configProvider := cluster.ConfigClientProvider{
		DefaultClient: options.TalosClient,
		TalosConfig:   options.TalosConfig,
	}

	return &Adapter{
		ConfigClientProvider: configProvider,
		KubernetesClient: cluster.KubernetesClient{
			ClientProvider: &configProvider,
			ForceEndpoint:  options.ForceEndpoint,
		},
		APICrashDumper: cluster.APICrashDumper{
			ClientProvider: &configProvider,
			Info:           info,
		},
		APIBootstrapper: cluster.APIBootstrapper{
			ClientProvider: &configProvider,
			Info:           info,
		},
		Info: info,
	}
}
