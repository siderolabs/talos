// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package basic

import (
	"bytes"
	"crypto/tls"
	stdx509 "crypto/x509"

	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/gen/xslices"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/siderolabs/talos/pkg/grpc/dialer"
)

// Credentials describes an authorization method.
type Credentials interface {
	credentials.PerRPCCredentials

	UnaryInterceptor() grpc.UnaryServerInterceptor
}

// NewConnection initializes a grpc.ClientConn configured for basic
// authentication.
func NewConnection(address string, creds credentials.PerRPCCredentials, acceptedCAs []*x509.PEMEncodedCertificate) (conn *grpc.ClientConn, err error) {
	tlsConfig := &tls.Config{}

	tlsConfig.RootCAs = stdx509.NewCertPool()
	tlsConfig.RootCAs.AppendCertsFromPEM(bytes.Join(
		xslices.Map(
			acceptedCAs,
			func(cert *x509.PEMEncodedCertificate) []byte {
				return cert.Crt
			},
		),
		nil,
	))

	grpcOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
		grpc.WithPerRPCCredentials(creds),
		grpc.WithSharedWriteBuffer(true),
		grpc.WithContextDialer(dialer.DynamicProxyDialer),
	}

	conn, err = grpc.NewClient(address, grpcOpts...)
	if err != nil {
		return
	}

	return conn, nil
}
