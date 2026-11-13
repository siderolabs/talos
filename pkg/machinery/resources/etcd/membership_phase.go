// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package etcd

// MembershipPhase describes the phase of the etcd cluster membership of this machine.
type MembershipPhase int

// MembershipPhase values.
//
//structprotogen:gen_enum
const (
	// MembershipPhasePreparing means that the prerequisites (etcd spec, PKI, etcd data volume) are not ready yet.
	MembershipPhasePreparing MembershipPhase = iota // preparing
	// MembershipPhaseJoining means that the etcd data is empty, and the machine is either trying to join
	// an existing etcd cluster, or it is waiting for the bootstrap request.
	MembershipPhaseJoining // joining
	// MembershipPhaseLearner means that the machine joined the etcd cluster as a learner,
	// and it is not promoted to a voting member yet.
	//
	// The etcd data of a learner is disposable: if etcd is interrupted in this phase, the data is wiped,
	// and the machine joins the cluster from scratch.
	MembershipPhaseLearner // learner
	// MembershipPhaseMember means that the machine is a (voting) member of the etcd cluster.
	MembershipPhaseMember // member
	// MembershipPhaseLeaving means that the machine is in the process of leaving the etcd cluster.
	MembershipPhaseLeaving // leaving
	// MembershipPhaseLeft means that the machine left the etcd cluster, and the etcd data was wiped.
	MembershipPhaseLeft // left
)
