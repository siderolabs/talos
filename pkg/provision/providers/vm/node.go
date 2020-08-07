// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vm

import (
	"fmt"

	multierror "github.com/hashicorp/go-multierror"

	"github.com/talos-systems/talos/pkg/provision"
)

// DestroyNodes destroys all VMs.
func (p *Provisioner) DestroyNodes(cluster provision.ClusterInfo, options *provision.Options) error {
	errCh := make(chan error)

	nodes := append(cluster.Nodes, cluster.ExtraNodes...)

	for _, node := range nodes {
		go func(node provision.NodeInfo) {
			fmt.Fprintln(options.LogWriter, "stopping VM", node.Name)

			errCh <- p.DestroyNode(node)
		}(node)
	}

	var multiErr *multierror.Error

	for range nodes {
		multiErr = multierror.Append(multiErr, <-errCh)
	}

	return multiErr.ErrorOrNil()
}

// DestroyNode destroys VM.
func (p *Provisioner) DestroyNode(node provision.NodeInfo) error {
	return stopProcessByPidfile(node.ID) // node.ID stores PID path for control process
}
