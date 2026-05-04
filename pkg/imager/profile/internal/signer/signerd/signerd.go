// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package signerd adapts a gRPC SignerService into measure.RSAKey and
// pesign.CertificateSigner.
package signerd

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/siderolabs/talos/internal/pkg/measure"
	"github.com/siderolabs/talos/internal/pkg/secureboot/pesign"
	"github.com/siderolabs/talos/pkg/machinery/api/signer"
)

type rsaSigner struct {
	conn   *grpc.ClientConn
	client signer.SignerServiceClient
	pubKey *rsa.PublicKey
}

func dial(address string) (*grpc.ClientConn, error) {
	if !strings.HasPrefix(address, "unix://") {
		return nil, fmt.Errorf("signer address %q must use unix:// scheme", address)
	}

	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial signer at %s: %w", address, err)
	}

	return conn, nil
}

func (s *rsaSigner) Public() crypto.PublicKey { return s.pubKey }

func (s *rsaSigner) Sign(_ io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	hash, err := mapHash(opts)
	if err != nil {
		return nil, err
	}

	scheme := signer.Scheme_SCHEME_RSA_PKCS1V15
	if _, ok := opts.(*rsa.PSSOptions); ok {
		scheme = signer.Scheme_SCHEME_RSA_PSS
	}

	resp, err := s.client.Sign(context.Background(), &signer.SignRequest{
		Digest: digest,
		Hash:   hash,
		Scheme: scheme,
	})
	if err != nil {
		return nil, fmt.Errorf("signerd Sign: %w", err)
	}

	return resp.Signature, nil
}

// PCRSigner implements measure.RSAKey.
type PCRSigner struct {
	rsaSigner
}

// Verify interface.
var _ measure.RSAKey = (*PCRSigner)(nil)

// NewPCRSigner creates a new PCRSigner.
func NewPCRSigner(ctx context.Context, address string) (s *PCRSigner, err error) {
	conn, err := dial(address)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err != nil {
			conn.Close() //nolint:errcheck
		}
	}()

	client := signer.NewSignerServiceClient(conn)

	resp, err := client.GetPublicKey(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, fmt.Errorf("signerd GetPublicKey: %w", err)
	}

	pubKey, err := parseRSAPublicKey(resp.PubKeyPem)
	if err != nil {
		return nil, err
	}

	return &PCRSigner{
		rsaSigner: rsaSigner{conn: conn, client: client, pubKey: pubKey},
	}, nil
}

// PublicRSAKey returns the public key.
func (s *PCRSigner) PublicRSAKey() *rsa.PublicKey { return s.pubKey }

// Close releases the gRPC connection.
func (s *PCRSigner) Close() error { return s.conn.Close() }

// SecureBootSigner implements pesign.CertificateSigner.
type SecureBootSigner struct {
	rsaSigner

	cert *x509.Certificate
}

// Verify interface.
var _ pesign.CertificateSigner = (*SecureBootSigner)(nil)

// NewSecureBootSigner creates a new SecureBootSigner.
func NewSecureBootSigner(ctx context.Context, address string) (s *SecureBootSigner, err error) {
	conn, err := dial(address)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err != nil {
			conn.Close() //nolint:errcheck
		}
	}()

	client := signer.NewSignerServiceClient(conn)

	resp, err := client.GetCertificate(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, fmt.Errorf("signerd GetCertificate: %w", err)
	}

	cert, err := parseCertificate(resp.CertPem)
	if err != nil {
		return nil, err
	}

	pubKey, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("signer certificate public key is not RSA (got %T)", cert.PublicKey)
	}

	return &SecureBootSigner{
		rsaSigner: rsaSigner{conn: conn, client: client, pubKey: pubKey},
		cert:      cert,
	}, nil
}

// Signer returns the signer.
func (s *SecureBootSigner) Signer() crypto.Signer { return &s.rsaSigner }

// Certificate returns the certificate.
func (s *SecureBootSigner) Certificate() *x509.Certificate { return s.cert }

// Close releases the gRPC connection.
func (s *SecureBootSigner) Close() error { return s.conn.Close() }

func mapHash(opts crypto.SignerOpts) (signer.Hash, error) {
	hf := crypto.SHA256

	if opts != nil {
		hf = opts.HashFunc()
	}

	switch hf { //nolint:exhaustive
	case crypto.SHA256:
		return signer.Hash_HASH_SHA256, nil
	case crypto.SHA384:
		return signer.Hash_HASH_SHA384, nil
	case crypto.SHA512:
		return signer.Hash_HASH_SHA512, nil
	default:
		return signer.Hash_HASH_UNSPECIFIED, fmt.Errorf("unsupported hash function %v", hf)
	}
}

func parseRSAPublicKey(pemBytes []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("failed to decode PEM public key")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse PKIX public key: %w", err)
	}

	rsaKey, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("public key is not RSA (got %T)", pub)
	}

	return rsaKey, nil
}

func parseCertificate(pemBytes []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("failed to decode PEM certificate")
	}

	return x509.ParseCertificate(block.Bytes)
}
