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

// UsernameAndPasswordCredentials implements credentials.PerRPCCredentials. It uses a basic
// username and password lookup to authenticate users.
type UsernameAndPasswordCredentials struct {
	Username string
	Password string
}

// NewUsernameAndPasswordCredentials initializes username and password
// Credentials.
func NewUsernameAndPasswordCredentials(username, password string) (creds Credentials) {
	creds = &UsernameAndPasswordCredentials{
		Username: username,
		Password: password,
	}

	return creds
}

// GetRequestMetadata sets the value for the username and password.
func (b *UsernameAndPasswordCredentials) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{
		"username": b.Username,
		"password": b.Password,
	}, nil
}

// RequireTransportSecurity is set to true in order to encrypt the
// communication.
func (b *UsernameAndPasswordCredentials) RequireTransportSecurity() bool {
	return true
}

func (b *UsernameAndPasswordCredentials) authorize(ctx context.Context) error {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if len(md["username"]) > 0 && md["username"][0] == b.Username &&
			len(md["password"]) > 0 && md["password"][0] == b.Password {
			return nil
		}

		return fmt.Errorf("%s", codes.Unauthenticated.String())
	}

	return nil
}

// UnaryInterceptor sets the UnaryServerInterceptor for the server and enforces
// basic authentication.
func (b *UsernameAndPasswordCredentials) UnaryInterceptor() grpc.UnaryServerInterceptor {
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
