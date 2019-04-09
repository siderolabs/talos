/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package gen

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/talos-systems/talos/internal/app/trustd/proto"
	"github.com/talos-systems/talos/internal/pkg/grpc/middleware/auth/basic"
	"github.com/talos-systems/talos/pkg/crypto/x509"
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

	creds := basic.NewCredentials(
		data.Services.Trustd.Username,
		data.Services.Trustd.Password,
	)

	// TODO: In the case of failure, attempt to generate the identity from
	// another RoT.
	var conn *grpc.ClientConn
	conn, err = basic.NewConnection(data.Services.Trustd.Endpoints[0], port, creds)
	if err != nil {
		return nil, err
	}
	client := proto.NewTrustdClient(conn)

	return &Generator{
		client: client,
	}, nil
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
	timeout := time.NewTimer(time.Minute * 5).C
	tick := time.NewTicker(time.Second * 5).C

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for certificate")
		case <-tick:
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
