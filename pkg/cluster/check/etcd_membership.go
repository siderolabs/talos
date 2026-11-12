// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package check

import (
	"context"
	"fmt"
	"slices"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/hashicorp/go-multierror"
	"google.golang.org/grpc/codes"

	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/resources/etcd"
)

// EtcdHealthAssertion checks that etcd is a healthy member of the etcd cluster on all control plane nodes.
//
// The etcd service is only started once the machine is about to become a member of the etcd cluster,
// so the progress is reported from the etcd membership status first: the machine is waiting to join
// the cluster (or to be bootstrapped), it is catching up with the cluster as a learner, etc.
//
// Once all machines are members of the etcd cluster, the health of the etcd service is asserted,
// which verifies the etcd quorum.
//
// Talos versions which don't have the etcd membership status are only checked by the etcd service health.
func EtcdHealthAssertion(ctx context.Context, cl ClusterInfo) error {
	cli, err := cl.Client()
	if err != nil {
		return err
	}

	nodes := slices.Concat(cl.NodesByType(machine.TypeInit), cl.NodesByType(machine.TypeControlPlane))

	var multiErr *multierror.Error

	for _, node := range mapIPsToStrings(mapNodeInfosToInternalIPs(nodes)) {
		status, err := safe.StateGetByID[*etcd.MembershipStatus](client.WithNode(ctx, node), cli.State(), etcd.MembershipStatusID)

		switch {
		case err == nil:
			if problem := EtcdMembershipProblem(status.TypedSpec()); problem != "" {
				multiErr = multierror.Append(multiErr, fmt.Errorf("%s: %s", node, problem))
			}
		case client.StatusCode(err) == codes.PermissionDenied:
			// the resource is not known to this version of Talos, it is covered by the service health check below
		case state.IsNotFoundError(err):
			multiErr = multierror.Append(multiErr, fmt.Errorf("%s: etcd membership status is not available yet", node))
		default:
			return fmt.Errorf("error getting etcd membership status from node %q: %w", node, err)
		}
	}

	if err = multiErr.ErrorOrNil(); err != nil {
		return err
	}

	return ServiceHealthAssertion(ctx, cl, "etcd", WithNodeTypes(machine.TypeInit, machine.TypeControlPlane))
}

// EtcdMembershipProblem describes why the machine is not a member of the etcd cluster (yet).
//
// It returns an empty string if the machine is a member of the etcd cluster.
func EtcdMembershipProblem(status *etcd.MembershipStatusSpec) string {
	var problem string

	switch status.Phase {
	case etcd.MembershipPhaseMember:
		return ""
	case etcd.MembershipPhasePreparing:
		problem = "etcd is waiting for the prerequisites"
	case etcd.MembershipPhaseJoining:
		problem = "etcd is waiting to join the cluster (or for the cluster to be bootstrapped)"
	case etcd.MembershipPhaseLearner:
		problem = "etcd is syncing with the cluster as a learner"
	case etcd.MembershipPhaseLeaving:
		problem = "etcd is leaving the cluster"
	case etcd.MembershipPhaseLeft:
		problem = "etcd left the cluster"
	default:
		problem = fmt.Sprintf("etcd is in phase %s", status.Phase)
	}

	if status.Error != "" {
		problem += ": " + status.Error
	}

	return problem
}
