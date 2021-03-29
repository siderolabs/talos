// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"fmt"
	goruntime "runtime"

	"github.com/talos-systems/crypto/x509"

	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// Image implements the config.Etcd interface.
func (e *EtcdConfig) Image() string {
	image := e.ContainerImage
	suffix := ""

	if goruntime.GOARCH == "arm64" {
		suffix = "-arm64"
	}

	if image == "" {
		image = fmt.Sprintf("%s:%s%s", constants.EtcdImage, constants.DefaultEtcdVersion, suffix)
	}

	return image
}

// CA implements the config.Etcd interface.
func (e *EtcdConfig) CA() *x509.PEMEncodedCertificateAndKey {
	return e.RootCA
}

// ExtraArgs implements the config.Etcd interface.
func (e *EtcdConfig) ExtraArgs() map[string]string {
	if e.EtcdExtraArgs == nil {
		return make(map[string]string)
	}

	return e.EtcdExtraArgs
}
