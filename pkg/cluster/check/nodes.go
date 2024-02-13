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
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/dustin/go-humanize"
	"github.com/hashicorp/go-multierror"
	"google.golang.org/grpc/codes"

	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
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

	ctx = client.WithNodes(ctx, nodesIP...)

	nodesMounts, err := getNodesMounts(ctx, cl)
	if err != nil {
		return err
	}

	var resultErr error

	for _, nodeIP := range nodesIP {
		data, err := getEphemeralPartitionData(ctx, cl.COSI, nodeIP)
		if errors.Is(err, ErrOldTalosVersion) {
			continue
		} else if err != nil {
			resultErr = multierror.Append(resultErr, err)

			continue
		}

		nodeMounts, ok := nodesMounts[nodeIP]
		if !ok {
			resultErr = multierror.Append(resultErr, fmt.Errorf("node %q not found in mounts", nodeIP))

			continue
		}

		idx := slices.IndexFunc(nodeMounts, func(mnt mntData) bool { return mnt.Filesystem == data.Source })
		if idx == -1 {
			resultErr = multierror.Append(resultErr, fmt.Errorf("ephemeral partition %q not found for node %q", data.Source, nodeIP))

			continue
		}

		minimalDiskSize := minimal.DiskSize()

		// adjust by 1400 MiB to account for the size of system stuff
		if actualDiskSize := nodeMounts[idx].Size + 1400*humanize.MiByte; actualDiskSize < minimal.DiskSize() {
			resultErr = multierror.Append(resultErr, fmt.Errorf(
				"ephemeral partition %q for node %q is too small, expected at least %s, actual %s",
				data.Source,
				nodeIP,
				humanize.IBytes(minimalDiskSize),
				humanize.IBytes(actualDiskSize),
			))

			continue
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

type mountData struct {
	Source string
}

// ErrOldTalosVersion is returned when the node is running an old version of Talos.
var ErrOldTalosVersion = errors.New("old Talos version")

func getEphemeralPartitionData(ctx context.Context, state state.State, nodeIP string) (mountData, error) {
	items, err := safe.StateListAll[*runtime.MountStatus](client.WithNode(ctx, nodeIP), state)
	if err != nil {
		if client.StatusCode(err) == codes.Unimplemented {
			// old version of Talos without COSI API
			return mountData{}, ErrOldTalosVersion
		}

		return mountData{}, fmt.Errorf("error listing mounts for node %q: %w", nodeIP, err)
	}

	for it := items.Iterator(); it.Next(); {
		mount := it.Value()
		mountID := mount.Metadata().ID()

		if mountID == constants.EphemeralPartitionLabel {
			return mountData{
				Source: mount.TypedSpec().Source,
			}, nil
		}
	}

	return mountData{}, fmt.Errorf("no ephemeral partition found for node '%s'", nodeIP)
}

type mntData struct {
	Filesystem string
	Size       uint64
}

func getNodesMounts(ctx context.Context, cl *client.Client) (map[string][]mntData, error) {
	diskResp, err := cl.Mounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting nodes mounts: %w", err)
	}

	if len(diskResp.Messages) == 0 {
		return nil, errors.New("no nodes with mounts found")
	}

	nodesMnts := map[string][]mntData{}

	for _, msg := range diskResp.Messages {
		switch {
		case msg.Metadata == nil:
			return nil, errors.New("no metadata in response")
		case len(msg.GetStats()) == 0:
			return nil, fmt.Errorf("no mounts found for node %q", msg.Metadata.Hostname)
		}

		hostname := msg.Metadata.Hostname

		for _, mnt := range msg.GetStats() {
			nodesMnts[hostname] = append(nodesMnts[hostname], mntData{
				Filesystem: mnt.Filesystem,
				Size:       mnt.Size,
			})
		}
	}

	return nodesMnts, nil
}
