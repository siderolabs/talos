// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package basic

import (
	"bytes"
	"crypto/tls"
	stdx509 "crypto/x509"
	"net"

	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/gen/xslices"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/siderolabs/talos/pkg/httpdefaults"
	"github.com/siderolabs/talos/pkg/machinery/client/dialer"
	"github.com/siderolabs/talos/pkg/machinery/labels"
)

// Credentials describes an authorization method.
type Credentials interface {
	credentials.PerRPCCredentials

	UnaryInterceptor() grpc.UnaryServerInterceptor
}

// NewConnection initializes a grpc.ClientConn configured for basic
// authentication.
func NewConnection(address string, host string, creds credentials.PerRPCCredentials, acceptedCAs []*x509.PEMEncodedCertificate) (conn *grpc.ClientConn, err error) {
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
		grpc.WithAuthority(ParseAuthority(host)),
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
		grpc.WithPerRPCCredentials(creds),
		grpc.WithSharedWriteBuffer(true),
		grpc.WithContextDialer(dialer.DynamicProxyDialerWithTLSConfig(httpdefaults.RootCAsTLSConfig)),
	}

	conn, err = grpc.NewClient(address, grpcOpts...)
	if err != nil {
		return conn, err
	}

	return conn, nil
}

// ParseAuthority checks if provided host parameter is neither empty nor
// an IP address and returns the extracted host if found
// or an empty string in all other cases.
func ParseAuthority(host string) string {
	if host == "" {
		return ""
	}

	var parsedHost string

	// Check if port is provided and remove it
	h, _, err := net.SplitHostPort(host)
	if err == nil {
		parsedHost = h
	} else {
		parsedHost = host
	}

	// If parsedHost is an IP address it should not be used as an authority
	if ip := net.ParseIP(parsedHost); ip != nil {
		return ""
	}

	if err := labels.ValidateDNS1123Subdomain(parsedHost); err != nil {
		return ""
	}

	// Otherwise return the parsed host
	return parsedHost
}
