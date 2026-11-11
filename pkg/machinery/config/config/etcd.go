// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import (
	"github.com/siderolabs/crypto/x509"
)

// EtcdCAConfig defines the etcd PKI configuration.
//
// The etcd CA is only present on the controlplane machines.
type EtcdCAConfig interface {
	EtcdCAConfigSignal()
	// IssuingCA returns the key pair used to issue etcd certificates.
	IssuingCA() *x509.PEMEncodedCertificateAndKey
	// AcceptedCAs returns the list of CA certificates trusted by etcd and etcd clients.
	//
	// The issuing CA certificate is always the first element of the list.
	AcceptedCAs() []*x509.PEMEncodedCertificate
}

// EtcdMemberConfig defines the configuration of the etcd member running on the machine.
//
// The etcd member configuration is only present on the controlplane machines,
// and it requires the etcd CA configuration to be present as well.
type EtcdMemberConfig interface {
	EtcdMemberConfigSignal()
	// Image returns the etcd container image reference.
	Image() string
	// ExtraArgs returns the extra arguments to pass to etcd.
	ExtraArgs() map[string][]string
	// AdvertisedSubnets returns the subnets to pick the etcd advertised addresses from.
	AdvertisedSubnets() []string
	// ListenSubnets returns the subnets to pick the etcd listen addresses from.
	ListenSubnets() []string
	// ManualLearnerPromotion returns true if a member that joins as a learner is left a learner
	// for the operator to promote, instead of being promoted by Talos once it catches up.
	ManualLearnerPromotion() bool
}
