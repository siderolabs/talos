// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"net"
	"net/url"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	clientconfig "github.com/talos-systems/talos/pkg/machinery/client/config"
	"github.com/talos-systems/talos/pkg/machinery/client/resolver"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/machinery/generic/slices"
)

// Conn returns underlying client connection.
func (c *Client) Conn() grpc.ClientConnInterface {
	return c.conn
}

// getConn creates new gRPC connection.
func (c *Client) getConn(ctx context.Context, opts ...grpc.DialOption) (*grpcConnectionWrapper, error) {
	endpoints := c.GetEndpoints()

	target := c.getTarget(
		resolver.EnsureEndpointsHavePorts(
			reduceURLsToAddresses(endpoints),
			constants.ApidPort),
	)

	dialOpts := []grpc.DialOption(nil)

	dialOpts = append(dialOpts, c.options.grpcDialOptions...)

	dialOpts = append(dialOpts, opts...)

	if c.options.unixSocketPath != "" {
		conn, err := grpc.DialContext(ctx, target, dialOpts...)

		return newGRPCConnectionWrapper(c.GetClusterName(), conn), err
	}

	tlsConfig := c.options.tlsConfig

	if tlsConfig != nil {
		return c.makeConnection(ctx, target, credentials.NewTLS(tlsConfig), dialOpts)
	}

	if err := c.resolveConfigContext(); err != nil {
		return nil, fmt.Errorf("failed to resolve configuration context: %w", err)
	}

	basicAuth := c.options.configContext.Auth.Basic
	if basicAuth != nil {
		dialOpts = append(dialOpts, WithGRPCBasicAuth(basicAuth.Username, basicAuth.Password))
	}

	creds, err := buildCredentials(c.options.configContext, endpoints)
	if err != nil {
		return nil, err
	}

	return c.makeConnection(ctx, target, creds, dialOpts)
}

func buildTLSConfig(configContext *clientconfig.Context) (*tls.Config, error) {
	tlsConfig := &tls.Config{}

	caBytes, err := getCA(configContext)
	if err != nil {
		return nil, fmt.Errorf("failed to get CA: %w", err)
	}

	if len(caBytes) > 0 {
		tlsConfig.RootCAs = x509.NewCertPool()

		if ok := tlsConfig.RootCAs.AppendCertsFromPEM(caBytes); !ok {
			return nil, fmt.Errorf("failed to append CA certificate to RootCAs pool")
		}
	}

	crt, err := CertificateFromConfigContext(configContext)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire credentials: %w", err)
	}

	if crt != nil {
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
		tlsConfig.Certificates = append(tlsConfig.Certificates, *crt)
	}

	return tlsConfig, nil
}

func (c *Client) makeConnection(ctx context.Context, target string, creds credentials.TransportCredentials, dialOpts []grpc.DialOption) (*grpcConnectionWrapper, error) {
	dialOpts = append(dialOpts,
		grpc.WithTransportCredentials(creds),
		grpc.WithInitialWindowSize(65535*32),
		grpc.WithInitialConnWindowSize(65535*16))

	conn, err := grpc.DialContext(ctx, target, dialOpts...)

	return newGRPCConnectionWrapper(c.GetClusterName(), conn), err
}

func (c *Client) getTarget(endpoints []string) string {
	switch {
	case c.options.unixSocketPath != "":
		return fmt.Sprintf("unix:///%s", c.options.unixSocketPath)
	case len(endpoints) > 1:
		return fmt.Sprintf("%s:///%s", resolver.RoundRobinResolverScheme, strings.Join(endpoints, ","))
	default:
		// NB: we use the `dns` scheme here in order to handle fancier situations
		// when there is a single endpoint.
		// Such possibilities include SRV records, multiple IPs from A and/or AAAA
		// records, and descriptive TXT records which include things like load
		// balancer specs.
		return fmt.Sprintf("dns:///%s", endpoints[0])
	}
}

func getCA(context *clientconfig.Context) ([]byte, error) {
	if context.CA == "" {
		return nil, nil
	}

	caBytes, err := base64.StdEncoding.DecodeString(context.CA)
	if err != nil {
		return nil, fmt.Errorf("error decoding CA: %w", err)
	}

	return caBytes, err
}

// CertificateFromConfigContext constructs the client Credentials from the given configuration Context.
func CertificateFromConfigContext(context *clientconfig.Context) (*tls.Certificate, error) {
	if context.Crt == "" && context.Key == "" {
		return nil, nil
	}

	crtBytes, err := base64.StdEncoding.DecodeString(context.Crt)
	if err != nil {
		return nil, fmt.Errorf("error decoding certificate: %w", err)
	}

	keyBytes, err := base64.StdEncoding.DecodeString(context.Key)
	if err != nil {
		return nil, fmt.Errorf("error decoding key: %w", err)
	}

	crt, err := tls.X509KeyPair(crtBytes, keyBytes)
	if err != nil {
		return nil, fmt.Errorf("could not load client key pair: %s", err)
	}

	return &crt, nil
}

func reduceURLsToAddresses(endpoints []string) []string {
	return slices.Map(endpoints, func(endpoint string) string {
		u, err := url.Parse(endpoint)
		if err != nil {
			return endpoint
		}

		if u.Scheme == "https" && u.Port() == "" {
			return net.JoinHostPort(u.Hostname(), "443")
		}

		if u.Scheme != "" {
			if u.Port() != "" {
				return net.JoinHostPort(u.Hostname(), u.Port())
			}

			if u.Opaque == "" {
				return u.Host
			}
		}

		return endpoint
	})
}
