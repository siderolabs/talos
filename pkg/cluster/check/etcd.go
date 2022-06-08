// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package check

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sort"

	"github.com/talos-systems/talos/pkg/cluster"
	machineapi "github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/client"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/machinery/generic/maps"
	"github.com/talos-systems/talos/pkg/machinery/generic/slices"
)

// EtcdConsistentAssertion checks that etcd membership is consistent across nodes.
//nolint:gocyclo
func EtcdConsistentAssertion(ctx context.Context, cl ClusterInfo) error {
	cli, err := cl.Client()
	if err != nil {
		return err
	}

	var nodes []cluster.NodeInfo

	initNodes := cl.NodesByType(machine.TypeInit)
	nodes = append(nodes, initNodes...)
	controlPlaneNodes := cl.NodesByType(machine.TypeControlPlane)

	nodes = append(nodes, controlPlaneNodes...)

	nodesCtx := client.WithNodes(ctx, mapIPsToStrings(mapNodeInfosToInternalIPs(nodes))...)

	resp, err := cli.EtcdMemberList(nodesCtx, &machineapi.EtcdMemberListRequest{})
	if err != nil {
		return err
	}

	type data struct {
		hostname  string
		id        uint64
		isLearner bool
	}

	knownMembers := map[data]struct{}{}

	messages := resp.GetMessages()
	if len(messages) == 0 {
		return errors.New("no messages returned")
	}

	sort.Slice(messages, func(i, j int) bool {
		return messages[i].GetMetadata().GetHostname() < messages[j].GetMetadata().GetHostname()
	})

	for i, message := range messages {
		if i == 0 {
			// Fill data using first message
			for _, member := range message.Members {
				knownMembers[data{member.Hostname, member.Id, member.IsLearner}] = struct{}{}
			}

			continue
		}

		node := message.Metadata.GetHostname()

		if len(message.Members) != len(knownMembers) {
			expected := maps.ToSlice(knownMembers, func(k data, v struct{}) string { return k.hostname })
			actual := slices.Map(message.Members, (*machineapi.EtcdMember).GetHostname)

			return fmt.Errorf("%s: expected to have %v members, got %v", node, expected, actual)
		}

		// check that member list is the same on all nodes
		for _, member := range message.Members {
			if _, found := knownMembers[data{member.Hostname, member.Id, member.IsLearner}]; !found {
				return fmt.Errorf("%s: found unexpected etcd member %s", node, member.Hostname)
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
	if !maps.Contains(slices.ToSet(controlPlaneNodeIPs), memberIPs) {
		return errors.New("mismatch between etcd member and control plane nodes")
	}

	return nil
}
