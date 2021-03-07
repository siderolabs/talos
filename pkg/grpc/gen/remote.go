// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package gen

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
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
	done  chan struct{}
	creds basic.Credentials

	// connMu protects conn & client
	connMu sync.Mutex
	conn   *grpc.ClientConn
	client securityapi.SecurityServiceClient
}

// NewRemoteGenerator initializes a RemoteGenerator with a preconfigured grpc.ClientConn.
func NewRemoteGenerator(token string, endpoints []string) (g *RemoteGenerator, err error) {
	if len(endpoints) == 0 {
		return nil, fmt.Errorf("at least one root of trust endpoint is required")
	}

	g = &RemoteGenerator{
		done:  make(chan struct{}),
		creds: basic.NewTokenCredentials(token),
	}

	if err = g.SetEndpoints(endpoints); err != nil {
		return nil, err
	}

	return g, nil
}

// SetEndpoints updates the list of endpoints to talk to.
func (g *RemoteGenerator) SetEndpoints(endpoints []string) error {
	conn, err := basic.NewConnection(fmt.Sprintf("%s:///%s", trustdResolverScheme, strings.Join(endpoints, ",")), g.creds)
	if err != nil {
		return err
	}

	g.connMu.Lock()
	defer g.connMu.Unlock()

	g.conn = conn
	g.client = securityapi.NewSecurityServiceClient(g.conn)

	return nil
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

func (g *RemoteGenerator) certificate(in *securityapi.CertificateRequest) (resp *securityapi.CertificateResponse, err error) {
	g.connMu.Lock()
	defer g.connMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return g.client.Certificate(ctx, in)
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

			resp, err = g.certificate(in)
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
