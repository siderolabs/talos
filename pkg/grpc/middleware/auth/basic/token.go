// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package basic

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
)

// TokenGetterFunc is the function to dynamically retrieve the token.
type TokenGetterFunc func(context.Context) (string, error)

// TokenCredentials implements credentials.PerRPCCredentials. It uses a basic
// token lookup to authenticate users.
type TokenCredentials struct {
	tokenGetter TokenGetterFunc
}

// NewTokenCredentials initializes ClientCredentials with the token.
func NewTokenCredentials(token string) (creds Credentials) {
	creds = &TokenCredentials{
		tokenGetter: func(context.Context) (string, error) {
			return token, nil
		},
	}

	return creds
}

// NewTokenCredentialsDynamic initializes ClientCredentials with the dynamic token token.
func NewTokenCredentialsDynamic(f TokenGetterFunc) (creds Credentials) {
	creds = &TokenCredentials{
		tokenGetter: f,
	}

	return creds
}

// GetRequestMetadata sets the value for the "token" key.
func (b *TokenCredentials) GetRequestMetadata(ctx context.Context, s ...string) (map[string]string, error) {
	token, err := b.tokenGetter(ctx)
	if err != nil {
		return nil, err
	}

	return map[string]string{
		"token": token,
	}, nil
}

// RequireTransportSecurity is set to true in order to encrypt the
// communication.
func (b *TokenCredentials) RequireTransportSecurity() bool {
	return true
}

func (b *TokenCredentials) authenticate(ctx context.Context) error {
	token, err := b.tokenGetter(ctx)
	if err != nil {
		return err
	}

	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if len(md["token"]) > 0 && md["token"][0] == token {
			return nil
		}
	}

	return fmt.Errorf("%s", codes.Unauthenticated.String())
}

// UnaryInterceptor sets the UnaryServerInterceptor for the server and enforces
// basic authentication.
func (b *TokenCredentials) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if err := b.authenticate(ctx); err != nil {
			return nil, err
		}

		return handler(ctx, req)
	}
}
