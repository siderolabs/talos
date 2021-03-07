// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package check

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	machineapi "github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/client"
)

// AllNodesBootedAssertion checks whether nodes reached end of 'Boot' sequence.
//nolint:gocyclo
func AllNodesBootedAssertion(ctx context.Context, cluster ClusterInfo) error {
	cli, err := cluster.Client()
	if err != nil {
		return err
	}

	nodes := cluster.Nodes()

	ctx, cancel := context.WithCancel(ctx)
	nodesCtx := client.WithNodes(ctx, nodes...)

	nodesBootStarted := map[string]struct{}{}
	nodesBootStopped := map[string]struct{}{}

	err = cli.EventsWatch(nodesCtx, func(ch <-chan client.Event) {
		defer cancel()

		for event := range ch {
			if msg, ok := event.Payload.(*machineapi.SequenceEvent); ok {
				if msg.GetSequence() == "boot" { // can't use runtime constants as they're in `internal/`
					switch msg.GetAction() { //nolint:exhaustive
					case machineapi.SequenceEvent_START:
						nodesBootStarted[event.Node] = struct{}{}
					case machineapi.SequenceEvent_STOP:
						nodesBootStopped[event.Node] = struct{}{}
					}
				}
			}
		}
	}, client.WithTailEvents(-1))

	if err != nil {
		unwrappedErr := err

		for {
			if s, ok := status.FromError(unwrappedErr); ok && s.Code() == codes.DeadlineExceeded {
				// ignore deadline exceeded as we've just exhausted events list
				err = nil

				break
			}

			unwrappedErr = errors.Unwrap(unwrappedErr)
			if unwrappedErr == nil {
				break
			}
		}
	}

	if err != nil {
		return err
	}

	nodesNotFinishedBooting := []string{}

	// check for nodes which have Boot/Start event, but no Boot/Stop even
	// if the node is up long enough, Boot/Start even might get out of the window,
	// so we can't check such nodes reliably
	for node := range nodesBootStarted {
		if _, ok := nodesBootStopped[node]; !ok {
			nodesNotFinishedBooting = append(nodesNotFinishedBooting, node)
		}
	}

	sort.Strings(nodesNotFinishedBooting)

	if len(nodesNotFinishedBooting) > 0 {
		return fmt.Errorf("nodes %q are still in boot sequence", nodesNotFinishedBooting)
	}

	return nil
}
