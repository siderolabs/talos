// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package reg

import (
	"context"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/crypto/x509"
	"google.golang.org/grpc"

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
func (r *Registrator) Certificate(ctx context.Context, in *securityapi.CertificateRequest) (resp *securityapi.CertificateResponse, err error) {
	osRoot, err := safe.StateGet[*secrets.OSRoot](ctx, r.Resources, resource.NewMetadata(secrets.NamespaceName, secrets.OSRootType, secrets.OSRootID, resource.VersionUndefined))
	if err != nil {
		return nil, err
	}

	// TODO: Verify that the request is coming from the IP addresss declared in
	// the CSR.
	signed, err := x509.NewCertificateFromCSRBytes(osRoot.TypedSpec().CA.Crt, osRoot.TypedSpec().CA.Key, in.Csr)
	if err != nil {
		return
	}

	resp = &securityapi.CertificateResponse{
		Ca:  osRoot.TypedSpec().CA.Crt,
		Crt: signed.X509CertificatePEM,
	}

	return resp, nil
}
