// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package basic

import (
	"bytes"
	"crypto/tls"
	stdx509 "crypto/x509"
	"encoding/asn1"
	"errors"
	"fmt"
	"slices"

	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/gen/xslices"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/siderolabs/talos/pkg/httpdefaults"
	"github.com/siderolabs/talos/pkg/machinery/client/dialer"
)

// Credentials describes an authorization method.
type Credentials interface {
	credentials.PerRPCCredentials

	UnaryInterceptor() grpc.UnaryServerInterceptor
	StreamInterceptor() grpc.StreamServerInterceptor
}

// ConnectionOptions configures the connection.
type ConnectionOptions struct {
	RequiredServerExtKeyUsage asn1.ObjectIdentifier
}

// ConnectionOption is a functional option for NewConnection.
type ConnectionOption func(*ConnectionOptions)

// WithRequiredServerExtKeyUsage requires the server certificate to have the specified custom extended key usage.
//
// The check is performed in addition to the regular certificate verification.
func WithRequiredServerExtKeyUsage(oid asn1.ObjectIdentifier) ConnectionOption {
	return func(opts *ConnectionOptions) {
		opts.RequiredServerExtKeyUsage = oid
	}
}

// NewConnection initializes a grpc.ClientConn configured for basic
// authentication.
func NewConnection(address string, creds credentials.PerRPCCredentials, acceptedCAs []*x509.PEMEncodedCertificate, opts ...ConnectionOption) (conn *grpc.ClientConn, err error) {
	grpcOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(credentials.NewTLS(NewTLSConfig(acceptedCAs, opts...))),
		grpc.WithPerRPCCredentials(creds),
		grpc.WithContextDialer(dialer.DynamicProxyDialerWithTLSConfig(httpdefaults.RootCAsTLSConfig)),
	}

	conn, err = grpc.NewClient(address, grpcOpts...)
	if err != nil {
		return conn, err
	}

	return conn, nil
}

// NewTLSConfig builds client TLS configuration which verifies the server against the accepted CAs.
func NewTLSConfig(acceptedCAs []*x509.PEMEncodedCertificate, opts ...ConnectionOption) *tls.Config {
	var options ConnectionOptions

	for _, opt := range opts {
		opt(&options)
	}

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

	if options.RequiredServerExtKeyUsage != nil {
		requiredEKU := options.RequiredServerExtKeyUsage

		// VerifyConnection is called after the regular certificate verification (chain, hostname, server auth EKU),
		// so we only need to check the custom extended key usage here.
		tlsConfig.VerifyConnection = func(cs tls.ConnectionState) error {
			if len(cs.PeerCertificates) == 0 {
				return errors.New("no server certificate presented")
			}

			if !slices.ContainsFunc(cs.PeerCertificates[0].UnknownExtKeyUsage, requiredEKU.Equal) {
				return fmt.Errorf("server certificate doesn't have the required extended key usage %s", requiredEKU)
			}

			return nil
		}
	}

	return tlsConfig
}
