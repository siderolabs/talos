// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubeprism_test

import (
	"context"
	"encoding/pem"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/siderolabs/crypto/x509"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/k8s/internal/kubeprism"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// newAPIServer starts a TLS server which serves /readyz with the given status.
//
// The server certificate is valid for 127.0.0.1 and example.com.
func newAPIServer(t *testing.T, status *atomic.Int32) (*httptest.Server, *x509.PEMEncodedCertificate) {
	t.Helper()

	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/readyz" {
			w.WriteHeader(http.StatusNotFound)

			return
		}

		code := int(status.Load())

		w.WriteHeader(code)

		if code == http.StatusOK {
			w.Write([]byte("ok")) //nolint:errcheck
		} else {
			w.Write([]byte("[-]etcd failed: reason withheld\nreadyz check failed\n")) //nolint:errcheck
		}
	}))
	srv.StartTLS()
	t.Cleanup(srv.Close)

	return srv, &x509.PEMEncodedCertificate{
		Crt: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw}),
	}
}

func newChecker(t *testing.T, ca *x509.PEMEncodedCertificate, serverName string) *kubeprism.ReadinessChecker {
	t.Helper()

	var checker kubeprism.ReadinessChecker

	require.NoError(t, checker.Update([]*x509.PEMEncodedCertificate{ca}, serverName))

	return &checker
}

func otherCA(t *testing.T) *x509.PEMEncodedCertificate {
	t.Helper()

	ca, err := x509.NewSelfSignedCertificateAuthority(x509.ECDSA(true))
	require.NoError(t, err)

	return &x509.PEMEncodedCertificate{Crt: ca.CrtPEM}
}

func TestReadinessCheck(t *testing.T) {
	t.Parallel()

	var status atomic.Int32

	status.Store(http.StatusOK)

	srv, ca := newAPIServer(t, &status)
	address := srv.Listener.Addr().String()

	checker := newChecker(t, ca, "127.0.0.1")

	require.NoError(t, checker.Check(t.Context(), address))

	status.Store(http.StatusInternalServerError)

	err := checker.Check(t.Context(), address)
	require.Error(t, err)
	assert.ErrorContains(t, err, "500 Internal Server Error")
	assert.ErrorContains(t, err, "etcd failed")

	status.Store(http.StatusUnauthorized)

	assert.ErrorContains(t, checker.Check(t.Context(), address), "401 Unauthorized")

	// the TLS configuration can be updated on the fly
	status.Store(http.StatusOK)

	require.NoError(t, checker.Update([]*x509.PEMEncodedCertificate{otherCA(t), ca}, "127.0.0.1"))
	require.NoError(t, checker.Check(t.Context(), address))
}

func TestReadinessCheckTLSVerification(t *testing.T) {
	t.Parallel()

	var status atomic.Int32

	status.Store(http.StatusOK)

	srv, ca := newAPIServer(t, &status)
	address := srv.Listener.Addr().String()

	for _, test := range []struct {
		name       string
		ca         *x509.PEMEncodedCertificate
		serverName string

		expectedErr string
	}{
		{
			name:       "server name override",
			ca:         ca,
			serverName: "example.com",
		},
		{
			name:        "wrong server name",
			ca:          ca,
			serverName:  "kubernetes",
			expectedErr: "certificate is valid for",
		},
		{
			name:        "wrong CA",
			ca:          otherCA(t),
			serverName:  "127.0.0.1",
			expectedErr: "certificate signed by unknown authority",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := newChecker(t, test.ca, test.serverName).Check(t.Context(), address)

			if test.expectedErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, test.expectedErr)
			}
		})
	}
}

func TestReadinessCheckNotConfigured(t *testing.T) {
	t.Parallel()

	var checker kubeprism.ReadinessChecker

	require.ErrorContains(t, checker.Check(t.Context(), "127.0.0.1:6443"), "not configured")
	require.Error(t, checker.Update(nil, "127.0.0.1"))
	require.Error(t, checker.Update([]*x509.PEMEncodedCertificate{{Crt: []byte("garbage")}}, "127.0.0.1"))
}

func TestReadinessCheckTimeout(t *testing.T) {
	t.Parallel()

	t.Run("stuck handshake", func(t *testing.T) {
		t.Parallel()

		// a TCP listener which accepts connections, but never responds
		lis, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
		require.NoError(t, err)

		t.Cleanup(func() { lis.Close() }) //nolint:errcheck

		go func() {
			for {
				conn, err := lis.Accept()
				if err != nil {
					return
				}

				t.Cleanup(func() { conn.Close() }) //nolint:errcheck
			}
		}()

		checker := newChecker(t, otherCA(t), "127.0.0.1")

		ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
		t.Cleanup(cancel)

		start := time.Now()

		require.ErrorIs(t, checker.Check(ctx, lis.Addr().String()), context.DeadlineExceeded)
		assert.Less(t, time.Since(start), 5*time.Second)
	})

	t.Run("stuck response", func(t *testing.T) {
		t.Parallel()

		unblock := make(chan struct{})

		srv := httptest.NewUnstartedServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			<-unblock
		}))
		srv.StartTLS()

		t.Cleanup(srv.Close)
		t.Cleanup(func() { close(unblock) })

		checker := newChecker(t, &x509.PEMEncodedCertificate{
			Crt: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw}),
		}, "127.0.0.1")

		ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
		t.Cleanup(cancel)

		start := time.Now()

		require.ErrorIs(t, checker.Check(ctx, srv.Listener.Addr().String()), context.DeadlineExceeded)
		assert.Less(t, time.Since(start), 5*time.Second)
	})
}

func TestReadinessCheckNoLeaks(t *testing.T) {
	// not parallel: counts file descriptors of the process
	var status atomic.Int32

	status.Store(http.StatusOK)

	srv, ca := newAPIServer(t, &status)
	address := srv.Listener.Addr().String()

	goodChecker := newChecker(t, ca, "127.0.0.1")
	badChecker := newChecker(t, otherCA(t), "127.0.0.1")

	countFDs := func() int {
		entries, err := os.ReadDir("/proc/self/fd")
		require.NoError(t, err)

		return len(entries)
	}

	// warm up (e.g. lazy initialization in the standard library)
	require.NoError(t, goodChecker.Check(t.Context(), address))

	initialFDs := countFDs()

	for i := range 100 {
		switch i % 4 {
		case 0:
			status.Store(http.StatusOK)

			require.NoError(t, goodChecker.Check(t.Context(), address))
		case 1:
			status.Store(http.StatusInternalServerError)

			require.Error(t, goodChecker.Check(t.Context(), address))
		case 2:
			require.Error(t, badChecker.Check(t.Context(), address))
		case 3:
			ctx, cancel := context.WithCancel(t.Context())
			cancel()

			require.Error(t, goodChecker.Check(ctx, address))
		}
	}

	// client side of each connection is closed when Check returns, the server side might lag a bit
	assert.Eventually(t, func() bool {
		return countFDs() <= initialFDs
	}, 5*time.Second, 10*time.Millisecond, "file descriptors leaked: initial %d, current %d", initialFDs, countFDs())
}
