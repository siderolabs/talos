// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package access

import (
	"net/netip"

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

func (wrapper *infoWrapper) Nodes() ([]cluster.NodeInfo, error) {
	nodes := make([]cluster.NodeInfo, len(wrapper.clusterInfo.Nodes))

	for i := range nodes {
		node := wrapper.clusterInfo.Nodes[i]

		nodeInfo, err := toNodeInfo(node)
		if err != nil {
			return nil, err
		}

		nodes[i] = *nodeInfo
	}

	return nodes, nil
}

func (wrapper *infoWrapper) NodesByType(t machine.Type) ([]cluster.NodeInfo, error) {
	var nodes []cluster.NodeInfo

	for _, node := range wrapper.clusterInfo.Nodes {
		if node.Type == t {
			nodeInfo, err := toNodeInfo(node)
			if err != nil {
				return nil, err
			}

			nodes = append(nodes, *nodeInfo)
		}
	}

	return nodes, nil
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

func toNodeInfo(info provision.NodeInfo) (*cluster.NodeInfo, error) {
	internalIP, err := netip.ParseAddr(info.IPs[0].String())
	if err != nil {
		return nil, err
	}

	ips := make([]netip.Addr, len(info.IPs))

	for i, ip := range info.IPs {
		addr, err2 := netip.ParseAddr(ip.String())
		if err2 != nil {
			return nil, err2
		}

		ips[i] = addr
	}

	return &cluster.NodeInfo{
		InternalIP: internalIP,
		IPs:        ips,
	}, nil
}
