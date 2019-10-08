/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package cluster

import (
	"github.com/talos-systems/talos/pkg/config/machine"
	"github.com/talos-systems/talos/pkg/crypto/x509"
)

// Cluster defines the requirements for a config that pertains to cluster
// related options.
type Cluster interface {
	Version() string
	IPs() []string
	Token() Token
	CertSANs() []string
	CA() *x509.PEMEncodedCertificateAndKey
	AESCBCEncryptionSecret() string
	Config(machine.Type) (string, error)
	Etcd() Etcd
	Network() Network
}

// Network defines the requirements for a config that pertains to cluster
// network options.
type Network interface {
	CNI() string
	PodCIDR() string
	ServiceCIDR() string
}

// Etcd defines the requirements for a config that pertains to etcd related
// options.
type Etcd interface {
	Image() string
	CA() *x509.PEMEncodedCertificateAndKey
}

// Token defines the requirements for a config that pertains to Kubernetes
// bootstrap token.
type Token interface {
	ID() string
	Secret() string
}
