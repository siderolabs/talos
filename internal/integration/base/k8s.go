// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration_k8s

package base

import (
	"context"
	"time"

	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// K8sSuite is a base suite for K8s tests.
type K8sSuite struct {
	APISuite

	Clientset       *kubernetes.Clientset
	DiscoveryClient *discovery.DiscoveryClient
}

// SetupSuite initializes Kubernetes client.
func (k8sSuite *K8sSuite) SetupSuite() {
	k8sSuite.APISuite.SetupSuite()

	kubeconfig, err := k8sSuite.Client.Kubeconfig(context.Background())
	k8sSuite.Require().NoError(err)

	config, err := clientcmd.BuildConfigFromKubeconfigGetter("", func() (*clientcmdapi.Config, error) {
		return clientcmd.Load(kubeconfig)
	})
	k8sSuite.Require().NoError(err)

	// patch timeout
	config.Timeout = time.Minute
	if k8sSuite.K8sEndpoint != "" {
		config.Host = k8sSuite.K8sEndpoint
	}

	k8sSuite.Clientset, err = kubernetes.NewForConfig(config)
	k8sSuite.Require().NoError(err)

	k8sSuite.DiscoveryClient, err = discovery.NewDiscoveryClientForConfig(config)
	k8sSuite.Require().NoError(err)
}
