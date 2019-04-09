/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package reg

import (
	"context"
	"io/ioutil"
	"log"
	"os"
	"path"

	"github.com/talos-systems/talos/internal/app/trustd/proto"
	"github.com/talos-systems/talos/pkg/crypto/x509"
	"github.com/talos-systems/talos/pkg/userdata"
	"google.golang.org/grpc"
)

// Registrator is the concrete type that implements the factory.Registrator and
// proto.TrustdServer interfaces.
type Registrator struct {
	Data *userdata.OSSecurity
}

// Register implements the factory.Registrator interface.
func (r *Registrator) Register(s *grpc.Server) {
	proto.RegisterTrustdServer(s, r)
}

// Certificate implements the proto.TrustdServer interface.
func (r *Registrator) Certificate(ctx context.Context, in *proto.CertificateRequest) (resp *proto.CertificateResponse, err error) {
	// TODO: Verify that the request is coming from the IP addresss declared in
	// the CSR.
	signed, err := x509.NewCertificateFromCSRBytes(r.Data.CA.Crt, r.Data.CA.Key, in.Csr)
	if err != nil {
		return
	}

	resp = &proto.CertificateResponse{
		Ca:  r.Data.CA.Crt,
		Crt: signed.X509CertificatePEM,
	}

	return resp, nil
}

// ReadFile implements the proto.TrustdServer interface.
func (r *Registrator) ReadFile(ctx context.Context, in *proto.ReadFileRequest) (resp *proto.ReadFileResponse, err error) {
	var b []byte
	if b, err = ioutil.ReadFile(in.Path); err != nil {
		return nil, err
	}

	log.Printf("read file on disk: %s", in.Path)
	resp = &proto.ReadFileResponse{Data: b}

	return resp, nil
}

// WriteFile implements the proto.TrustdServer interface.
func (r *Registrator) WriteFile(ctx context.Context, in *proto.WriteFileRequest) (resp *proto.WriteFileResponse, err error) {
	if err = os.MkdirAll(path.Dir(in.Path), os.ModeDir); err != nil {
		return
	}
	if err = ioutil.WriteFile(in.Path, in.Data, os.FileMode(in.Perm)); err != nil {
		return
	}

	log.Printf("wrote file to disk: %s", in.Path)
	resp = &proto.WriteFileResponse{}

	return resp, nil
}
