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

	machineapi "github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/client"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
)

// EtcdConsistentAssertion checks that etcd membership is consistent across nodes.
//nolint:gocyclo
func EtcdConsistentAssertion(ctx context.Context, cluster ClusterInfo) error {
	cli, err := cluster.Client()
	if err != nil {
		return err
	}

	nodes := append(cluster.NodesByType(machine.TypeInit), cluster.NodesByType(machine.TypeControlPlane)...)
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
			expected := mapCollect(knownMembers, func(k data, v struct{}) string { return k.hostname })
			actual := sliceCollect(message.Members, func(v *machineapi.EtcdMember) string { return v.GetHostname() })

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
func EtcdControlPlaneNodesAssertion(ctx context.Context, cluster ClusterInfo) error {
	cli, err := cluster.Client()
	if err != nil {
		return err
	}

	controlPlaneNodes := append(cluster.NodesByType(machine.TypeInit), cluster.NodesByType(machine.TypeControlPlane)...)

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

	controlPlaneNodeIPs := mapIPsToStrings(flatMapNodeInfosToIPs(controlPlaneNodes))
	if !isSubset(controlPlaneNodeIPs, memberIPs) {
		return errors.New("mismatch between etcd member and control plane nodes")
	}

	return nil
}

func mapCollect[M ~map[K]V, Z any, K comparable, V any](m M, fn func(K, V) Z) []Z {
	r := make([]Z, 0, len(m))
	for k, v := range m {
		r = append(r, fn(k, v))
	}

	return r
}

func sliceCollect[S ~[]V, V any, R any](slc S, fn func(V) R) []R {
	r := make([]R, 0, len(slc))
	for _, v := range slc {
		r = append(r, fn(v))
	}

	return r
}

func isSubset[S ~[]V, V comparable](set S, subset S) bool {
	inputMap := toKeyMap(set)
	for _, v := range subset {
		if _, found := inputMap[v]; !found {
			return false
		}
	}

	return true
}

func toKeyMap[S ~[]V, V comparable](input S) map[V]struct{} {
	m := make(map[V]struct{}, len(input))

	for _, val := range input {
		m[val] = struct{}{}
	}

	return m
}
