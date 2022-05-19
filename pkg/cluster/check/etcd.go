// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package check

import (
	"context"
	"errors"
	"fmt"
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
	nodesCtx := client.WithNodes(ctx, nodes...)

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
		fmt.Println("hostname", message.GetMetadata().GetHostname())

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
