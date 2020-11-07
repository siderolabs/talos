// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package basic

import (
	"context"
	"fmt"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
)

// TokenCredentials implements credentials.PerRPCCredentials. It uses a basic
// token lookup to authenticate users.
type TokenCredentials struct {
	Token string
}

// NewTokenCredentials initializes ClientCredentials with the token.
func NewTokenCredentials(token string) (creds Credentials) {
	creds = &TokenCredentials{
		Token: token,
	}

	return creds
}

// GetRequestMetadata sets the value for the "token" key.
func (b *TokenCredentials) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{
		"token": b.Token,
	}, nil
}

// RequireTransportSecurity is set to true in order to encrypt the
// communication.
func (b *TokenCredentials) RequireTransportSecurity() bool {
	return true
}

func (b *TokenCredentials) authorize(ctx context.Context) error {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if len(md["token"]) > 0 && md["token"][0] == b.Token {
			return nil
		}
	}

	return fmt.Errorf("%s", codes.Unauthenticated.String())
}

// UnaryInterceptor sets the UnaryServerInterceptor for the server and enforces
// basic authentication.
func (b *TokenCredentials) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()

		if err := b.authorize(ctx); err != nil {
			return nil, err
		}

		h, err := handler(ctx, req)

		log.Printf("request - Method:%s\tDuration:%s\tError:%v\n",
			info.FullMethod,
			time.Since(start),
			err,
		)

		return h, err
	}
}
