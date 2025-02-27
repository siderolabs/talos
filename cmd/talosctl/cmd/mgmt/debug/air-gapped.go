// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package debug

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"embed"
	"encoding/pem"
	"fmt"
	"io"
	"io/fs"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/siderolabs/talos/pkg/cli"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
)

//go:embed httproot/*
var httpFs embed.FS

var airgappedFlags struct {
	advertisedAddress net.IP
	proxyPort         int
	httpsPort         int
}

// airgappedCmd represents the `gen ca` command.
var airgappedCmd = &cobra.Command{
	Use:   "air-gapped",
	Short: "Starts a local HTTP proxy and HTTPS server serving a test manifest.",
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cli.WithContext(
			context.Background(), func(ctx context.Context) error {
				certPEM, keyPEM, err := generateSelfSignedCert()
				if err != nil {
					return nil
				}

				if err = generateConfigPatch(certPEM); err != nil {
					return err
				}

				eg, ctx := errgroup.WithContext(ctx)

				eg.Go(func() error { return runHTTPServer(ctx, certPEM, keyPEM) })
				eg.Go(func() error { return runHTTPProxy(ctx) })

				return eg.Wait()
			},
		)
	},
}

func generateConfigPatch(caPEM []byte) error {
	patch := &v1alpha1.Config{
		MachineConfig: &v1alpha1.MachineConfig{
			MachineEnv: map[string]string{
				"http_proxy":  fmt.Sprintf("http://%s", net.JoinHostPort(airgappedFlags.advertisedAddress.String(), strconv.Itoa(airgappedFlags.proxyPort))),
				"https_proxy": fmt.Sprintf("http://%s", net.JoinHostPort(airgappedFlags.advertisedAddress.String(), strconv.Itoa(airgappedFlags.proxyPort))),
				"no_proxy":    fmt.Sprintf("%s/24", airgappedFlags.advertisedAddress.String()),
			},
			MachineFiles: []*v1alpha1.MachineFile{
				{
					FilePath:        "/etc/ssl/certs/ca-certificates",
					FileContent:     string(caPEM),
					FilePermissions: 0o644,
					FileOp:          "append",
				},
			},
		},
		ClusterConfig: &v1alpha1.ClusterConfig{
			ExtraManifests: []string{
				fmt.Sprintf("https://%s/debug.yaml", net.JoinHostPort(airgappedFlags.advertisedAddress.String(), strconv.Itoa(airgappedFlags.httpsPort))),
			},
		},
	}

	patchBytes, err := encoder.NewEncoder(patch, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	if err != nil {
		return err
	}

	const patchFile = "air-gapped-patch.yaml"

	log.Printf("writing config patch to %s", patchFile)

	return os.WriteFile(patchFile, patchBytes, 0o644)
}

func generateSelfSignedCert() ([]byte, []byte, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}

	template := x509.Certificate{
		SerialNumber:       big.NewInt(1),
		SignatureAlgorithm: x509.ECDSAWithSHA256,
		Subject: pkix.Name{
			Organization: []string{"Test Only"},
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(time.Hour * 24),

		KeyUsage: x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
			x509.ExtKeyUsageClientAuth,
		},
		BasicConstraintsValid: true,

		IsCA: true,

		IPAddresses: []net.IP{airgappedFlags.advertisedAddress},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return nil, nil, err
	}

	var crt bytes.Buffer

	if err = pem.Encode(&crt, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return nil, nil, err
	}

	var key bytes.Buffer

	keyBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return nil, nil, err
	}

	if err = pem.Encode(&key, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes}); err != nil {
		return nil, nil, err
	}

	return crt.Bytes(), key.Bytes(), nil
}

func runHTTPServer(ctx context.Context, certPEM, keyPEM []byte) error {
	certificate, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return err
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{certificate},
	}

	subFs, err := fs.Sub(httpFs, "httproot")
	if err != nil {
		return err
	}

	srv := &http.Server{
		Addr:      net.JoinHostPort("", strconv.Itoa(airgappedFlags.httpsPort)),
		Handler:   loggingMiddleware(http.FileServer(http.FS(subFs))),
		TLSConfig: tlsConfig,
	}

	log.Printf("starting HTTPS server with self-signed cert on %s", srv.Addr)

	go srv.ListenAndServeTLS("", "") //nolint:errcheck

	<-ctx.Done()

	return srv.Close()
}

func handleTunneling(w http.ResponseWriter, r *http.Request) {
	addr := r.URL.Host

	dstConn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)

		return
	}

	dst := dstConn.(*net.TCPConn)

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)

		return
	}

	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)

		return
	}

	src := clientConn.(*net.TCPConn)

	src.Write([]byte("HTTP/1.0 200 Connection established\r\n\r\n")) //nolint:errcheck

	log.Printf("HTTP CONNECT: tunneling to %s", addr)

	defer dst.Close() //nolint:errcheck
	defer src.Close() //nolint:errcheck

	var eg errgroup.Group

	eg.Go(func() error { return transfer(dst, src, "src -> dst: "+addr) })
	eg.Go(func() error { return transfer(src, dst, "dst -> src: "+addr) })

	if err = eg.Wait(); err != nil {
		log.Printf("HTTP CONNECT: tunneling to %s: failed %v", addr, err)
	}
}

func transfer(destination *net.TCPConn, source *net.TCPConn, label string) error {
	defer destination.CloseWrite() //nolint:errcheck
	defer source.CloseRead()       //nolint:errcheck

	n, err := io.Copy(destination, source)
	if err != nil {
		return fmt.Errorf("transfer failed %s (%d bytes copied): %w", label, n, err)
	}

	return nil
}

func handleHTTP(w http.ResponseWriter, req *http.Request) {
	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)

		return
	}

	defer resp.Body.Close() //nolint:errcheck

	copyHeaders(w.Header(), resp.Header)

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body) //nolint:errcheck
}

func copyHeaders(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func loggingMiddleware(h http.Handler) http.Handler {
	logFn := func(rw http.ResponseWriter, r *http.Request) {
		h.ServeHTTP(rw, r) // serve the original request

		log.Printf("%s %s", r.Method, r.RequestURI)
	}

	return http.HandlerFunc(logFn)
}

func runHTTPProxy(ctx context.Context) error {
	srv := &http.Server{
		Addr: net.JoinHostPort("", strconv.Itoa(airgappedFlags.proxyPort)),
		Handler: loggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodConnect {
				handleTunneling(w, r)
			} else {
				handleHTTP(w, r)
			}
		})),
		// Disable HTTP/2.
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler)),
	}

	log.Printf("starting HTTP proxy on %s", srv.Addr)

	go srv.ListenAndServe() //nolint:errcheck

	<-ctx.Done()

	return srv.Close()
}

func init() {
	airgappedCmd.Flags().IPVar(&airgappedFlags.advertisedAddress, "advertised-address", net.IPv4(10, 5, 0, 2), "The address to advertise to the cluster.")
	airgappedCmd.Flags().IntVar(&airgappedFlags.httpsPort, "https-port", 8001, "The HTTPS server port.")
	airgappedCmd.Flags().IntVar(&airgappedFlags.proxyPort, "proxy-port", 8002, "The HTTP proxy port.")

	Cmd.AddCommand(airgappedCmd)
}
