// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_k8s
// +build integration_k8s

package base

import (
	"context"
	"fmt"
	"time"

	"github.com/talos-systems/go-retry/retry"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// GetK8sNodeByInternalIP returns the kubernetes node by its internal ip or error if it is not found.
func (k8sSuite *K8sSuite) GetK8sNodeByInternalIP(ctx context.Context, internalIP string) (*corev1.Node, error) {
	nodeList, err := k8sSuite.Clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, item := range nodeList.Items {
		for _, address := range item.Status.Addresses {
			if address.Type == corev1.NodeInternalIP {
				if address.Address == internalIP {
					return &item, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("node with internal IP %s not found", internalIP)
}

// WaitForK8sNodeReadinessStatus waits for node to have the given status.
// It retries until the node with the name is found and matches the expected condition.
func (k8sSuite *K8sSuite) WaitForK8sNodeReadinessStatus(ctx context.Context, nodeName string, checkFn func(corev1.ConditionStatus) bool) error {
	return retry.Constant(5 * time.Minute).Retry(func() error {
		readinessStatus, err := k8sSuite.GetK8sNodeReadinessStatus(ctx, nodeName)
		if errors.IsNotFound(err) {
			return retry.ExpectedError(err)
		}

		if err != nil {
			return err
		}

		if !checkFn(readinessStatus) {
			return retry.ExpectedError(fmt.Errorf("node readiness status is %s", readinessStatus))
		}

		return nil
	})
}

// GetK8sNodeReadinessStatus returns the node readiness status of the node.
func (k8sSuite *K8sSuite) GetK8sNodeReadinessStatus(ctx context.Context, nodeName string) (corev1.ConditionStatus, error) {
	node, err := k8sSuite.Clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady {
			return condition.Status, nil
		}
	}

	return "", fmt.Errorf("node %s has no readiness condition", nodeName)
}
