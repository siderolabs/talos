// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package client

import (
	"context"
	"encoding/base64"

	"google.golang.org/grpc"
)

// BasicAuth implements the credentials.PerRPCCredentials interface and holds credentials for Basic Auth.
type BasicAuth struct {
	auth string
}

// GetRequestMetadata implements credentials.PerGRPCCredentials.
func (c BasicAuth) GetRequestMetadata(ctx context.Context, url ...string) (map[string]string, error) {
	enc := base64.StdEncoding.EncodeToString([]byte(c.auth))

	return map[string]string{
		"Authorization": "Basic " + enc,
	}, nil
}

// WithGRPCBasicAuth returns gRPC credentials for basic auth.
func WithGRPCBasicAuth(username, password string) grpc.DialOption {
	return grpc.WithPerRPCCredentials(BasicAuth{
		auth: username + ":" + password,
	})
}
