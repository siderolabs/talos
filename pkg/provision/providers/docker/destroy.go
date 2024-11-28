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

// Destroy Talos cluster as set of Docker nodes.
//
// Only cluster.Info().ClusterName and cluster.Info().Network.Name is being used.
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

	if options.SaveClusterLogsArchivePath != "" {
		fmt.Fprintf(options.LogWriter, "saving cluster logs archive to %s\n", options.SaveClusterLogsArchivePath)

		p.saveContainerLogs(ctx, cluster, options.SaveClusterLogsArchivePath)
	}

	if options.SaveSupportArchivePath != "" {
		fmt.Fprintf(options.LogWriter, "saving support archive to %s\n", options.SaveSupportArchivePath)

		cl.Crashdump(ctx, cluster, options.LogWriter, options.SaveSupportArchivePath)
	}

	if err := p.destroyNodes(ctx, cluster.Info().ClusterName, &options); err != nil {
		return err
	}

	fmt.Fprintln(os.Stderr, "destroying network", cluster.Info().Network.Name)

	if err := p.destroyNetwork(ctx, cluster.Info().Network.Name); err != nil {
		return err
	}

	return deleteStateDirectory(stateDirectoryPath, true)
}

func (p *provisioner) saveContainerLogs(ctx context.Context, cluster provision.Cluster, logsArchivePath string) {
	containers, err := p.listNodes(ctx, cluster.Info().ClusterName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error listing containers: %s\n", err)

		return
	}

	statePath, err := cluster.StatePath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error getting state path: %s\n", err)

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
			fmt.Fprintf(os.Stderr, "error querying container logs: %s\n", err)

			continue
		}

		logPath := filepath.Join(statePath, fmt.Sprintf("%s.log", name))

		var logData bytes.Buffer

		if _, err := io.Copy(&logData, logs); err != nil {
			fmt.Fprintf(os.Stderr, "error reading container logs: %s\n", err)

			continue
		}

		if err := os.WriteFile(logPath, logData.Bytes(), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "error writing container logs: %s\n", err)

			continue
		}
	}

	cl.SaveClusterLogsArchive(statePath, logsArchivePath)
}
