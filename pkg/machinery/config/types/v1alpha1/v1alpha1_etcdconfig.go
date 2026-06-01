// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"fmt"

	"github.com/siderolabs/crypto/x509"

	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// Image implements the config.Etcd interface.
func (e *EtcdConfig) Image() string {
	image := e.ContainerImage

	if image == "" {
		image = fmt.Sprintf("%s:%s", constants.EtcdImage, constants.DefaultEtcdVersion)
	}

	return image
}

// CA implements the config.Etcd interface.
func (e *EtcdConfig) CA() *x509.PEMEncodedCertificateAndKey {
	return e.RootCA
}

// ExtraArgs implements the config.Etcd interface.
func (e *EtcdConfig) ExtraArgs() map[string][]string {
	return e.EtcdExtraArgs.ToMap()
}

// AdvertisedSubnets implements the config.Etcd interface.
func (e *EtcdConfig) AdvertisedSubnets() []string {
	if len(e.EtcdAdvertisedSubnets) > 0 {
		return e.EtcdAdvertisedSubnets
	}

	if e.EtcdSubnet != "" {
		return []string{e.EtcdSubnet}
	}

	return nil
}

// ListenSubnets implements the config.Etcd interface.
func (e *EtcdConfig) ListenSubnets() []string {
	if len(e.EtcdListenSubnets) > 0 {
		return e.EtcdListenSubnets
	}

	// if advertised subnets are set, use them
	if len(e.EtcdAdvertisedSubnets) > 0 {
		return e.EtcdAdvertisedSubnets
	}

	// nothing set, rely on defaults (listen on all interfaces)

	return nil
}
