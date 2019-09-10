/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package gen

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/talos-systems/talos/internal/app/trustd/proto"
	"github.com/talos-systems/talos/pkg/crypto/x509"
	"github.com/talos-systems/talos/pkg/grpc/middleware/auth/basic"
	"github.com/talos-systems/talos/pkg/userdata"
	"google.golang.org/grpc"
)

// Generator represents the OS identity generator.
type Generator struct {
	client proto.TrustdClient
}

// NewGenerator initializes a Generator with a preconfigured grpc.ClientConn.
func NewGenerator(data *userdata.UserData, port int) (g *Generator, err error) {
	if len(data.Services.Trustd.Endpoints) == 0 {
		return nil, fmt.Errorf("at least one root of trust endpoint is required")
	}

	creds, err := basic.NewCredentials(data.Services.Trustd)
	if err != nil {
		return nil, err
	}

	// Loop through trustd endpoints and attempt to download PKI
	var conn *grpc.ClientConn
	var multiError *multierror.Error
	for i := 0; i < len(data.Services.Trustd.Endpoints); i++ {
		conn, err = basic.NewConnection(data.Services.Trustd.Endpoints[i], port, creds)
		if err != nil {
			multiError = multierror.Append(multiError, err)
			// Unable to connect, bail and attempt to contact next endpoint
			continue
		}
		client := proto.NewTrustdClient(conn)
		return &Generator{client: client}, nil
	}

	// We were unable to connect to any trustd endpoint
	// Return error from last attempt.
	return nil, multiError.ErrorOrNil()
}

// Certificate implements the proto.TrustdClient interface.
func (g *Generator) Certificate(in *proto.CertificateRequest) (resp *proto.CertificateResponse, err error) {
	ctx := context.Background()
	resp, err = g.client.Certificate(ctx, in)
	if err != nil {
		return
	}

	return resp, err
}

// Identity creates a CSR and sends it to trustd for signing.
// A signed certificate is returned.
func (g *Generator) Identity(data *userdata.UserData) (err error) {
	if data.Security == nil {
		data.Security = &userdata.Security{}
	}
	data.Security.OS = &userdata.OSSecurity{CA: &x509.PEMEncodedCertificateAndKey{}}
	var csr *x509.CertificateSigningRequest
	if csr, err = data.NewIdentityCSR(); err != nil {
		return err
	}
	req := &proto.CertificateRequest{
		Csr: csr.X509CertificateRequestPEM,
	}

	return poll(g, req, data.Security.OS)
}

func poll(g *Generator, in *proto.CertificateRequest, data *userdata.OSSecurity) (err error) {
	timeout := time.NewTimer(time.Minute * 5)
	defer timeout.Stop()
	tick := time.NewTicker(time.Second * 5)
	defer tick.Stop()

	for {
		select {
		case <-timeout.C:
			return fmt.Errorf("timeout waiting for certificate")
		case <-tick.C:
			resp, _err := g.Certificate(in)
			if _err != nil {
				log.Println(_err)
				continue
			}
			data.CA = &x509.PEMEncodedCertificateAndKey{}
			data.CA.Crt = resp.Ca
			data.Identity.Crt = resp.Crt

			return nil
		}
	}
}
