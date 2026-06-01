// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package generate

import (
	clientconfig "github.com/siderolabs/talos/pkg/machinery/client/config"
)

// Talosconfig returns the talos admin Talos config.
func (in *Input) Talosconfig() (*clientconfig.Config, error) {
	clientcert, err := in.Options.SecretsBundle.GenerateTalosAPIClientCertificate(in.Options.Roles)
	if err != nil {
		return nil, err
	}

	return clientconfig.NewConfig(in.ClusterName, in.Options.EndpointList, in.Options.SecretsBundle.Certs.OS.Crt, clientcert), nil
}
