// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build linux || darwin

package mgmt

import (
	"context"
	"path/filepath"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/logtail"
	remoteprovisionpb "github.com/siderolabs/talos/pkg/provision/api"
	"github.com/siderolabs/talos/pkg/provision/providers/qemu"
)

// logMachinesPollInterval is how often StreamLogs retries Reflect when
// the cluster doesn't exist yet and the client requested follow.
const logMachinesPollInterval = time.Second

// StreamLogs tails QEMU console logs from the server's state directory —
// the remote equivalent of `tail -F <machine>.log`. With an empty
// machine_name it fans in every machine in the cluster.
func (s *remoteProvisionImpl) StreamLogs(req *remoteprovisionpb.LogsRequest, stream grpc.ServerStreamingServer[remoteprovisionpb.LogData]) error {
	ctx := stream.Context()

	machines, err := s.logMachines(ctx, req.GetClusterName(), req.GetMachineName(), req.GetFollow())
	if err != nil {
		return err
	}

	// Each per-machine tailer feeds this channel; the main loop below is
	// the single writer to the gRPC stream (Send must not be concurrent).
	lines := make(chan *remoteprovisionpb.LogData, 256)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup

	for _, machine := range machines {
		wg.Add(1)

		go func(machine string) {
			defer wg.Done()

			tailLogFile(ctx,
				filepath.Join(s.stateDir, req.GetClusterName(), machine+".log"),
				machine, req.GetFollow(), lines)
		}(machine)
	}

	go func() {
		wg.Wait()
		close(lines)
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case data, ok := <-lines:
			if !ok {
				return nil
			}

			if err := stream.Send(data); err != nil {
				return err
			}
		}
	}
}

// logMachines resolves the machine list to tail: the single requested
// machine, or — when machineName is empty — every node in the cluster.
//
// In follow mode the lookup is retried on NotFound so the client can
// `logs -f` a cluster that hasn't been created yet (the tail -F
// equivalent of "wait for the file to appear" applied to the cluster
// state itself).
//
//nolint:gocyclo
func (s *remoteProvisionImpl) logMachines(ctx context.Context, clusterName, machineName string, follow bool) ([]string, error) {
	if machineName != "" {
		return []string{machineName}, nil
	}

	provisioner, err := qemu.NewProvisioner(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "qemu provisioner init: %v", err)
	}

	defer provisioner.Close() //nolint:errcheck

	for {
		cluster, reflectErr := provisioner.Reflect(ctx, clusterName, s.stateDir)
		if reflectErr == nil {
			var machines []string

			for _, node := range cluster.Info().Nodes {
				machines = append(machines, node.Name)
			}

			if len(machines) > 0 {
				return machines, nil
			}

			if !follow {
				return nil, status.Errorf(codes.NotFound, "cluster %q has no machines", clusterName)
			}
		} else if !follow {
			return nil, status.Errorf(codes.NotFound, "reflect cluster %q: %v", clusterName, reflectErr)
		}

		select {
		case <-ctx.Done():
			return nil, status.FromContextError(ctx.Err()).Err()
		case <-time.After(logMachinesPollInterval):
		}
	}
}

// tailLogFile streams complete lines of a console log file into out,
// tagged with the machine name, via the shared logtail helper (tail -F).
func tailLogFile(ctx context.Context, path, machine string, follow bool, out chan<- *remoteprovisionpb.LogData) {
	logtail.Tail(ctx, path, follow, func(line []byte) bool {
		select {
		case out <- &remoteprovisionpb.LogData{MachineName: machine, Line: string(line)}:
			return true
		case <-ctx.Done():
			return false
		}
	})
}
