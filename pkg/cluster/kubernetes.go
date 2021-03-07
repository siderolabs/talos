// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import (
	"context"
	"fmt"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	k8s "github.com/talos-systems/talos/pkg/kubernetes"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// KubernetesClient provides Kubernetes client built via Talos API Kubeconfig.
type KubernetesClient struct {
	// Base Talos client provider.
	ClientProvider

	// ForceEndpoint overrides default Kubernetes API endpoint.
	ForceEndpoint string

	KubeHelper *k8s.Client

	kubeconfig []byte
	clientset  *kubernetes.Clientset
}

// Kubeconfig returns raw kubeconfig.
//
// Kubeconfig is cached.
func (k *KubernetesClient) Kubeconfig(ctx context.Context) ([]byte, error) {
	if k.kubeconfig != nil {
		return k.kubeconfig, nil
	}

	client, err := k.Client()
	if err != nil {
		return nil, err
	}

	k.kubeconfig, err = client.Kubeconfig(ctx)

	return k.kubeconfig, err
}

// K8sRestConfig returns *rest.Config (parsed kubeconfig).
func (k *KubernetesClient) K8sRestConfig(ctx context.Context) (*rest.Config, error) {
	kubeconfig, err := k.Kubeconfig(ctx)
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
		config.Host = fmt.Sprintf("%s:%d", k.ForceEndpoint, constants.DefaultControlPlanePort)
	}

	return config, nil
}

// K8sClient builds Kubernetes client via Talos Kubeconfig API.
//
// Kubernetes client instance is cached.
func (k *KubernetesClient) K8sClient(ctx context.Context) (*kubernetes.Clientset, error) {
	if k.clientset != nil {
		return k.clientset, nil
	}

	config, err := k.K8sRestConfig(ctx)
	if err != nil {
		return nil, err
	}

	if k.KubeHelper, err = k8s.NewForConfig(config); err != nil {
		return nil, err
	}

	k.clientset = k.KubeHelper.Clientset

	return k.clientset, nil
}

// K8sHelper returns wrapper around K8sClient.
func (k *KubernetesClient) K8sHelper(ctx context.Context) (*k8s.Client, error) {
	if k.KubeHelper != nil {
		return k.KubeHelper, nil
	}

	_, err := k.K8sClient(ctx)
	if err != nil {
		return nil, err
	}

	return k.KubeHelper, nil
}
