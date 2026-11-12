// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package etcd

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	clientv3 "go.etcd.io/etcd/client/v3"

	etcdresource "github.com/siderolabs/talos/pkg/machinery/resources/etcd"
)

// JoinResult describes the outcome of joining the etcd cluster as a learner.
type JoinResult struct {
	// InitialCluster is the value for the etcd --initial-cluster flag.
	InitialCluster string
	// LearnerMemberID is the ID this node was assigned as a learner.
	LearnerMemberID uint64
	// VotingMemberEndpoints are the etcd client URLs advertised by the voting members of the cluster.
	VotingMemberEndpoints []string
}

// JoinAsLearner adds this node to the etcd cluster as a learner.
//
// JoinAsLearner should only be called when the etcd data directory is empty, so the node is about to join the etcd cluster.
//
//nolint:gocyclo
func (c *Client) JoinAsLearner(ctx context.Context, name string, peerURLs []string) (JoinResult, error) {
	ctx = clientv3.WithRequireLeader(ctx)

	list, err := c.MemberList(ctx)
	if err != nil {
		return JoinResult{}, fmt.Errorf("error getting etcd member list: %w", err)
	}

	for _, member := range list.Members {
		// if there's already a member with same hostname, it should be removed, as there will be a conflict between the existing
		// member and a new joining member.
		// here we assume that control plane nodes have unique hostnames (if that's not the case, it will be a problem anyways)
		if member.Name == name {
			if _, err = c.MemberRemove(ctx, member.ID); err != nil {
				return JoinResult{}, fmt.Errorf("error removing self from the member list: %w", err)
			}
		}
	}

	var memberID uint64

	add, err := c.MemberAddAsLearner(ctx, peerURLs)

	switch {
	case err == nil:
		memberID = add.Member.ID

		list, err = c.MemberList(ctx)
		if err != nil {
			return JoinResult{}, fmt.Errorf("error getting second etcd member list: %w", err)
		}
	case errors.Is(err, rpctypes.ErrPeerURLExist):
		// member already exists with the same peer URLs, see if it's ourselves as a learner
		// we can't really say for sure, but we try to match the peer URLs, and name should
		// be still empty at this point
		idx := slices.IndexFunc(list.Members, func(member *etcdserverpb.Member) bool {
			return slices.Equal(member.PeerURLs, peerURLs) && member.IsLearner && member.Name == ""
		})
		if idx == -1 {
			return JoinResult{}, fmt.Errorf("member already exists with the same peer URLs %q, but is not a learner: %w", peerURLs, err)
		}

		memberID = list.Members[idx].ID
	default:
		return JoinResult{}, fmt.Errorf("error adding member: %w", err)
	}

	result := JoinResult{
		LearnerMemberID: memberID,
	}

	var conf []string

	for _, memb := range list.Members {
		for _, u := range memb.PeerURLs {
			n := memb.Name
			if memb.ID == memberID {
				n = name
			}

			conf = append(conf, fmt.Sprintf("%s=%s", n, u))
		}

		// a learner (including this node) can't serve the promotion call, so only voting members are of interest
		if memb.ID != memberID && !memb.IsLearner {
			result.VotingMemberEndpoints = append(result.VotingMemberEndpoints, memb.ClientURLs...)
		}
	}

	result.InitialCluster = strings.Join(conf, ",")

	return result, nil
}

// MemberState describes the state of a member in the etcd cluster.
type MemberState int

// MemberState values.
const (
	// MemberStateAbsent means the member is not part of the cluster.
	MemberStateAbsent MemberState = iota
	// MemberStateLearner means the member is a learner.
	MemberStateLearner
	// MemberStateVoting means the member is a voting member.
	MemberStateVoting
)

// GetMemberState looks up the member in the etcd cluster.
func (c *Client) GetMemberState(ctx context.Context, memberID uint64) (MemberState, error) {
	list, err := c.MemberList(clientv3.WithRequireLeader(ctx))
	if err != nil {
		return MemberStateAbsent, fmt.Errorf("error getting etcd member list: %w", err)
	}

	for _, member := range list.Members {
		if member.ID != memberID {
			continue
		}

		if member.IsLearner {
			return MemberStateLearner, nil
		}

		return MemberStateVoting, nil
	}

	return MemberStateAbsent, nil
}

