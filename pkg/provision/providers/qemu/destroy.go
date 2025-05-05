// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package qemu

import (
	"context"
	"fmt"
	"os"

	cl "github.com/siderolabs/talos/pkg/cluster"
	"github.com/siderolabs/talos/pkg/provision"
	"github.com/siderolabs/talos/pkg/provision/providers/vm"
)

// Destroy Talos cluster as set of qemu VMs.
//
//nolint:gocyclo
func (p *provisioner) Destroy(ctx context.Context, cluster provision.Cluster, opts ...provision.Option) error {
	options := provision.DefaultOptions()

	for _, opt := range opts {
		if err := opt(&options); err != nil {
			return err
		}
	}

	stateDirectoryPath, err := cluster.StatePath()
	if err != nil {
		return err
	}

	complete := false
	deleteStateDirectory := func(stateDir string, shouldDelete bool) error {
		if complete || !shouldDelete {
			return nil
		}

		complete = true

		return os.RemoveAll(stateDir)
	}

	defer deleteStateDirectory(stateDirectoryPath, options.DeleteStateOnErr) //nolint:errcheck

	if options.SaveSupportArchivePath != "" {
		fmt.Fprintf(options.LogWriter, "saving support archive to %s\n", options.SaveSupportArchivePath)

		cl.Crashdump(ctx, cluster, options.LogWriter, options.SaveSupportArchivePath)
	}

	fmt.Fprintln(options.LogWriter, "stopping VMs")

	if err := p.DestroyNodes(cluster.Info(), &options); err != nil {
		return err
	}

	if err := p.destroyVirtualTPMs(cluster.Info()); err != nil {
		return err
	}

	state, ok := cluster.(*vm.State)
	if !ok {
		return fmt.Errorf("error inspecting QEMU state, %#+v", cluster)
	}

	fmt.Fprintln(options.LogWriter, "removing dhcpd")

	if err := p.DestroyDHCPd(state); err != nil {
		return fmt.Errorf("error stopping dhcpd: %w", err)
	}

	fmt.Fprintln(options.LogWriter, "removing load balancer")

	if err := p.DestroyLoadBalancer(state); err != nil {
		return fmt.Errorf("error stopping loadbalancer: %w", err)
	}

	fmt.Fprintln(options.LogWriter, "removing kms")

	if err := p.DestroyKMS(state); err != nil {
		return err
	}

	fmt.Fprintln(options.LogWriter, "removing network")

	if err := p.DestroyNetwork(state); err != nil {
		return err
	}

	fmt.Fprintln(options.LogWriter, "removing siderolink agent")

	if err := p.DestroySiderolinkAgent(state); err != nil {
		return err
	}

	fmt.Fprintln(options.LogWriter, "removing state directory")

	if options.SaveClusterLogsArchivePath != "" {
		fmt.Fprintf(options.LogWriter, "saving cluster logs archive to %s\n", options.SaveClusterLogsArchivePath)

		cl.SaveClusterLogsArchive(stateDirectoryPath, options.SaveClusterLogsArchivePath)
	}

	fmt.Fprintln(options.LogWriter, "removing json logs")

	if err := p.DestroyJSONLogs(state); err != nil {
		return err
	}

	return deleteStateDirectory(stateDirectoryPath, true)
}
