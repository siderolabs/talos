// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package check

import (
	"context"
	"errors"
	fmt "fmt"
	"slices"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/dustin/go-humanize"
	"github.com/hashicorp/go-multierror"
	"google.golang.org/grpc/codes"

	"github.com/siderolabs/talos/pkg/conditions"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/client/multiplex"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/minimal"
)

// AllNodesMemorySizes checks that all nodes have enough memory.
func AllNodesMemorySizes(ctx context.Context, cluster ClusterInfo) error {
	cl, err := cluster.Client()
	if err != nil {
		return fmt.Errorf("error getting client: %w", err)
	}

	nodesIP, err := getNonContainerNodes(
		ctx,
		cl,
		mapIPsToStrings(mapNodeInfosToInternalIPs(cluster.Nodes())),
	)
	if err != nil {
		return err
	}

	if len(nodesIP) == 0 {
		return nil
	}

	respCh := multiplex.Unary(
		ctx, nodesIP,
		func(ctx context.Context) (*machineapi.MemoryResponse, error) {
			return cl.Memory(ctx)
		},
	)

	var resultErr error

	nodeToType := getNodesTypes(cluster, machine.TypeInit, machine.TypeControlPlane, machine.TypeWorker)

	for resp := range respCh {
		hostname := resp.Node

		if resp.Err != nil {
			resultErr = multierror.Append(resultErr, fmt.Errorf("error getting memory info for node %q: %w", hostname, resp.Err))

			continue
		}

		typ, ok := nodeToType[hostname]
		if !ok {
			return fmt.Errorf("unexpected node %q in response", hostname)
		}

		minimum, _, err := minimal.Memory(typ)
		if err != nil {
			resultErr = multierror.Append(resultErr, err)

			continue
		}

		msg := resp.Payload.GetMessages()[0]

		if totalMemory := msg.Meminfo.Memtotal * humanize.KiByte; totalMemory < minimum {
			resultErr = multierror.Append(
				resultErr,
				fmt.Errorf(
					"node %q does not meet memory requirements: expected at least %d MiB, actual %d MiB",
					hostname,
					minimum/humanize.MiByte,
					totalMemory/humanize.MiByte,
				),
			)
		}
	}

	return resultErr
}

func getNodesTypes(cluster ClusterInfo, nodeTypes ...machine.Type) map[string]machine.Type {
	result := map[string]machine.Type{}

	for _, typ := range nodeTypes {
		for _, node := range cluster.NodesByType(typ) {
			result[node.InternalIP.String()] = typ
		}
	}

	return result
}

// AllNodesDiskSizes checks that all nodes have enough disk space.
//
//nolint:gocyclo
func AllNodesDiskSizes(ctx context.Context, cluster ClusterInfo) error {
	cl, err := cluster.Client()
	if err != nil {
		return fmt.Errorf("error getting client: %w", err)
	}

	nodesIP, err := getNonContainerNodes(
		ctx,
		cl,
		mapIPsToStrings(mapNodeInfosToInternalIPs(cluster.Nodes())),
	)
	if err != nil {
		return err
	}

	if len(nodesIP) == 0 {
		return nil
	}

	var resultErr error

	slices.Sort(nodesIP)

	for _, nodeIP := range nodesIP {
		vs, err := safe.StateGetByID[*block.VolumeStatus](client.WithNode(ctx, nodeIP), cl.COSI, constants.EphemeralPartitionLabel)
		if err != nil {
			if client.StatusCode(err) == codes.PermissionDenied {
				// old Talos versions don't support this resource
				return conditions.ErrSkipAssertion
			}

			resultErr = multierror.Append(resultErr, fmt.Errorf("error getting volume status for node %q: %w", nodeIP, err))

			continue
		}

		actualSize := vs.TypedSpec().Size

		// calculate EPHEMERAL by subtracting the size of all other partitions and GPT overhead
		ps := quirks.New("").PartitionSizes()
		minimalEphemeralSize := minimal.DiskSize() - (ps.UKIEFISize() + ps.StateSize() + ps.METASize() + 10*1048576 /* GPT overhead, including alignment */)

		if actualSize < minimalEphemeralSize {
			resultErr = multierror.Append(resultErr, fmt.Errorf(
				"ephemeral partition %q for node %q is too small, expected at least %s, actual %s",
				vs.TypedSpec().Location,
				nodeIP,
				humanize.IBytes(minimalEphemeralSize),
				humanize.IBytes(actualSize),
			))
		}
	}

	return resultErr
}

func getNonContainerNodes(ctx context.Context, cl *client.Client, nodes []string) ([]string, error) {
	respCh := multiplex.Unary(
		ctx, nodes,
		func(ctx context.Context) (*machineapi.VersionResponse, error) {
			return cl.Version(ctx)
		},
	)

	var errs error

	result := make([]string, 0, len(nodes))

	for resp := range respCh {
		if resp.Err != nil {
			errs = errors.Join(errs, fmt.Errorf("error getting version from node %q: %w", resp.Node, resp.Err))

			continue
		}

		if resp.Payload.GetMessages()[0].GetPlatform().GetMode() == "container" {
			continue
		}

		result = append(result, resp.Node)
	}

	return result, errs
}