// RemoveMemberIfExists removes the member from the etcd cluster, ignoring the member which is already gone.
func (c *Client) RemoveMemberIfExists(ctx context.Context, memberID uint64) error {
	err := c.RemoveMemberByMemberID(clientv3.WithRequireLeader(ctx), memberID)
	if err != nil && !errors.Is(err, rpctypes.ErrMemberNotFound) {
		return err
	}

	return nil
}

// promoteEndpointTimeout bounds a single promotion call against a single endpoint.
//
// Client.PromoteMember reports an endpoint which refuses connections almost immediately, so this
// timeout only bounds endpoints which accept the connection but never answer (e.g. a blackholed
// address or a wedged member).
const promoteEndpointTimeout = 5 * time.Second

// PromoteLearner promotes the learner to a voting member of the etcd cluster.
//
// The promotion has to be served by a voting member, so the endpoints are tried one by one until
// the promotion succeeds. The call fails while the learner is still catching up with the leader,
// so it should be retried.
//
// The member which is already a voting member is not an error.
func PromoteLearner(ctx context.Context, clientSpec *etcdresource.ClientSpecSpec, endpoints []string, memberID uint64) error {
	if len(endpoints) == 0 {
		return errors.New("no endpoints")
	}

	errs := make([]error, 0, len(endpoints))

	for _, endpoint := range endpoints {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		err := attemptPromote(ctx, clientSpec, endpoint, memberID)
		if err == nil || errors.Is(err, rpctypes.ErrMemberNotLearner) {
			return nil
		}

		errs = append(errs, fmt.Errorf("%s: %w", endpoint, err))
	}

	return errors.Join(errs...)
}

func attemptPromote(ctx context.Context, clientSpec *etcdresource.ClientSpecSpec, endpoint string, memberID uint64) error {
	ctx, cancel := context.WithTimeout(ctx, promoteEndpointTimeout)
	defer cancel()

	client, err := NewClientFromSpec(ctx, clientSpec, []string{endpoint})
	if err != nil {
		return err
	}

	defer client.Close() //nolint:errcheck

	return client.PromoteMember(ctx, memberID)
}

// PromotionEndpoints builds the ordered list of endpoints to try the promotion call against.
//
// The endpoints etcd advertises for the voting members come first, followed by the discovered
// control plane endpoints; endpoints belonging to this node are dropped, as a learner can't promote
// itself, and duplicates across the two sources are collapsed.
//
// votingMemberEndpoints are the client URLs etcd itself advertises for the voting members, captured
// when this node joined; they are tried first, as the discovered control plane endpoints are node
// addresses which might not be running etcd at all (e.g. the Kubernetes control plane endpoint, or a
// node address outside of the configured etcd listen subnets).
func PromotionEndpoints(selfEndpoints, votingMemberEndpoints, discoveredEndpoints []string) []string {
	total := len(votingMemberEndpoints) + len(discoveredEndpoints)

	endpoints := make([]string, 0, total)
	seen := make(map[string]struct{}, total+len(selfEndpoints))

	for _, endpoint := range selfEndpoints {
		seen[normalizeEndpoint(endpoint)] = struct{}{}
	}

	for _, endpoint := range slices.Concat(votingMemberEndpoints, discoveredEndpoints) {
		endpoint = normalizeEndpoint(endpoint)

		if _, ok := seen[endpoint]; ok {
			continue
		}

		seen[endpoint] = struct{}{}

		endpoints = append(endpoints, endpoint)
	}

	return endpoints
}

// normalizeEndpoint strips the scheme off an etcd endpoint, so that endpoints coming from
// different sources (etcd member client URLs vs. discovered control plane addresses) can be
// compared and de-duplicated.
func normalizeEndpoint(endpoint string) string {
	if _, hostPort, ok := strings.Cut(endpoint, "://"); ok {
		return hostPort
	}

	return endpoint
}
