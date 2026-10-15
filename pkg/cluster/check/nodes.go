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
	"github.com/siderolabs/gen/maps"
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
//
//nolint:gocyclo
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

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	respCh := multiplex.Unary(
		ctx, nodesIP,
		func(ctx context.Context) (*machineapi.MemoryResponse, error) {
			return cl.Memory(ctx)
		},
	)

	nodeToType := getNodesTypes(cluster, machine.TypeInit, machine.TypeControlPlane, machine.TypeWorker)

	errorsByNode := map[string]error{}

	for resp := range respCh {
		hostname := resp.Node

		if resp.Err != nil {
			errorsByNode[hostname] = fmt.Errorf("error getting memory info for node %q: %w", hostname, resp.Err)

			continue
		}

		typ, ok := nodeToType[hostname]
		if !ok {
			return fmt.Errorf("unexpected node %q in response", hostname)
		}

		minimum, _, err := minimal.Memory(typ)
		if err != nil {
			errorsByNode[hostname] = fmt.Errorf("error getting minimal memory for node %q: %w", hostname, err)

			continue
		}

		if len(resp.Payload.GetMessages()) == 0 {
			errorsByNode[hostname] = fmt.Errorf("no memory info returned for node %q", hostname)

			continue
		}

		msg := resp.Payload.GetMessages()[0]

		if totalMemory := msg.Meminfo.Memtotal * humanize.KiByte; totalMemory < minimum {
			errorsByNode[hostname] = fmt.Errorf(
				"node %q does not meet memory requirements: expected at least %d MiB, actual %d MiB",
				hostname,
				minimum/humanize.MiByte,
				totalMemory/humanize.MiByte,
			)
		}
	}

	if len(errorsByNode) == 0 {
		return nil
	}

	errorNodes := maps.Keys(errorsByNode)
	slices.Sort(errorNodes)

	var resultErr *multierror.Error

	for _, node := range errorNodes {
		resultErr = multierror.Append(resultErr, errorsByNode[node])
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

	// calculate EPHEMERAL by subtracting the size of all other partitions and GPT overhead
	ps := quirks.New("").PartitionSizes()
	minimalEphemeralSize := minimal.DiskSize() - (ps.UKIEFISize() + ps.StateSize() + ps.METASize() + 10*1048576 /* GPT overhead, including alignment */)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	respCh := multiplex.Unary(
		ctx, nodesIP,
		func(ctx context.Context) (*block.VolumeStatus, error) {
			return safe.StateGetByID[*block.VolumeStatus](ctx, cl.COSI, constants.EphemeralPartitionLabel)
		},
	)

	errorsByNode := map[string]error{}

	for resp := range respCh {
		if resp.Err != nil {
			if client.StatusCode(resp.Err) == codes.PermissionDenied {
				// old Talos versions don't support this resource
				return conditions.ErrSkipAssertion
			}

			errorsByNode[resp.Node] = fmt.Errorf("error getting volume status for node %q: %w", resp.Node, resp.Err)

			continue
		}

		actualSize := resp.Payload.TypedSpec().Size

		if actualSize < minimalEphemeralSize {
			errorsByNode[resp.Node] = fmt.Errorf(
				"ephemeral partition %q for node %q is too small, expected at least %s, actual %s",
				resp.Payload.TypedSpec().Location,
				resp.Node,
				humanize.IBytes(minimalEphemeralSize),
				humanize.IBytes(actualSize),
			)
		}
	}

	if len(errorsByNode) == 0 {
		return nil
	}

	errorNodes := maps.Keys(errorsByNode)
	slices.Sort(errorNodes)

	var resultErr *multierror.Error

	for _, node := range errorNodes {
		resultErr = multierror.Append(resultErr, errorsByNode[node])
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
