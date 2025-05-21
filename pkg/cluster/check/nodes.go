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
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
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
		client.WithNodes(
			ctx,
			mapIPsToStrings(mapNodeInfosToInternalIPs(cluster.Nodes()))...,
		),
		cl,
	)
	if err != nil {
		return err
	}

	if len(nodesIP) == 0 {
		return nil
	}

	resp, err := cl.Memory(client.WithNodes(ctx, nodesIP...))
	if err != nil {
		return fmt.Errorf("error getting nodes memory: %w", err)
	}

	var resultErr error

	nodeToType := getNodesTypes(cluster, machine.TypeInit, machine.TypeControlPlane, machine.TypeWorker)

	for _, msg := range resp.Messages {
		if msg.Metadata == nil {
			return errors.New("no metadata in the response")
		}

		hostname := msg.Metadata.Hostname

		typ, ok := nodeToType[hostname]
		if !ok {
			return fmt.Errorf("unexpected node %q in response", hostname)
		}

		minimum, _, err := minimal.Memory(typ)
		if err != nil {
			resultErr = multierror.Append(resultErr, err)

			continue
		}

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
		client.WithNodes(
			ctx,
			mapIPsToStrings(mapNodeInfosToInternalIPs(cluster.Nodes()))...,
		),
		cl,
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
		adjustment := uint64(1400 * humanize.MiByte) // adjust for system stuff
		minimalSize := minimal.DiskSize() - adjustment

		if actualSize < minimalSize {
			resultErr = multierror.Append(resultErr, fmt.Errorf(
				"ephemeral partition %q for node %q is too small, expected at least %s, actual %s",
				vs.TypedSpec().Location,
				nodeIP,
				humanize.IBytes(minimalSize),
				humanize.IBytes(actualSize),
			))
		}
	}

	return resultErr
}

func getNonContainerNodes(ctx context.Context, cl *client.Client) ([]string, error) {
	resp, err := cl.Version(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting version: %w", err)
	}

	result := make([]string, 0, len(resp.Messages))

	for _, msg := range resp.Messages {
		if msg.Metadata == nil {
			return nil, errors.New("got empty metadata")
		}

		if msg.Platform.Mode == "container" {
			continue
		}

		result = append(result, msg.Metadata.Hostname)
	}

	return result, nil
}
