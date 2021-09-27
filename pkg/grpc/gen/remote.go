// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package gen

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/talos-systems/crypto/x509"
	"github.com/talos-systems/go-retry/retry"
	"google.golang.org/grpc"

	"github.com/talos-systems/talos/pkg/grpc/middleware/auth/basic"
	securityapi "github.com/talos-systems/talos/pkg/machinery/api/security"
	"github.com/talos-systems/talos/pkg/machinery/client/resolver"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

var trustdResolverScheme string

func init() {
	trustdResolverScheme = resolver.RegisterRoundRobinResolver(constants.TrustdPort)
}

// RemoteGenerator represents the OS identity generator.
type RemoteGenerator struct {
	conn   *grpc.ClientConn
	client securityapi.SecurityServiceClient
}

// NewRemoteGenerator initializes a RemoteGenerator with a preconfigured grpc.ClientConn.
func NewRemoteGenerator(token string, endpoints []string, ca *x509.PEMEncodedCertificateAndKey) (g *RemoteGenerator, err error) {
	if len(endpoints) == 0 {
		return nil, fmt.Errorf("at least one root of trust endpoint is required")
	}

	g = &RemoteGenerator{}

	conn, err := basic.NewConnection(fmt.Sprintf("%s:///%s", trustdResolverScheme, strings.Join(endpoints, ",")), basic.NewTokenCredentials(token), ca)
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

	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	err = retry.Exponential(5*time.Minute,
		retry.WithAttemptTimeout(30*time.Second),
		retry.WithUnits(5*time.Second),
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
	})

	if err != nil {
		return nil, nil, err
	}

	return ca, crt, nil
}

// Close closes the gRPC client connection.
func (g *RemoteGenerator) Close() error {
	return g.conn.Close()
}
