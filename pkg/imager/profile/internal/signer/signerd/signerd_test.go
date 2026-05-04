// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package signerd_test

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/siderolabs/talos/pkg/imager/profile/internal/signer/signerd"
	"github.com/siderolabs/talos/pkg/machinery/api/signer"
)

// fixture holds the in-memory key/cert pair the fake server returns and uses
// to produce real signatures.
type fixture struct {
	key     *rsa.PrivateKey
	cert    *x509.Certificate
	certPEM []byte
	pubPEM  []byte
}

func newFixture(t *testing.T) fixture {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)

	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)

	pubDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	require.NoError(t, err)

	return fixture{
		key:     key,
		cert:    cert,
		certPEM: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pubPEM:  pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER}),
	}
}

// fakeServer implements signer.SignerServiceServer with the fixture's key+cert.
// Sign requests produce real RSA-PKCS1v15 signatures so client-side tests
// can verify against the fixture pubkey.
type fakeServer struct {
	signer.UnimplementedSignerServiceServer

	fx fixture
}

func (s *fakeServer) GetCertificate(_ context.Context, _ *emptypb.Empty) (*signer.CertificateResponse, error) {
	return &signer.CertificateResponse{CertPem: s.fx.certPEM}, nil
}

func (s *fakeServer) GetPublicKey(_ context.Context, _ *emptypb.Empty) (*signer.PublicKeyResponse, error) {
	return &signer.PublicKeyResponse{PubKeyPem: s.fx.pubPEM}, nil
}

func (s *fakeServer) Sign(_ context.Context, req *signer.SignRequest) (*signer.SignResponse, error) {
	hash, err := mapHash(req.Hash)
	if err != nil {
		return nil, err
	}

	var sig []byte

	switch req.Scheme { //nolint:exhaustive
	case signer.Scheme_SCHEME_RSA_PSS:
		sig, err = rsa.SignPSS(rand.Reader, s.fx.key, hash, req.Digest, nil)
		if err != nil {
			return nil, err
		}
	default:
		sig, err = rsa.SignPKCS1v15(rand.Reader, s.fx.key, hash, req.Digest)
		if err != nil {
			return nil, err
		}
	}

	return &signer.SignResponse{Signature: sig}, nil
}

func mapHash(h signer.Hash) (crypto.Hash, error) {
	switch h {
	case signer.Hash_HASH_SHA256, signer.Hash_HASH_UNSPECIFIED:
		return crypto.SHA256, nil
	case signer.Hash_HASH_SHA384:
		return crypto.SHA384, nil
	case signer.Hash_HASH_SHA512:
		return crypto.SHA512, nil
	default:
		return 0, fmt.Errorf("unsupported hash %v", h)
	}
}

// startTestServer launches a fakeServer on a Unix socket inside t.TempDir().
// It returns the address (in the form expected by signerd.NewPCRSigner et al)
// and registers cleanup that stops the server.
func startTestServer(t *testing.T, fx fixture) string {
	t.Helper()

	socketPath := filepath.Join(t.TempDir(), "signer.sock")

	var lc net.ListenConfig

	lis, err := lc.Listen(t.Context(), "unix", socketPath)
	require.NoError(t, err)

	grpcSrv := grpc.NewServer()
	signer.RegisterSignerServiceServer(grpcSrv, &fakeServer{fx: fx})

	var wg sync.WaitGroup

	wg.Go(func() {
		if err := grpcSrv.Serve(lis); err != nil {
			t.Errorf("grpc serve: %v", err)
		}
	})

	t.Cleanup(func() {
		grpcSrv.Stop()
		wg.Wait()
	})

	return "unix://" + socketPath
}

func sha256Of(b []byte) [32]byte {
	h := crypto.SHA256.New()
	h.Write(b)

	var out [32]byte

	copy(out[:], h.Sum(nil))

	return out
}

func TestPCRSigner(t *testing.T) {
	fx := newFixture(t)
	addr := startTestServer(t, fx)

	s, err := signerd.NewPCRSigner(t.Context(), addr)
	require.NoError(t, err)

	t.Cleanup(func() { require.NoError(t, s.Close()) })

	require.Equal(t, fx.key.PublicKey.N, s.PublicRSAKey().N)
	require.Equal(t, fx.key.PublicKey.E, s.PublicRSAKey().E)

	digest := sha256Of([]byte("hello"))

	sig, err := s.Sign(rand.Reader, digest[:], crypto.SHA256)
	require.NoError(t, err)
	require.NoError(t, rsa.VerifyPKCS1v15(&fx.key.PublicKey, crypto.SHA256, digest[:], sig))
}

func TestSecureBootSigner(t *testing.T) {
	fx := newFixture(t)
	addr := startTestServer(t, fx)

	s, err := signerd.NewSecureBootSigner(t.Context(), addr)
	require.NoError(t, err)

	t.Cleanup(func() { require.NoError(t, s.Close()) })

	require.Equal(t, fx.cert.SerialNumber, s.Certificate().SerialNumber)
	require.Equal(t, fx.cert.Subject.CommonName, s.Certificate().Subject.CommonName)

	digest := sha256Of([]byte("pe binary"))

	sig, err := s.Signer().Sign(rand.Reader, digest[:], crypto.SHA256)
	require.NoError(t, err)
	require.NoError(t, rsa.VerifyPKCS1v15(&fx.key.PublicKey, crypto.SHA256, digest[:], sig))
}

func TestPCRSignerDialFailure(t *testing.T) {
	_, err := signerd.NewPCRSigner(t.Context(), "unix:///does/not/exist.sock")
	require.Error(t, err)
}

func TestPCRSignerRejectsTCP(t *testing.T) {
	_, err := signerd.NewPCRSigner(t.Context(), "tcp://localhost:5000")
	require.Error(t, err)
	require.Contains(t, err.Error(), "unix://")
}

func TestSecureBootSignerRejectsTCP(t *testing.T) {
	_, err := signerd.NewSecureBootSigner(t.Context(), "tcp://localhost:5000")
	require.Error(t, err)
	require.Contains(t, err.Error(), "unix://")
}
