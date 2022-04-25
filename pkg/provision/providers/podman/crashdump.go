// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package podman

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/containers/podman/v4/pkg/bindings/containers"

	"github.com/talos-systems/talos/pkg/provision"
)

// CrashDump produces debug information to help with debugging failures.
func (p *provisioner) CrashDump(ctx context.Context, cluster provision.Cluster, out io.Writer) {
	nodes, err := p.listNodes(ctx, cluster.Info().ClusterName)
	if err != nil {
		fmt.Fprintf(out, "error listing containers: %s\n", err)
		return
	}

	for _, node := range nodes {
		name := node.Names[0][1:]
		fmt.Fprintf(out, "%s\n%s\n\n", name, strings.Repeat("=", len(name)))

		stderr := make(chan string)
		stdout := make(chan string)
		err = containers.Logs(p.connection, name, &containers.LogOptions{
			Stdout: &[]bool{true}[0],
			Stderr: &[]bool{true}[0],
			Tail:   &[]string{"1000"}[0],
		}, stdout, stderr)
		if err != nil {
			fmt.Fprintf(out, "error querying container logs: %s\n", err)

			continue
		}

		_, _ = io.WriteString(out, <-stdout)
		_, _ = io.WriteString(out, <-stderr)
	}
}
