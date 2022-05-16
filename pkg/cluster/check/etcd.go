// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package check

import (
	"context"
	"fmt"

	machineapi "github.com/talos-systems/talos/pkg/machinery/api/machine"
)

// EtcdConsistentAssertion checks that etcd membership is consistent across nodes.
//nolint:gocyclo
func EtcdConsistentAssertion(ctx context.Context, cluster ClusterInfo) error {
	cli, err := cluster.Client()
	if err != nil {
		return err
	}

	resp, err := cli.EtcdMemberList(ctx, &machineapi.EtcdMemberListRequest{})
	if err != nil {
		return err
	}

	type data struct {
		hostname string
		id       uint64
	}

	knownMembers := map[data]struct{}{}

	for i, message := range resp.GetMessages() {
		// TODO(DmitriyMV): should we check if we got no messages?
		if i == 0 {
			// Fill data using first message
			for _, member := range message.Members {
				// TODO(DmitriyMV): should we check if we got no members?
				knownMembers[data{member.Hostname, member.Id}] = struct{}{}
			}

			continue
		}

		node := message.Metadata.GetHostname()
		expectedMembers := len(knownMembers)
		actualMembers := len(message.Members)

		if actualMembers != expectedMembers {
			return fmt.Errorf("%s: expected to have %d members, got %d", node, expectedMembers, actualMembers)
		}

		// check that member list is the same on all nodes
		for _, member := range message.Members {
			if _, found := knownMembers[data{member.Hostname, member.Id}]; !found {
				return fmt.Errorf("%s: found extra etcd member %s", node, member.Hostname)
			}
		}
	}

	return nil
}
