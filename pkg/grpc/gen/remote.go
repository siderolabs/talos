// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package gen

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/talos-systems/crypto/x509"
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
	client securityapi.SecurityServiceClient
	conn   *grpc.ClientConn
	done   chan struct{}
}

// NewRemoteGenerator initializes a RemoteGenerator with a preconfigured grpc.ClientConn.
func NewRemoteGenerator(token string, endpoints []string) (g *RemoteGenerator, err error) {
	if len(endpoints) == 0 {
		return nil, fmt.Errorf("at least one root of trust endpoint is required")
	}

	creds := basic.NewTokenCredentials(token)

	conn, err := basic.NewConnection(fmt.Sprintf("%s:///%s", trustdResolverScheme, strings.Join(endpoints, ",")), creds)
	if err != nil {
		return nil, err
	}

	client := securityapi.NewSecurityServiceClient(conn)

	g = &RemoteGenerator{
		client: client,
		conn:   conn,
		done:   make(chan struct{}),
	}

	return g, nil
}

// Certificate implements the securityapi.SecurityClient interface.
func (g *RemoteGenerator) Certificate(in *securityapi.CertificateRequest) (resp *securityapi.CertificateResponse, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err = g.client.Certificate(ctx, in)
	if err != nil {
		return nil, err
	}

	return resp, err
}

// Identity creates an identity certificate via the security API.
func (g *RemoteGenerator) Identity(csr *x509.CertificateSigningRequest) (ca, crt []byte, err error) {
	req := &securityapi.CertificateRequest{
		Csr: csr.X509CertificateRequestPEM,
	}

	ca, crt, err = g.poll(req)
	if err != nil {
		return nil, nil, err
	}

	return ca, crt, nil
}

// Close closes the gRPC client connection.
func (g *RemoteGenerator) Close() error {
	g.done <- struct{}{}

	return g.conn.Close()
}

func (g *RemoteGenerator) poll(in *securityapi.CertificateRequest) (ca, crt []byte, err error) {
	timeout := time.NewTimer(time.Minute * 5)
	defer timeout.Stop()

	tick := time.NewTicker(time.Second * 5)
	defer tick.Stop()

	for {
		select {
		case <-timeout.C:
			return nil, nil, fmt.Errorf("timeout waiting for certificate")
		case <-tick.C:
			var resp *securityapi.CertificateResponse

			resp, err = g.Certificate(in)
			if err != nil {
				log.Println(err)

				continue
			}

			ca = resp.Ca
			crt = resp.Crt

			return ca, crt, nil
		case <-g.done:
			return nil, nil, nil
		}
	}
}
