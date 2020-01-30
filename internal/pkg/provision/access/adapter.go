// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package access

import (
	"context"
	"fmt"
	"strings"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/talos-systems/talos/cmd/osctl/pkg/client"
	"github.com/talos-systems/talos/internal/pkg/provision"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/grpc/tls"
)

// NewAdapter returns ClusterAccess object from Cluster.
func NewAdapter(cluster provision.Cluster, opts ...provision.Option) provision.ClusterAccess {
	options := provision.DefaultOptions()

	for _, opt := range opts {
		if err := opt(&options); err != nil {
			panic(err)
		}
	}

	adapter := &adapter{
		Cluster: cluster,
		clients: make(map[string]*client.Client),
		options: &options,
	}

	if options.TalosClient != nil {
		// inject default client if provided
		adapter.clients[""] = options.TalosClient
	}

	return adapter
}

type adapter struct {
	provision.Cluster

	clients   map[string]*client.Client
	clientset *kubernetes.Clientset
	options   *provision.Options
}

func (a *adapter) Client(endpoints ...string) (*client.Client, error) {
	key := strings.Join(endpoints, ",")

	if cli := a.clients[key]; cli != nil {
		return cli, nil
	}

	configContext, creds, err := client.NewClientContextAndCredentialsFromParsedConfig(a.options.TalosConfig, "")
	if err != nil {
		return nil, err
	}

	if len(endpoints) == 0 {
		endpoints = configContext.Endpoints
	}

	tlsconfig, err := tls.New(
		tls.WithKeypair(creds.Crt),
		tls.WithClientAuthType(tls.Mutual),
		tls.WithCACertPEM(creds.CA),
	)
	if err != nil {
		return nil, err
	}

	client, err := client.NewClient(tlsconfig, endpoints, constants.ApidPort)
	if err == nil {
		a.clients[key] = client
	}

	return client, err
}

func (a *adapter) K8sClient(ctx context.Context) (*kubernetes.Clientset, error) {
	if a.clientset != nil {
		return a.clientset, nil
	}

	client, err := a.Client()
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

	if a.options.ForceEndpoint != "" {
		config.Host = fmt.Sprintf("%s:%d", a.options.ForceEndpoint, 6443)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err == nil {
		a.clientset = clientset
	}

	return clientset, err
}

func (a *adapter) Close() error {
	for _, cli := range a.clients {
		if cli == a.options.TalosClient {
			// this client was provided, don't close it
			continue
		}

		if err := cli.Close(); err != nil {
			return err
		}
	}

	a.clients = nil
	a.clientset = nil

	return nil
}
