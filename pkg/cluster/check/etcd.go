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

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	machineapi "github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/client"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/machinery/constants"
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
	nodeCtx := client.WithNodes(ctx, controlPlaneNodes[0])

	resp, err := cli.EtcdMemberList(nodeCtx, &machineapi.EtcdMemberListRequest{})
	if err != nil {
		return err
	}

	messages := resp.GetMessages()
	if len(messages) != 1 {
		return fmt.Errorf("unexpected number of messages: %d", len(messages))
	}

	message := messages[0]
	members := message.GetMembers()

	memberIPKeyMap := make(map[string]struct{})

	for _, member := range members {
		for _, peerURL := range member.GetPeerUrls() {
			parsed, err2 := url.Parse(peerURL)
			if err2 != nil {
				return err2
			}

			ip := parsed.Hostname()
			memberIPKeyMap[ip] = struct{}{}
		}
	}

	controlPlaneKeyMap := toKeyMap(controlPlaneNodes)

	discoveryControlPlaneNodes, err := getDiscoveryControlPlaneNodeIPs(ctx, cluster)
	if err != nil {
		return err
	}

	discoveryControlPlaneNodeMap := toKeyMap(discoveryControlPlaneNodes)

	if !mapsAreEqual(memberIPKeyMap, controlPlaneKeyMap) ||
		!mapsAreEqual(memberIPKeyMap, discoveryControlPlaneNodeMap) {
		return errors.New("mismatch between etcd member and control plane nodes")
	}

	return nil
}

func getDiscoveryControlPlaneNodeIPs(ctx context.Context, cluster ClusterInfo) ([]string, error) {
	k8sClient, err := cluster.K8sClient(ctx)
	if err != nil {
		return nil, err
	}

	discoveryNodes, err := k8sClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	controlPlaneNodes := sliceFilter(discoveryNodes.Items, isControlPlaneNode)

	ips := sliceCollect(controlPlaneNodes, func(node v1.Node) string {
		return node.Status.Addresses[0].Address
	})

	return ips, nil
}

func isControlPlaneNode(node *v1.Node) bool {
	if _, ok := node.GetLabels()[constants.LabelNodeRoleControlPlane]; ok {
		return true
	}

	if _, ok := node.GetLabels()[constants.LabelNodeRoleMaster]; ok {
		return true
	}

	return false
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

func sliceFilter[S ~[]T, T any](input S, keepFn func(*T) bool) S {
	var result []T

	for _, val := range input {
		if keepFn(&val) {
			result = append(result, val)
		}
	}

	return result
}

func toKeyMap[K comparable](input []K) map[K]struct{} {
	m := make(map[K]struct{})

	for _, val := range input {
		m[val] = struct{}{}
	}

	return m
}

func mapsAreEqual[M ~map[K]V, K comparable, V comparable](m1, m2 M) bool {
	if len(m1) != len(m2) {
		return false
	}

	for k, val1 := range m1 {
		if val2, ok := m2[k]; !ok || val1 != val2 {
			return false
		}
	}

	return true
}
