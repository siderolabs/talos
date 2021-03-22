// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package reg

import (
	"context"
	"io/ioutil"
	"log"
	"os"
	"path"

	"github.com/talos-systems/crypto/x509"
	"google.golang.org/grpc"

	securityapi "github.com/talos-systems/talos/pkg/machinery/api/security"
	"github.com/talos-systems/talos/pkg/machinery/config"
)

// Registrator is the concrete type that implements the factory.Registrator and
// securityapi.SecurityServiceServer interfaces.
type Registrator struct {
	securityapi.UnimplementedSecurityServiceServer

	Config config.Provider
}

// Register implements the factory.Registrator interface.
//
//nolint:interfacer
func (r *Registrator) Register(s *grpc.Server) {
	securityapi.RegisterSecurityServiceServer(s, r)
}

// Certificate implements the securityapi.SecurityServer interface.
func (r *Registrator) Certificate(ctx context.Context, in *securityapi.CertificateRequest) (resp *securityapi.CertificateResponse, err error) {
	// TODO: Verify that the request is coming from the IP addresss declared in
	// the CSR.
	signed, err := x509.NewCertificateFromCSRBytes(r.Config.Machine().Security().CA().Crt, r.Config.Machine().Security().CA().Key, in.Csr)
	if err != nil {
		return
	}

	resp = &securityapi.CertificateResponse{
		Ca:  r.Config.Machine().Security().CA().Crt,
		Crt: signed.X509CertificatePEM,
	}

	return resp, nil
}

// ReadFile implements the securityapi.SecurityServer interface.
func (r *Registrator) ReadFile(ctx context.Context, in *securityapi.ReadFileRequest) (resp *securityapi.ReadFileResponse, err error) {
	var b []byte

	if b, err = ioutil.ReadFile(in.Path); err != nil {
		return nil, err
	}

	log.Printf("read file on disk: %s", in.Path)

	resp = &securityapi.ReadFileResponse{Data: b}

	return resp, nil
}

// WriteFile implements the securityapi.SecurityServer interface.
func (r *Registrator) WriteFile(ctx context.Context, in *securityapi.WriteFileRequest) (resp *securityapi.WriteFileResponse, err error) {
	if err = os.MkdirAll(path.Dir(in.Path), os.ModeDir); err != nil {
		return
	}

	if err = ioutil.WriteFile(in.Path, in.Data, os.FileMode(in.Perm)); err != nil {
		return
	}

	log.Printf("wrote file to disk: %s", in.Path)

	resp = &securityapi.WriteFileResponse{}

	return resp, nil
}
