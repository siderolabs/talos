// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubeprism

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/siderolabs/go-loadbalancer/upstream"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func TestHealthCheckVerifiesClusterCAAndReadiness(t *testing.T) {
	t.Parallel()

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/readyz" {
			w.WriteHeader(http.StatusBadRequest)

			return
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	checker := &healthChecker{logger: zap.NewNop()}
	require.NoError(t, checker.SetKubeconfig(makeKubeconfig(t, server.Certificate().Raw, nil, nil)))

	address := server.Listener.Addr().String()
	require.NoError(t, checker.check(t.Context(), address))

	require.NoError(t, checker.SetKubeconfig(makeKubeconfig(t, makeTestCA(t), nil, nil)))
	require.ErrorContains(t, checker.check(t.Context(), address), "certificate")

	require.ErrorContains(
		t,
		checker.SetKubeconfig(makeKubeconfig(t, []byte("not a CA"), nil, nil)),
		"invalid cluster CA",
	)
}

func TestHealthCheckRetriesWithClientCertificate(t *testing.T) {
	t.Parallel()

	clientCert, clientKey := makeClientCertificate(t)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
			w.WriteHeader(http.StatusForbidden)

			return
		}

		w.WriteHeader(http.StatusOK)
	}))
	server.TLS = &tls.Config{ClientAuth: tls.RequireAnyClientCert}
	server.Config.ErrorLog = log.New(io.Discard, "", 0)
	server.StartTLS()
	defer server.Close()

	checker := &healthChecker{logger: zap.NewNop()}
	require.NoError(t, checker.SetKubeconfig(makeKubeconfig(t, server.Certificate().Raw, clientCert, clientKey)))
	require.NoError(t, checker.check(t.Context(), server.Listener.Addr().String()))
}

func TestLoadBalancerFallsBackToTCPHealthyUpstreams(t *testing.T) {
	t.Parallel()

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/readyz" {
			w.WriteHeader(http.StatusServiceUnavailable)

			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "fallback")
	}))
	defer server.Close()

	kubeconfig := makeKubeconfig(t, server.Certificate().Raw, nil, nil)
	loadBalancer := NewLoadBalancer(
		freeAddress(t),
		zap.NewNop(),
		time.Second,
		time.Second,
		0,
		upstream.WithHealthcheckInterval(10*time.Millisecond),
		upstream.WithHealthcheckTimeout(time.Second),
	)
	require.NoError(t, loadBalancer.SetKubeconfig(kubeconfig))
	require.NoError(t, loadBalancer.Start([]string{server.Listener.Addr().String()}))
	defer func() {
		require.NoError(t, loadBalancer.Shutdown())
	}()

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()

	require.NoError(t, loadBalancer.l7Upstreams.WaitForInitialHealthcheck(ctx))
	require.NoError(t, loadBalancer.tcpUpstreams.WaitForInitialHealthcheck(ctx))

	require.Eventually(t, func() bool {
		_, err := loadBalancer.l7Upstreams.Pick()

		return err == upstream.ErrNoUpstreams
	}, 2*time.Second, 10*time.Millisecond)

	healthy, err := loadBalancer.Healthy()
	require.NoError(t, err)
	require.True(t, healthy, "TCP fallback should keep the route available")

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{RootCAs: trustedCertPool(t, server.Certificate().Raw)},
	}
	defer transport.CloseIdleConnections()

	client := &http.Client{Transport: transport, Timeout: time.Second}
	response, err := client.Get("https://" + loadBalancer.endpoint + "/fallback")
	require.NoError(t, err)
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.Equal(t, "fallback", string(body))
}

func TestConnectionTrackerDrainsAfterRepeatedHealthCheckFailures(t *testing.T) {
	t.Parallel()

	tracker := newConnectionTracker()
	clientA, serverA := net.Pipe()
	clientB, serverB := net.Pipe()
	defer serverA.Close()
	defer serverB.Close()

	tracked := tracker.add("upstream", clientA, clientB)
	defer tracker.remove("upstream", tracked)

	for range drainFailureThreshold - 1 {
		require.False(t, tracker.recordFailure("upstream"))
	}

	require.True(t, tracker.recordFailure("upstream"))
	tracker.drain("upstream")

	require.Eventually(t, func() bool {
		_ = serverA.SetReadDeadline(time.Now().Add(time.Millisecond))
		_, err := serverA.Read(make([]byte, 1))

		return err != nil
	}, time.Second, time.Millisecond)
}

func makeKubeconfig(t *testing.T, ca, cert, key []byte) string {
	t.Helper()

	config := &clientcmdapi.Config{
		CurrentContext: "test",
		Clusters: map[string]*clientcmdapi.Cluster{
			"test": {
				Server:                   "https://127.0.0.1:6443",
				CertificateAuthorityData: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ca}),
			},
		},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			"test": {
				ClientCertificateData: cert,
				ClientKeyData:         key,
			},
		},
		Contexts: map[string]*clientcmdapi.Context{
			"test": {
				Cluster:  "test",
				AuthInfo: "test",
			},
		},
	}

	data, err := clientcmd.Write(*config)
	require.NoError(t, err)

	return string(data)
}

func makeClientCertificate(t *testing.T) ([]byte, []byte) {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "health-check"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	certificate, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	require.NoError(t, err)

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate}),
		pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})
}

func makeTestCA(t *testing.T) []byte {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(2),
		Subject:               pkix.Name{CommonName: "unrelated-test-ca"},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}

	certificate, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	require.NoError(t, err)

	return certificate
}

func trustedCertPool(t *testing.T, certificate []byte) *x509.CertPool {
	t.Helper()

	pool := x509.NewCertPool()
	require.True(t, pool.AppendCertsFromPEM(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate})))

	return pool
}

func freeAddress(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	address := listener.Addr().String()
	require.NoError(t, listener.Close())

	return address
}
