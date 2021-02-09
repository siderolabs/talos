// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import (
	"context"

	k8s "k8s.io/client-go/kubernetes"

	"github.com/talos-systems/talos/pkg/kubernetes"
)

// KubernetesFromKubeletClient provides Kubernetes client built from local kubelet config.
type KubernetesFromKubeletClient struct {
	KubeHelper *kubernetes.Client
	clientset  *k8s.Clientset
}

// K8sClient builds Kubernetes client from local kubelet config.
//
// Kubernetes client instance is cached.
func (k *KubernetesFromKubeletClient) K8sClient(ctx context.Context) (*k8s.Clientset, error) {
	if k.clientset != nil {
		return k.clientset, nil
	}

	var err error
	if k.KubeHelper, err = kubernetes.NewClientFromKubeletKubeconfig(); err != nil {
		return nil, err
	}

	k.clientset = k.KubeHelper.Clientset

	return k.clientset, nil
}

// K8sHelper returns wrapper around K8sClient.
func (k *KubernetesFromKubeletClient) K8sHelper(ctx context.Context) (*kubernetes.Client, error) {
	if k.KubeHelper != nil {
		return k.KubeHelper, nil
	}

	_, err := k.K8sClient(ctx)
	if err != nil {
		return nil, err
	}

	return k.KubeHelper, nil
}
