/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package basic

import (
	"crypto/tls"
	"errors"
	"fmt"

	"github.com/talos-systems/talos/pkg/userdata"
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
func NewConnection(address string, port int, creds credentials.PerRPCCredentials) (conn *grpc.ClientConn, err error) {
	grpcOpts := []grpc.DialOption{}

	grpcOpts = append(
		grpcOpts,
		grpc.WithTransportCredentials(
			credentials.NewTLS(&tls.Config{
				InsecureSkipVerify: true,
			})),
		grpc.WithPerRPCCredentials(creds),
	)
	conn, err = grpc.Dial(fmt.Sprintf("%s:%d", address, port), grpcOpts...)
	if err != nil {
		return
	}

	return conn, nil
}

// NewCredentials returns credentials.PerRPCCredentials based on username and
// password, or a token. The token method takes precedence over the username
// and password.
func NewCredentials(data *userdata.Trustd) (creds Credentials, err error) {
	switch {
	case data.Username != "" && data.Password != "":
		creds = NewUsernameAndPasswordCredentials(data.Username, data.Password)
	case data.Token != "":
		creds = NewTokenCredentials(data.Token)
	default:
		return nil, errors.New("failed to find valid credentials")
	}

	return creds, nil
}
