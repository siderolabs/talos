// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package basic

import (
	"crypto/tls"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// Credentials describes an authorization method.
type Credentials interface {
	credentials.PerRPCCredentials

	UnaryInterceptor() grpc.UnaryServerInterceptor
}

// NewConnection initializes a grpc.ClientConn configured for basic
// authentication.
func NewConnection(address string, creds credentials.PerRPCCredentials) (conn *grpc.ClientConn, err error) {
	grpcOpts := []grpc.DialOption{}

	grpcOpts = append(
		grpcOpts,
		grpc.WithTransportCredentials(
			credentials.NewTLS(&tls.Config{
				InsecureSkipVerify: true,
			})),
		grpc.WithPerRPCCredentials(creds),
	)

	conn, err = grpc.Dial(address, grpcOpts...)
	if err != nil {
		return
	}

	return conn, nil
}
