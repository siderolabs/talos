// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package check_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/cluster/check"
	"github.com/siderolabs/talos/pkg/machinery/resources/etcd"
)

func TestEtcdMembershipProblem(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		status   etcd.MembershipStatusSpec
		expected string
	}{
		{
			name:   "member",
			status: etcd.MembershipStatusSpec{Phase: etcd.MembershipPhaseMember},
		},
		{
			name:   "member with a stale error",
			status: etcd.MembershipStatusSpec{Phase: etcd.MembershipPhaseMember, Error: "ignored"},
		},
		{
			name:     "preparing",
			status:   etcd.MembershipStatusSpec{Phase: etcd.MembershipPhasePreparing},
			expected: "etcd is waiting for the prerequisites",
		},
		{
			name:     "joining",
			status:   etcd.MembershipStatusSpec{Phase: etcd.MembershipPhaseJoining, Error: "no controlplane endpoints discovered yet"},
			expected: "etcd is waiting to join the cluster (or for the cluster to be bootstrapped): no controlplane endpoints discovered yet",
		},
		{
			name:     "learner",
			status:   etcd.MembershipStatusSpec{Phase: etcd.MembershipPhaseLearner},
			expected: "etcd is syncing with the cluster as a learner",
		},
		{
			name:     "leaving",
			status:   etcd.MembershipStatusSpec{Phase: etcd.MembershipPhaseLeaving},
			expected: "etcd is leaving the cluster",
		},
		{
			name:     "left",
			status:   etcd.MembershipStatusSpec{Phase: etcd.MembershipPhaseLeft},
			expected: "etcd left the cluster",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.expected, check.EtcdMembershipProblem(&test.status))
		})
	}
}
