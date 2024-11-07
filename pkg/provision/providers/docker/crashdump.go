// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package docker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/docker/docker/api/types/container"

	cl "github.com/siderolabs/talos/pkg/cluster"
	"github.com/siderolabs/talos/pkg/provision"
)

// CrashDump produces debug information to help with debugging failures.
func (p *provisioner) CrashDump(ctx context.Context, cluster provision.Cluster, logWriter io.Writer) {
	containers, err := p.listNodes(ctx, cluster.Info().ClusterName)
	if err != nil {
		fmt.Fprintf(logWriter, "error listing containers: %s\n", err)

		return
	}

	statePath, err := cluster.StatePath()
	if err != nil {
		fmt.Fprintf(logWriter, "error getting state path: %s\n", err)

		return
	}

	for _, ctr := range containers {
		name := ctr.Names[0][1:]

		logs, err := p.client.ContainerLogs(ctx, ctr.ID, container.LogsOptions{
			ShowStdout: true,
			ShowStderr: true,
			Tail:       "1000",
		})
		if err != nil {
			fmt.Fprintf(logWriter, "error querying container logs: %s\n", err)

			continue
		}

		logPath := filepath.Join(statePath, fmt.Sprintf("%s.log", name))

		var logData bytes.Buffer

		if _, err := io.Copy(&logData, logs); err != nil {
			fmt.Fprintf(logWriter, "error reading container logs: %s\n", err)

			continue
		}

		if err := os.WriteFile(logPath, logData.Bytes(), 0o644); err != nil {
			fmt.Fprintf(logWriter, "error writing container logs: %s\n", err)

			continue
		}
	}

	supportZipPath := filepath.Join(statePath, "support.zip")

	cl.Crashdump(ctx, cluster, logWriter, supportZipPath)
}
