package reg

import (
	"context"
	stdlibx509 "crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"time"

	"github.com/autonomy/talos/internal/app/trustd/proto"
	"github.com/autonomy/talos/internal/pkg/crypto/x509"
	"github.com/autonomy/talos/internal/pkg/userdata"
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
	signed, err := GenerateCertificateFromCSR(r.Data.CA.Crt, r.Data.CA.Key, in.Csr)
	if err != nil {
		return
	}

	resp = &proto.CertificateResponse{
		Bytes: signed.X509CertificatePEM,
	}

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

// GenerateCertificateFromCSR creates a signed certificate using the provided
// certificate, key, and CSR.
func GenerateCertificateFromCSR(crt, key, csr []byte) (signed *x509.Certificate, err error) {
	caPemBlock, _ := pem.Decode(crt)
	if caPemBlock == nil {
		return nil, fmt.Errorf("decode PEM: %v", err)
	}
	caCrt, err := stdlibx509.ParseCertificate(caPemBlock.Bytes)
	if err != nil {
		return
	}
	keyPemBlock, _ := pem.Decode(key)
	if keyPemBlock == nil {
		return nil, fmt.Errorf("decode PEM: %v", err)
	}
	caKey, err := stdlibx509.ParseECPrivateKey(keyPemBlock.Bytes)
	if err != nil {
		return
	}
	csrPemBlock, _ := pem.Decode(csr)
	if csrPemBlock == nil {
		return
	}
	request, err := stdlibx509.ParseCertificateRequest(csrPemBlock.Bytes)
	if err != nil {
		return
	}
	signed, err = x509.NewCertificateFromCSR(caCrt, caKey, request, x509.NotAfter(time.Now().Add(time.Duration(8760)*time.Hour)))
	if err != nil {
		return
	}

	return signed, nil
}
