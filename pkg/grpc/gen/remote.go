// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package gen

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/go-retry/retry"
	"google.golang.org/grpc"

	"github.com/siderolabs/talos/pkg/grpc/middleware/auth/basic"
	securityapi "github.com/siderolabs/talos/pkg/machinery/api/security"
	"github.com/siderolabs/talos/pkg/machinery/client/resolver"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// RemoteGenerator represents the OS identity generator.
type RemoteGenerator struct {
	conn   *grpc.ClientConn
	client securityapi.SecurityServiceClient
}

// NewRemoteGenerator initializes a RemoteGenerator with a preconfigured grpc.ClientConn.
func NewRemoteGenerator(token string, endpoints []string, ca *x509.PEMEncodedCertificateAndKey) (g *RemoteGenerator, err error) {
	if len(endpoints) == 0 {
		return nil, errors.New("at least one root of trust endpoint is required")
	}

	endpoints = resolver.EnsureEndpointsHavePorts(endpoints, constants.TrustdPort)

	g = &RemoteGenerator{}

	conn, err := basic.NewConnection(fmt.Sprintf("%s:///%s", resolver.RoundRobinResolverScheme, strings.Join(endpoints, ",")), basic.NewTokenCredentials(token), ca)
	if err != nil {
		return nil, err
	}

	g.conn = conn
	g.client = securityapi.NewSecurityServiceClient(g.conn)

	return g, nil
}

// Identity creates an identity certificate via the security API.
func (g *RemoteGenerator) Identity(csr *x509.CertificateSigningRequest) (ca, crt []byte, err error) {
	return g.IdentityContext(context.Background(), csr)
}

// IdentityContext creates an identity certificate via the security API.
func (g *RemoteGenerator) IdentityContext(ctx context.Context, csr *x509.CertificateSigningRequest) (ca, crt []byte, err error) {
	req := &securityapi.CertificateRequest{
		Csr: csr.X509CertificateRequestPEM,
	}

	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	if err = retry.Exponential(time.Minute,
		retry.WithAttemptTimeout(10*time.Second),
		retry.WithUnits(time.Second),
		retry.WithJitter(100*time.Millisecond),
	).RetryWithContext(ctx, func(ctx context.Context) error {
		var resp *securityapi.CertificateResponse

		resp, err = g.client.Certificate(ctx, req)
		if err != nil {
			return retry.ExpectedError(err)
		}

		ca = resp.Ca
		crt = resp.Crt

		return nil
	}); err != nil {
		return nil, nil, err
	}

	return ca, crt, nil
}

// Close closes the gRPC client connection.
func (g *RemoteGenerator) Close() error {
	return g.conn.Close()
}
