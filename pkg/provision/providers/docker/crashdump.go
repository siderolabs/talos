// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package docker

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/docker/docker/api/types"

	"github.com/talos-systems/talos/pkg/provision"
)

// CrashDump produces debug information to help with debugging failures.
func (p *provisioner) CrashDump(ctx context.Context, cluster provision.Cluster, out io.Writer) {
	containers, err := p.listNodes(ctx, cluster.Info().ClusterName)
	if err != nil {
		fmt.Fprintf(out, "error listing containers: %s\n", err)

		return
	}

	for _, container := range containers {
		name := container.Names[0][1:]
		fmt.Fprintf(out, "%s\n%s\n\n", name, strings.Repeat("=", len(name)))

		logs, err := p.client.ContainerLogs(ctx, container.ID, types.ContainerLogsOptions{
			ShowStdout: true,
			ShowStderr: true,
			Tail:       "1000",
		})
		if err != nil {
			fmt.Fprintf(out, "error querying container logs: %s\n", err)

			continue
		}

		_, _ = io.Copy(out, logs) //nolint:errcheck
	}
}
