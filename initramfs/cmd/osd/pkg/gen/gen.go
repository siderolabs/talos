package gen

import (
	"context"
	stdlibx509 "crypto/x509"
	"encoding/pem"
	"fmt"
	"log"
	stdlibnet "net"
	"time"

	"github.com/autonomy/dianemo/initramfs/cmd/rotd/proto"
	"github.com/autonomy/dianemo/initramfs/pkg/crypto/x509"
	"github.com/autonomy/dianemo/initramfs/pkg/net"
	"github.com/autonomy/dianemo/initramfs/pkg/userdata"
	"google.golang.org/grpc"
)

// Generator represents the OS identity generator.
type Generator struct {
	client proto.ROTDClient
}

// NewGenerator initializes a Generator with a preconfigured grpc.ClientConn.
func NewGenerator(conn *grpc.ClientConn) (g *Generator) {
	client := proto.NewROTDClient(conn)

	return &Generator{
		client: client,
	}
}

// Certificate implements the proto.ROTDClient interface.
func (g *Generator) Certificate(in *proto.CertificateRequest) (resp *proto.CertificateResponse, err error) {
	ctx := context.Background()
	resp, err = g.client.Certificate(ctx, in)
	if err != nil {
		return
	}

	return resp, err
}

// Identity creates a CSR and sends it to a Root of Trust for signing.
// The Root of Trust responds with a signed certificate.
func (g *Generator) Identity(data *userdata.Security) (err error) {
	key, err := x509.NewKey()
	if err != nil {
		return
	}

	data.Identity = &userdata.PEMEncodedCertificateAndKey{}
	data.Identity.Key = key.KeyPEM

	pemBlock, _ := pem.Decode(key.KeyPEM)
	if pemBlock == nil {
		return fmt.Errorf("failed to decode key")
	}
	keyEC, err := stdlibx509.ParseECPrivateKey(pemBlock.Bytes)
	if err != nil {
		return
	}
	addr, err := net.IP()
	if err != nil {
		return
	}
	opts := []x509.Option{}
	ips := []stdlibnet.IP{addr}
	opts = append(opts, x509.IPAddresses(ips))
	opts = append(opts, x509.NotAfter(time.Now().Add(time.Duration(8760)*time.Hour)))
	csr, err := x509.NewCertificateSigningRequest(keyEC, opts...)
	if err != nil {
		return
	}
	req := &proto.CertificateRequest{
		Csr: csr.X509CertificateRequestPEM,
	}

	return poll(g, req, data.Identity)
}

func poll(g *Generator, in *proto.CertificateRequest, data *userdata.PEMEncodedCertificateAndKey) (err error) {
	timeout := time.NewTimer(time.Minute * 5).C
	tick := time.NewTicker(time.Second * 5).C

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for certificate")
		case <-tick:
			crt, _err := g.Certificate(in)
			if _err != nil {
				log.Println(_err)
				continue
			}
			data.Crt = crt.Bytes

			return nil
		}
	}
}
