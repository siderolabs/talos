// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build !no_siderov1_auth

package client

import (
	"github.com/siderolabs/go-api-signature/pkg/client/interceptor"
	"github.com/siderolabs/go-api-signature/pkg/pgp/client"
	"google.golang.org/grpc"

	clientconfig "github.com/siderolabs/talos/pkg/machinery/client/config"
	"github.com/siderolabs/talos/pkg/machinery/version"
)

// sideroV1DialOptions returns the gRPC dial options which sign requests with the SideroV1 auth.
func (c *Client) sideroV1DialOptions(sideroV1 *clientconfig.SideroV1) ([]grpc.DialOption, error) {
	var contextName string

	if c.options.config != nil {
		contextName = c.options.config.Context
	}

	if c.options.contextOverrideSet {
		contextName = c.options.contextOverride
	}

	authInterceptor := interceptor.New(interceptor.Options{
		UserKeyProvider:      getKeyProvider(c.options.sideroV1KeysDir),
		ContextName:          contextName,
		Identity:             sideroV1.Identity,
		ClientName:           version.Name,
		ServiceAccountBase64: c.options.serviceAccountBase64,
	})

	return []grpc.DialOption{
		grpc.WithUnaryInterceptor(authInterceptor.Unary()),
		grpc.WithStreamInterceptor(authInterceptor.Stream()),
	}, nil
}

func getKeyProvider(customKeysDir string) *client.KeyProvider {
	if customKeysDir != "" {
		return client.NewKeyProviderWithFallback("talos/keys", customKeysDir, "", true)
	}

	talosDir, err := clientconfig.GetTalosDirectory()
	if err != nil {
		// TODO: start failing instead of falling back to XDG data directory if we can't resolve Talos directory
		return client.NewKeyProvider("talos/keys")
	}

	return client.NewKeyProviderWithFallback("talos/keys", talosDir, "keys", true)
}
