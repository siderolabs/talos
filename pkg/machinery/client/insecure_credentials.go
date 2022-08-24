// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build sidero.debug

package client

import (
	"net/url"

	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	clientconfig "github.com/talos-systems/talos/pkg/machinery/client/config"
)

// shouldInsecureConnectionsBeAllowed returns true if one endpoint starts with http://
func shouldInsecureConnectionsBeAllowed(endpoints []string) bool {
	for _, endpoint := range endpoints {
		u, _ := url.Parse(endpoint)
		if u.Scheme == "http" {
			return true
		}
	}

	return false
}

// RequireTransportSecurity enables basic auth with insecure gRPC transport credentials.
func (c BasicAuth) RequireTransportSecurity() bool {
	return false
}

func buildCredentials(configContext *clientconfig.Context, endpoints []string) (credentials.TransportCredentials, error) {
	if shouldInsecureConnectionsBeAllowed(endpoints) {
		return insecure.NewCredentials(), nil
	}

	tlsConfig, err := buildTLSConfig(configContext)
	if err != nil {
		return nil, err
	}

	return credentials.NewTLS(tlsConfig), nil
}
