// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"github.com/siderolabs/crypto/x509"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
)

// EtcdCAConfig implements the config.Config interface.
func (c *Config) EtcdCAConfig() config.EtcdCAConfig {
	if c.ClusterConfig == nil || c.ClusterConfig.EtcdConfig == nil || c.ClusterConfig.EtcdConfig.RootCA == nil { //nolint:staticcheck // legacy config
		return nil
	}

	return etcdCAConfigShim{
		e: c.ClusterConfig.EtcdConfig, //nolint:staticcheck // legacy config
	}
}

type etcdCAConfigShim struct {
	e *EtcdConfig
}

// EtcdCAConfigSignal implements the config.EtcdCAConfig interface.
func (s etcdCAConfigShim) EtcdCAConfigSignal() {}

// IssuingCA implements the config.EtcdCAConfig interface.
func (s etcdCAConfigShim) IssuingCA() *x509.PEMEncodedCertificateAndKey {
	return s.e.RootCA
}

// AcceptedCAs implements the config.EtcdCAConfig interface.
func (s etcdCAConfigShim) AcceptedCAs() []*x509.PEMEncodedCertificate {
	return []*x509.PEMEncodedCertificate{
		{
			Crt: s.e.RootCA.Crt,
		},
	}
}

// EtcdMemberConfig implements the config.Config interface.
//
// The legacy .cluster.etcd carries both the CA and the member settings, so the member config
// is reported only if the CA is set: this keeps EtcdMemberConfig and EtcdCAConfig consistent,
// matching the multi-doc rule that the member config requires the CA config.
func (c *Config) EtcdMemberConfig() config.EtcdMemberConfig {
	if c.ClusterConfig == nil || c.ClusterConfig.EtcdConfig == nil || c.ClusterConfig.EtcdConfig.RootCA == nil { //nolint:staticcheck // legacy config
		return nil
	}

	return c.ClusterConfig.EtcdConfig //nolint:staticcheck // legacy config
}
