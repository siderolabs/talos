// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package generate

import (
	"encoding/base64"

	"github.com/talos-systems/talos/pkg/machinery/client/config"
)

// Talosconfig returns the talos admin Talos config.
func Talosconfig(in *Input, opts ...GenOption) (*config.Config, error) {
	options := DefaultGenOptions()

	for _, opt := range opts {
		if err := opt(&options); err != nil {
			return nil, err
		}
	}

	return &config.Config{
		Context: in.ClusterName,
		Contexts: map[string]*config.Context{
			in.ClusterName: {
				Endpoints: options.EndpointList,
				CA:        base64.StdEncoding.EncodeToString(in.Certs.OS.Crt),
				Crt:       base64.StdEncoding.EncodeToString(in.Certs.Admin.Crt),
				Key:       base64.StdEncoding.EncodeToString(in.Certs.Admin.Key),
			},
		},
	}, nil
}
