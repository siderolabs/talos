// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import (
	"context"
	"fmt"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// KubernetesClient provides Kubernetes client built via Talos API Kubeconfig.
type KubernetesClient struct {
	// Base Talos client provider.
	ClientProvider

	// ForceEndpoint overrides default Kubernetes API endpoint.
	ForceEndpoint string

	clientset *kubernetes.Clientset
}

// K8sClient builds Kubernetes client via Talos Kubeconfig API.
//
// Kubernetes client instance is cached.
func (k *KubernetesClient) K8sClient(ctx context.Context) (*kubernetes.Clientset, error) {
	if k.clientset != nil {
		return k.clientset, nil
	}

	client, err := k.Client()
	if err != nil {
		return nil, err
	}

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
	config.Timeout = time.Minute

	if k.ForceEndpoint != "" {
		config.Host = fmt.Sprintf("%s:%d", k.ForceEndpoint, 6443)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err == nil {
		k.clientset = clientset
	}

	return clientset, err
}
