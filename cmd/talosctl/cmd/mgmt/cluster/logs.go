// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/spf13/cobra"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/logtail"
	"github.com/siderolabs/talos/pkg/provision"
	"github.com/siderolabs/talos/pkg/provision/providers"
	"github.com/siderolabs/talos/pkg/provision/providers/remote"
)

var logsCmdFlags struct {
	follow bool
}

// logsCmd streams QEMU console logs for a cluster's machines.
var logsCmd = &cobra.Command{
	Use:   "logs [machine]",
	Short: "Stream QEMU console logs for cluster machines",
	Long: `Streams QEMU console logs (the per-machine <machine>.log files).

With no machine argument, every machine in the cluster is tailed, each line
prefixed with its machine name. Works against a local cluster or, with
--remote-endpoint, a remote-provision server.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		machine := ""
		if len(args) == 1 {
			machine = args[0]
		}

		if PersistentFlags.RemoteEndpoint != "" {
			return remoteClusterLogs(cmd.Context(), machine)
		}

		return localClusterLogs(cmd.Context(), machine)
	},
}

// remoteClusterLogs streams logs from a remote-provision server.
func remoteClusterLogs(ctx context.Context, machine string) error {
	provisioner, err := providers.Factory(ctx, providers.RemoteProviderName, providers.WithRemoteEndpoint(PersistentFlags.RemoteEndpoint))
	if err != nil {
		return err
	}

	defer provisioner.Close() //nolint:errcheck

	rp, ok := provisioner.(*remote.Provisioner)
	if !ok {
		return errors.New("remote provisioner expected")
	}

	return rp.StreamLogs(ctx, PersistentFlags.ClusterName, machine, logsCmdFlags.follow, os.Stdout)
}

// localClusterLogs tails the local state directory's console log files.
func localClusterLogs(ctx context.Context, machine string) error {
	machines := []string{machine}

	// A specific machine was requested — skip the per-line prefix.
	prefix := machine == ""

	if machine == "" {
		state, err := provision.ReadState(ctx, PersistentFlags.ClusterName, PersistentFlags.StateDir)
		if err != nil {
			return fmt.Errorf("failed to read cluster state: %w", err)
		}

		provisioner, err := providers.Factory(ctx, state.ProvisionerName)
		if err != nil {
			return err
		}

		defer provisioner.Close() //nolint:errcheck

		cluster, err := provisioner.Reflect(ctx, PersistentFlags.ClusterName, PersistentFlags.StateDir)
		if err != nil {
			return err
		}

		machines = nil

		for _, node := range cluster.Info().Nodes {
			machines = append(machines, node.Name)
		}
	}

	if len(machines) == 0 {
		return errors.New("no machines to tail")
	}

	var (
		wg sync.WaitGroup
		mu sync.Mutex
	)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	for _, m := range machines {
		wg.Add(1)

		go func(machine string) {
			defer wg.Done()

			path := filepath.Join(PersistentFlags.StateDir, PersistentFlags.ClusterName, machine+".log")
			tailLocalLog(ctx, path, machine, logsCmdFlags.follow, prefix, &mu)
		}(m)
	}

	wg.Wait()

	return nil
}

// tailLocalLog tails a single console log file to stdout via the shared
// logtail helper (tail -F). With prefix set, lines are tagged with the
// machine name. Writes are serialized through mu so concurrent tailers
// don't interleave mid-line.
func tailLocalLog(ctx context.Context, path, machine string, follow, prefix bool, mu *sync.Mutex) {
	logtail.Tail(ctx, path, follow, func(line []byte) bool {
		mu.Lock()

		if prefix {
			fmt.Fprintf(os.Stdout, "[%s] %s\n", machine, line)
		} else {
			fmt.Fprintf(os.Stdout, "%s\n", line)
		}

		mu.Unlock()

		return true
	})
}

func init() {
	logsCmd.Flags().BoolVarP(&logsCmdFlags.follow, "follow", "f", false, "keep streaming new output (tail -F)")
	Cmd.AddCommand(logsCmd)
}
