// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package kubeclient provides a reusable way to create a Kubernetes clientset
// from the admin kubeconfig fetched via the Talos API, without writing to the filesystem.
package kubeclient

import (
	"context"
	"fmt"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/siderolabs/talos/pkg/machinery/client"
)

// FromTalosClient fetches the admin kubeconfig from the Talos API and returns
// a Kubernetes clientset. The kubeconfig is held entirely in memory.
func FromTalosClient(ctx context.Context, c *client.Client) (kubernetes.Interface, error) {
	kubeconfigBytes, err := c.Kubeconfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("error fetching kubeconfig from Talos API: %w", err)
	}

	config, err := clientcmd.NewClientConfigFromBytes(kubeconfigBytes)
	if err != nil {
		return nil, fmt.Errorf("error parsing kubeconfig: %w", err)
	}

	restConfig, err := config.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("error building REST config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("error creating Kubernetes clientset: %w", err)
	}

	return clientset, nil
}
