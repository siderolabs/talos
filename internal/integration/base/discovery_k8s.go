// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_k8s

package base

import (
	"context"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/siderolabs/talos/pkg/cluster"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

//nolint:gocyclo
func discoverNodesK8s(ctx context.Context, client *client.Client, suite *TalosSuite) (cluster.Info, error) {
	kubeconfig, err := client.Kubeconfig(ctx)
	if err != nil {
		return nil, err
	}

	config, err := clientcmd.BuildConfigFromKubeconfigGetter("", func() (*clientcmdapi.Config, error) {
		return clientcmd.Load(kubeconfig)
	})
	if err != nil {
		return nil, err
	}

	// patch timeout
	if suite.K8sEndpoint != "" {
		config.Host = suite.K8sEndpoint
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var masterNodes, workerNodes []string

	for _, node := range nodes.Items {
		var address string

		for _, nodeAddress := range node.Status.Addresses {
			if nodeAddress.Type == v1.NodeInternalIP {
				address = nodeAddress.Address

				break
			}
		}

		if address == "" {
			continue
		}

		if _, ok := node.Labels[constants.LabelNodeRoleControlPlane]; ok {
			masterNodes = append(masterNodes, address)
		} else {
			workerNodes = append(workerNodes, address)
		}
	}

	return newNodeInfo(masterNodes, workerNodes)
}
