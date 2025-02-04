// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package check

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/channel"
	"github.com/siderolabs/gen/maps"
	"github.com/siderolabs/gen/xslices"

	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// AllNodesBootedAssertion checks whether nodes reached end of 'Boot' sequence.
//
//nolint:gocyclo
func AllNodesBootedAssertion(ctx context.Context, cluster ClusterInfo) error {
	cli, err := cluster.Client()
	if err != nil {
		return err
	}

	nodes := cluster.Nodes()
	nodeInternalIPs := mapIPsToStrings(mapNodeInfosToInternalIPs(nodes))

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	type eventWithNode struct {
		node  string
		event state.Event
	}

	eventCh := make(chan eventWithNode)

	for _, nodeIP := range nodeInternalIPs {
		nodeEventCh := make(chan state.Event)

		if err = cli.COSI.Watch(client.WithNode(ctx, nodeIP), runtime.NewMachineStatus().Metadata(), nodeEventCh); err != nil {
			return err
		}

		go func(nodeIP string) {
			for {
				select {
				case <-ctx.Done():
					return
				case ev := <-nodeEventCh:
					channel.SendWithContext(ctx, eventCh, eventWithNode{node: nodeIP, event: ev})
				}
			}
		}(nodeIP)
	}

	nodeStages := make(map[string]runtime.MachineStage, len(nodeInternalIPs))

	for _, nodeIP := range nodeInternalIPs {
		nodeStages[nodeIP] = runtime.MachineStageUnknown
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ev := <-eventCh:
			switch ev.event.Type {
			case state.Created, state.Updated:
				machineStatus, ok := ev.event.Resource.(*runtime.MachineStatus)
				if !ok {
					return fmt.Errorf("unexpected resource type: %T", ev.event.Resource)
				}

				nodeStages[ev.node] = machineStatus.TypedSpec().Stage
			case state.Destroyed, state.Bootstrapped, state.Noop:
				// nothing
			case state.Errored:
				return fmt.Errorf("error watching machine %s status: %w", ev.node, ev.event.Error)
			}
		}

		allNodesRunning := true
		allNodesReported := true
		stageWithNodes := map[runtime.MachineStage][]string{}

		for nodeIP, stage := range nodeStages {
			if stage != runtime.MachineStageRunning {
				allNodesRunning = false
			}

			if stage == runtime.MachineStageUnknown {
				allNodesReported = false
			}

			stageWithNodes[stage] = append(stageWithNodes[stage], nodeIP)
		}

		if !allNodesReported {
			// keep waiting for data from all nodes
			continue
		}

		if allNodesRunning {
			return nil
		}

		// if we're here, not all nodes are running
		delete(stageWithNodes, runtime.MachineStageRunning)

		stages := maps.Keys(stageWithNodes)
		slices.Sort(stages)

		message := xslices.Map(stages, func(stage runtime.MachineStage) string {
			nodeIPs := stageWithNodes[stage]
			slices.Sort(nodeIPs)

			return fmt.Sprintf("%s: %v", stage, nodeIPs)
		})

		return fmt.Errorf("nodes are not running: %s", strings.Join(message, ", "))
	}
}
