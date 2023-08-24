// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package generate provides Talos machine configuration generation and client config generation.
//
// This package is deprecated, use github.com/siderolabs/talos/pkg/machinery/config/generate instead.
package generate

import (
	"time"

	"github.com/siderolabs/crypto/x509"

	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/generate"
	"github.com/siderolabs/talos/pkg/machinery/config/generate/secrets"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	v1alpha1 "github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/role"
)

// Config returns the talos config for a given node type.
func Config(t machine.Type, in *Input) (*v1alpha1.Config, error) {
	cfg, err := in.Config(t)
	if err != nil {
		return nil, err
	}

	return cfg.RawV1Alpha1(), nil
}

// Input holds info about certs, ips, and node type.
//
//nolint:maligned
type Input = generate.Input

// Certs holds the base64 encoded keys and certificates.
type Certs = secrets.Certs

// Cluster holds Talos cluster-wide secrets.
type Cluster = secrets.Cluster

// Secrets holds the sensitive kubeadm data.
type Secrets = secrets.Secrets

// TrustdInfo holds the trustd credentials.
type TrustdInfo = secrets.TrustdInfo

// SecretsBundle holds trustd, kubeadm and certs information.
type SecretsBundle = secrets.Bundle

// Clock system clock.
type Clock = secrets.Clock

// SystemClock is a real system clock, but the time returned can be made fixed.
type SystemClock = secrets.SystemClock

// NewClock creates new SystemClock.
//
// Deprecated: use secrets.NewClock instead.
func NewClock() *SystemClock {
	return secrets.NewClock()
}

// NewSecretsBundle creates secrets bundle generating all secrets or reading from the input options if provided.
//
// Deprecated: use generate.NewSecretsBundle instead.
func NewSecretsBundle(clock Clock, opts ...GenOption) (*SecretsBundle, error) {
	o := generate.DefaultOptions()

	for _, opt := range opts {
		if err := opt(&o); err != nil {
			return nil, err
		}
	}

	return secrets.NewBundle(clock, o.VersionContract)
}

// NewSecretsBundleFromKubernetesPKI creates secrets bundle by reading the contents
// of a Kubernetes PKI directory (typically `/etc/kubernetes/pki`) and using the provided bootstrapToken as input.
//
// Deprecated: use generate.NewSecretsBundleFromKubernetesPKI instead.
func NewSecretsBundleFromKubernetesPKI(pkiDir, bootstrapToken string, versionContract *config.VersionContract) (*SecretsBundle, error) {
	return secrets.NewBundleFromKubernetesPKI(pkiDir, bootstrapToken, versionContract)
}

// NewSecretsBundleFromConfig creates secrets bundle using existing config.
//
// Deprecated: use generate.NewSecretsBundleFromConfig instead.
func NewSecretsBundleFromConfig(clock Clock, c config.Provider) *SecretsBundle {
	return secrets.NewBundleFromConfig(clock, c)
}

// NewEtcdCA generates a CA for the Etcd PKI.
//
// Deprecated: use secrets.NewEtcdCA instead.
func NewEtcdCA(currentTime time.Time, contract *config.VersionContract) (ca *x509.CertificateAuthority, err error) {
	return secrets.NewEtcdCA(currentTime, contract)
}

// NewKubernetesCA generates a CA for the Kubernetes PKI.
//
// Deprecated: use secrets.NewKubernetesCA instead.
func NewKubernetesCA(currentTime time.Time, contract *config.VersionContract) (ca *x509.CertificateAuthority, err error) {
	return secrets.NewKubernetesCA(currentTime, contract)
}

// NewAggregatorCA generates a CA for the Kubernetes aggregator/front-proxy.
//
// Deprecated: use secrets.NewAggregatorCA instead.
func NewAggregatorCA(currentTime time.Time, contract *config.VersionContract) (ca *x509.CertificateAuthority, err error) {
	return secrets.NewAggregatorCA(currentTime, contract)
}

// NewTalosCA generates a CA for the Talos PKI.
//
// Deprecated: use secrets.NewTalosCA instead.
func NewTalosCA(currentTime time.Time) (ca *x509.CertificateAuthority, err error) {
	return secrets.NewTalosCA(currentTime)
}

// NewAdminCertificateAndKey generates the admin Talos certificate and key.
//
// Deperecated: use secrets.NewAdminCertificateAndKey instead.
func NewAdminCertificateAndKey(currentTime time.Time, ca *x509.PEMEncodedCertificateAndKey, roles role.Set, ttl time.Duration) (p *x509.PEMEncodedCertificateAndKey, err error) {
	return secrets.NewAdminCertificateAndKey(currentTime, ca, roles, ttl)
}

// NewInput generates the sensitive data required to generate all config
// types.
//
// Deprecated: use generate.NewInput instead.
func NewInput(clustername, endpoint, kubernetesVersion string, secrets *SecretsBundle, opts ...GenOption) (input *Input, err error) {
	return generate.NewInput(clustername, endpoint, kubernetesVersion, append(opts, generate.WithSecretsBundle(secrets))...)
}
