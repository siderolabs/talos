// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package generate

import (
	"errors"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/types/etcd"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
)

// generateEtcdConfigs generates multi-doc etcd configuration for controlplane machines.
func (in *Input) generateEtcdConfigs() ([]config.Document, error) {
	if in.Options.VersionContract.EtcdDisabled() || !in.Options.VersionContract.EtcdMultidocConfig() {
		return nil, nil
	}

	if in.Options.SecretsBundle == nil || in.Options.SecretsBundle.Certs == nil || in.Options.SecretsBundle.Certs.Etcd == nil {
		return nil, errors.New("etcd CA is missing in the secrets bundle")
	}

	caConfig := etcd.NewEtcdCAConfigV1Alpha1()
	caConfig.CAIssuingCA = &meta.CertificateAndKey{
		Cert: string(in.Options.SecretsBundle.Certs.Etcd.Crt),
		Key:  string(in.Options.SecretsBundle.Certs.Etcd.Key),
	}

	return []config.Document{
		caConfig,
		etcd.NewEtcdMemberConfigV1Alpha1(),
	}, nil
}
