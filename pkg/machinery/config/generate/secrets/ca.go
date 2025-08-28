// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets

import (
	stdx509 "crypto/x509"
	"time"

	"github.com/siderolabs/crypto/x509"

	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/role"
)

// NewEtcdCA generates a CA for the Etcd PKI.
func NewEtcdCA(currentTime time.Time, contract *config.VersionContract) (ca *x509.CertificateAuthority, err error) {
	opts := []x509.Option{
		x509.Organization("etcd"),
		x509.NotAfter(currentTime.Add(CAValidityTime)),
		x509.NotBefore(currentTime),
		x509.ECDSA(true),
	}

	return x509.NewSelfSignedCertificateAuthority(opts...)
}

// NewKubernetesCA generates a CA for the Kubernetes PKI.
func NewKubernetesCA(currentTime time.Time, contract *config.VersionContract) (ca *x509.CertificateAuthority, err error) {
	opts := []x509.Option{
		x509.Organization("kubernetes"),
		x509.NotAfter(currentTime.Add(CAValidityTime)),
		x509.NotBefore(currentTime),
		x509.ECDSA(true),
	}

	return x509.NewSelfSignedCertificateAuthority(opts...)
}

// NewAggregatorCA generates a CA for the Kubernetes aggregator/front-proxy.
func NewAggregatorCA(currentTime time.Time, contract *config.VersionContract) (ca *x509.CertificateAuthority, err error) {
	opts := []x509.Option{
		x509.ECDSA(true),
		x509.CommonName("front-proxy"),
		x509.NotAfter(currentTime.Add(CAValidityTime)),
		x509.NotBefore(currentTime),
		x509.ECDSA(true),
	}

	return x509.NewSelfSignedCertificateAuthority(opts...)
}

// NewTalosCA generates a CA for the Talos PKI.
func NewTalosCA(currentTime time.Time) (ca *x509.CertificateAuthority, err error) {
	opts := []x509.Option{
		x509.Organization("talos"),
		x509.NotAfter(currentTime.Add(CAValidityTime)),
		x509.NotBefore(currentTime),
	}

	return x509.NewSelfSignedCertificateAuthority(opts...)
}

// NewAdminCertificateAndKey generates the admin Talos certificate and key.
func NewAdminCertificateAndKey(currentTime time.Time, ca *x509.PEMEncodedCertificateAndKey, roles role.Set, ttl time.Duration) (p *x509.PEMEncodedCertificateAndKey, err error) {
	opts := []x509.Option{
		x509.Organization(roles.Strings()...),
		x509.NotAfter(currentTime.Add(ttl)),
		x509.NotBefore(currentTime),
		x509.KeyUsage(stdx509.KeyUsageDigitalSignature),
		x509.ExtKeyUsage([]stdx509.ExtKeyUsage{stdx509.ExtKeyUsageClientAuth}),
	}

	talosCA, err := x509.NewCertificateAuthorityFromCertificateAndKey(ca)
	if err != nil {
		return nil, err
	}

	keyPair, err := x509.NewKeyPair(talosCA, opts...)
	if err != nil {
		return nil, err
	}

	return x509.NewCertificateAndKeyFromKeyPair(keyPair), nil
}
