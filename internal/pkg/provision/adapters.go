// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package provision

import (
	"context"

	"k8s.io/client-go/kubernetes"

	"github.com/talos-systems/talos/cmd/osctl/pkg/client"
)

// ClusterAccess extends Cluster interface to provide clients to the cluster.
type ClusterAccess interface {
	Cluster

	// Client returns default Talos client.
	Client(endpoints ...string) (*client.Client, error)

	// K8sClient returns Kubernetes client.
	K8sClient(context.Context) (*kubernetes.Clientset, error)

	// Close shuts down all the clients.
	Close() error
}
