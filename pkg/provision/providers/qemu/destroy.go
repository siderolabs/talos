// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package qemu

import (
	"context"
	"fmt"
	"os"

	"github.com/talos-systems/talos/pkg/provision"
	"github.com/talos-systems/talos/pkg/provision/providers/vm"
)

// Destroy Talos cluster as set of qemu VMs.
func (p *provisioner) Destroy(ctx context.Context, cluster provision.Cluster, opts ...provision.Option) error {
	options := provision.DefaultOptions()

	for _, opt := range opts {
		if err := opt(&options); err != nil {
			return err
		}
	}

	fmt.Fprintln(options.LogWriter, "stopping VMs")

	if err := p.DestroyNodes(cluster.Info(), &options); err != nil {
		return err
	}

	state, ok := cluster.(*vm.State)
	if !ok {
		return fmt.Errorf("error inspecting firecracker state, %#+v", cluster)
	}

	fmt.Fprintln(options.LogWriter, "removing dhcpd")

	if err := p.DestroyDHCPd(state); err != nil {
		return fmt.Errorf("error stopping dhcpd: %w", err)
	}

	fmt.Fprintln(options.LogWriter, "removing load balancer")

	if err := p.DestroyLoadBalancer(state); err != nil {
		return fmt.Errorf("error stopping loadbalancer: %w", err)
	}

	fmt.Fprintln(options.LogWriter, "removing network")

	if err := p.DestroyNetwork(state); err != nil {
		return err
	}

	fmt.Fprintln(options.LogWriter, "removing state directory")

	stateDirectoryPath, err := cluster.StatePath()
	if err != nil {
		return err
	}

	return os.RemoveAll(stateDirectoryPath)
}
