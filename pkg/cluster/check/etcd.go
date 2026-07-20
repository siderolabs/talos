// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package check

import (
	"cmp"
	"context"
	"fmt"
	"net/url"
	"slices"

	"github.com/siderolabs/gen/maps"
	"github.com/siderolabs/gen/xslices"

	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client/multiplex"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
)

// EtcdConsistentAssertion checks that etcd membership is consistent across nodes.
//
//nolint:gocyclo
func EtcdConsistentAssertion(ctx context.Context, cl ClusterInfo) error {
	cli, err := cl.Client()
	if err != nil {
		return err
	}

	initNodes := cl.NodesByType(machine.TypeInit)
	controlPlaneNodes := cl.NodesByType(machine.TypeControlPlane)
	nodes := slices.Concat(initNodes, controlPlaneNodes)

	respCh := multiplex.Unary(
		ctx, mapIPsToStrings(mapNodeInfosToInternalIPs(nodes)),
		func(ctx context.Context) (*machineapi.EtcdMemberListResponse, error) {
			return cli.EtcdMemberList(ctx, &machineapi.EtcdMemberListRequest{})
		},
	)

	type memberResponse struct {
		*machineapi.EtcdMembers

		node string
	}

	memberResponses := make([]memberResponse, 0, len(nodes))

	for resp := range respCh {
		if resp.Err != nil {
			return fmt.Errorf("error getting etcd member list from node %q: %w", resp.Node, resp.Err)
		}

		if len(resp.Payload.GetMessages()) == 0 {
			return fmt.Errorf("node %q: no messages returned", resp.Node)
		}

		memberResponses = append(memberResponses, memberResponse{node: resp.Node, EtcdMembers: resp.Payload.GetMessages()[0]})
	}

	slices.SortFunc(memberResponses, func(a, b memberResponse) int {
		return cmp.Compare(a.node, b.node)
	})

	// members are identified by their etcd member ID, which is stable across reboots;
	// the hostname (member name) is not used for identification, as a member can
	// temporarily report an empty name right after a reboot, before it re-learns
	// peer names via a raft proposal, even though cluster membership is otherwise consistent.
	type data struct {
		isLearner bool
	}

	knownMembers := map[uint64]data{}

	for i, message := range memberResponses {
		if i == 0 {
			// Fill data using first message
			for _, member := range message.Members {
				knownMembers[member.Id] = data{isLearner: member.IsLearner}
			}

			continue
		}

		if len(message.Members) != len(knownMembers) {
			expected := maps.ToSlice(knownMembers, func(id uint64, v data) string { return fmt.Sprintf("%016x", id) })
			actual := xslices.Map(message.Members, func(m *machineapi.EtcdMember) string { return fmt.Sprintf("%016x", m.GetId()) })

			return fmt.Errorf("%s: expected to have members %v, got %v", message.node, expected, actual)
		}

		// check that member list is the same on all nodes
		for _, member := range message.Members {
			known, found := knownMembers[member.Id]
			if !found {
				return fmt.Errorf("%s: found unexpected etcd member %016x (hostname %q)", message.node, member.Id, member.Hostname)
			}

			if known.isLearner != member.IsLearner {
				return fmt.Errorf("%s: etcd member %016x learner status mismatch: expected %v, got %v", message.node, member.Id, known.isLearner, member.IsLearner)
			}
		}
	}

	return nil
}

// EtcdControlPlaneNodesAssertion checks that etcd nodes are control plane nodes.
func EtcdControlPlaneNodesAssertion(ctx context.Context, cl ClusterInfo) error {
	cli, err := cl.Client()
	if err != nil {
		return err
	}

	nodes := append(cl.NodesByType(machine.TypeInit), cl.NodesByType(machine.TypeControlPlane)...)

	resp, err := cli.EtcdMemberList(ctx, &machineapi.EtcdMemberListRequest{})
	if err != nil {
		return err
	}

	members := resp.GetMessages()[0].GetMembers()

	var memberIPs []string

	for _, member := range members {
		for _, peerURL := range member.GetPeerUrls() {
			parsed, err2 := url.Parse(peerURL)
			if err2 != nil {
				return err2
			}

			memberIP := parsed.Hostname()
			memberIPs = append(memberIPs, memberIP)
		}
	}

	controlPlaneNodeIPs := mapIPsToStrings(flatMapNodeInfosToIPs(nodes))
	if !maps.Contains(xslices.ToSet(controlPlaneNodeIPs), memberIPs) {
		return fmt.Errorf("etcd member ips %q are not subset of control plane node ips %q",
			memberIPs, controlPlaneNodeIPs)
	}

	return nil
}
