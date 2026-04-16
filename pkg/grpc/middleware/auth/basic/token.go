// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package basic

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
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

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "missing token")
	}

	if len(md["token"]) == 0 {
		return status.Error(codes.Unauthenticated, "missing token")
	}

	incomingTokenHash := sha256.Sum256([]byte(md["token"][0]))
	expectedTokenHash := sha256.Sum256([]byte(token))

	if subtle.ConstantTimeCompare(incomingTokenHash[:], expectedTokenHash[:]) != 1 {
		return status.Error(codes.Unauthenticated, "invalid token")
	}

	return nil
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

// StreamInterceptor sets the StreamServerInterceptor for the server and enforces
// basic authentication.
//
// For now, it rejects any API, as we don't have any streaming APIs in trustd component.
// This is to prevent accidentally allowing unauthenticated access to streaming APIs in the future without realizing it.
func (b *TokenCredentials) StreamInterceptor() grpc.StreamServerInterceptor {
	return func(any, grpc.ServerStream, *grpc.StreamServerInfo, grpc.StreamHandler) error {
		return status.Error(codes.Unimplemented, "streaming APIs are not supported")
	}
}
