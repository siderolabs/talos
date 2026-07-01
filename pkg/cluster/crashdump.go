// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-talos-support/support"
	"github.com/siderolabs/go-talos-support/support/bundle"
	"github.com/siderolabs/go-talos-support/support/collectors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/provision"
)

// Crashdump creates a support.zip for the cluster.
func Crashdump(ctx context.Context, cluster provision.Cluster, logWriter io.Writer, zipFilePath string) {
	supportFile, err := os.Create(zipFilePath)
	if err != nil {
		fmt.Fprintf(logWriter, "error creating crashdump file: %s\n", err)

		return
	}

	defer supportFile.Close() //nolint:errcheck

	// limit support bundle generation time
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	c, err := client.New(ctx, client.WithDefaultConfig())
	if err != nil {
		fmt.Fprintf(logWriter, "error creating crashdump: %s\n", err)

		return
	}

	nodes := xslices.Map(cluster.Info().Nodes, func(nodeInfo provision.NodeInfo) string {
		return nodeInfo.IPs[0].String()
	})

	opts := []bundle.Option{
		bundle.WithArchiveOutput(supportFile),
		bundle.WithTalosClientProvider(func(ctx context.Context, node string) (context.Context, *client.Client, error) {
			return client.WithNode(ctx, node), c, nil
		}),
		bundle.WithNodes(nodes...),
		bundle.WithNumWorkers(4),
		bundle.WithLogOutput(io.Discard),
	}

	kubeclient, err := getKubernetesClient(ctx, c)
	// ignore error if we can't get a k8s client
	if err == nil {
		opts = append(opts, bundle.WithKubernetesClient(kubeclient))
	}

	options := bundle.NewOptions(opts...)

	collectors, err := collectors.GetForOptions(ctx, options)
	if err != nil {
		if len(collectors) == 0 {
			fmt.Fprintf(logWriter, "error creating crashdump collector options: %s\n", err)

			return
		}

		fmt.Fprintf(logWriter, "warning: error (ignored) creating crashdump collector options: %s\n", err)
	}

	if err := support.CreateSupportBundle(ctx, options, collectors...); err != nil {
		fmt.Fprintf(logWriter, "error creating crashdump: %s\n", err)

		return
	}
}

func getKubernetesClient(ctx context.Context, c *client.Client) (*k8s.Clientset, error) {
	kubeconfig, err := c.Kubeconfig(ctx)
	if err != nil {
		return nil, err
	}

	config, err := clientcmd.NewClientConfigFromBytes(kubeconfig)
	if err != nil {
		return nil, err
	}

	restconfig, err := config.ClientConfig()
	if err != nil {
		return nil, err
	}

	clientset, err := k8s.NewForConfig(restconfig)
	if err != nil {
		return nil, err
	}

	// just checking that k8s responds
	_, err = clientset.CoreV1().Namespaces().Get(ctx, "kube-system", v1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return clientset, nil
}
