// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build !sidero.debug
// +build !sidero.debug

package client

import (
	"google.golang.org/grpc/credentials"

	clientconfig "github.com/talos-systems/talos/pkg/machinery/client/config"
)

// RequireTransportSecurity implements credentials.PerRPCCredentials.
func (c BasicAuth) RequireTransportSecurity() bool {
	return true
}

func buildCredentials(configContext *clientconfig.Context, endpoints []string) (credentials.TransportCredentials, error) {
	tlsConfig, err := buildTLSConfig(configContext)
	if err != nil {
		return nil, err
	}

	return credentials.NewTLS(tlsConfig), nil
}
