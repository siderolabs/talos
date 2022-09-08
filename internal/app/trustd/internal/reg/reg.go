// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package reg

import (
	"context"
	stdx509 "crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"log"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/crypto/x509"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	securityapi "github.com/talos-systems/talos/pkg/machinery/api/security"
	"github.com/talos-systems/talos/pkg/machinery/resources/secrets"
)

// Registrator is the concrete type that implements the factory.Registrator and
// securityapi.SecurityServiceServer interfaces.
type Registrator struct {
	securityapi.UnimplementedSecurityServiceServer

	Resources state.State
}

// Register implements the factory.Registrator interface.
//
//nolint:interfacer
func (r *Registrator) Register(s *grpc.Server) {
	securityapi.RegisterSecurityServiceServer(s, r)
}

// Certificate implements the securityapi.SecurityServer interface.
//
// This API is called by Talos worker nodes to request a server certificate for apid running on the node.
// Control plane nodes generate certificates (client and server) directly from machine config PKI.
func (r *Registrator) Certificate(ctx context.Context, in *securityapi.CertificateRequest) (resp *securityapi.CertificateResponse, err error) {
	remotePeer, ok := peer.FromContext(ctx)
	if !ok {
		return nil, status.Error(codes.PermissionDenied, "peer not found")
	}

	osRoot, err := safe.StateGet[*secrets.OSRoot](ctx, r.Resources, resource.NewMetadata(secrets.NamespaceName, secrets.OSRootType, secrets.OSRootID, resource.VersionUndefined))
	if err != nil {
		return nil, err
	}

	// decode and validate CSR
	csrPemBlock, _ := pem.Decode(in.Csr)
	if csrPemBlock == nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to decode CSR")
	}

	request, err := stdx509.ParseCertificateRequest(csrPemBlock.Bytes)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to parse CSR: %s", err)
	}

	log.Printf("received CSR signing request from %s: subject %s dns names %s addresses %s", remotePeer.Addr, request.Subject, request.DNSNames, request.IPAddresses)

	// allow only server auth certificates
	x509Opts := []x509.Option{
		x509.KeyUsage(stdx509.KeyUsageDigitalSignature),
		x509.ExtKeyUsage([]stdx509.ExtKeyUsage{stdx509.ExtKeyUsageServerAuth}),
	}

	// don't allow any certificates which can be used for client authentication
	//
	// we don't return an error here, as otherwise workers running old versions of Talos
	// will fail to provision client certificate and will never launch apid
	//
	// instead, the returned certificate will be rejected when being used
	if len(request.Subject.Organization) > 0 {
		log.Printf("removing client auth organization from CSR: %s", request.Subject.Organization)

		x509Opts = append(x509Opts, x509.OverrideSubject(func(subject *pkix.Name) {
			subject.Organization = nil
		}))
	}

	// TODO: Verify that the request is coming from the IP address declared in
	// the CSR.
	signed, err := x509.NewCertificateFromCSRBytes(
		osRoot.TypedSpec().CA.Crt,
		osRoot.TypedSpec().CA.Key,
		in.Csr,
		x509Opts...,
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to sign CSR: %s", err)
	}

	resp = &securityapi.CertificateResponse{
		Ca:  osRoot.TypedSpec().CA.Crt,
		Crt: signed.X509CertificatePEM,
	}

	return resp, nil
}
