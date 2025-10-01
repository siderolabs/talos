// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package debug

import (
	"context"
	"crypto/tls"
	stdx509 "crypto/x509"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/siderolabs/crypto/x509"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/siderolabs/talos/pkg/cli"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/security"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
)

//go:embed httproot/*
var httpFs embed.FS

var airgappedFlags struct {
	advertisedAddress       net.IP
	proxyPort               int
	httpsProxyPort          int
	httpsPort               int
	useSecureProxy          bool
	injectHTTPProxy         bool
	httpsReverseProxyPort   int
	httpsReverseProxyTarget string
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
				caPEM, certPEM, keyPEM, err := generateSelfSignedCert()
				if err != nil {
					return nil
				}

				if err = generateConfigPatch(caPEM); err != nil {
					return err
				}

				eg, ctx := errgroup.WithContext(ctx)

				eg.Go(func() error { return runHTTPServer(ctx, certPEM, keyPEM) })
				eg.Go(func() error { return runHTTPProxy(ctx) })
				eg.Go(func() error { return runHTTPSProxy(ctx, certPEM, keyPEM) })
				eg.Go(func() error { return runHTTPSReverseProxy(ctx, certPEM, keyPEM) })

				return eg.Wait()
			},
		)
	},
}

func generateConfigPatch(caPEM []byte) error {
	patch1 := &v1alpha1.Config{
		ClusterConfig: &v1alpha1.ClusterConfig{
			ExtraManifests: []string{
				fmt.Sprintf("https://%s/debug.yaml", net.JoinHostPort(airgappedFlags.advertisedAddress.String(), strconv.Itoa(airgappedFlags.httpsPort))),
			},
		},
	}

	if airgappedFlags.injectHTTPProxy {
		proxyURL := fmt.Sprintf("http://%s", net.JoinHostPort(airgappedFlags.advertisedAddress.String(), strconv.Itoa(airgappedFlags.proxyPort)))

		if airgappedFlags.useSecureProxy {
			proxyURL = fmt.Sprintf("https://%s", net.JoinHostPort(airgappedFlags.advertisedAddress.String(), strconv.Itoa(airgappedFlags.httpsProxyPort)))
		}

		patch1.MachineConfig = &v1alpha1.MachineConfig{
			MachineEnv: map[string]string{
				"http_proxy":  proxyURL,
				"https_proxy": proxyURL,
				"no_proxy":    fmt.Sprintf("%s/24", airgappedFlags.advertisedAddress.String()),
			},
		}
	}

	patch2 := security.NewTrustedRootsConfigV1Alpha1()
	patch2.MetaName = "air-gapped-ca"
	patch2.Certificates = string(caPEM)

	ctr, err := container.New(patch1, patch2)
	if err != nil {
		return err
	}

	patchBytes, err := ctr.EncodeBytes(encoder.WithComments(encoder.CommentsDisabled))
	if err != nil {
		return err
	}

	const patchFile = "air-gapped-patch.yaml"

	log.Printf("writing config patch to %s", patchFile)

	return os.WriteFile(patchFile, patchBytes, 0o644)
}

func generateSelfSignedCert() ([]byte, []byte, []byte, error) {
	ca, err := x509.NewSelfSignedCertificateAuthority(x509.ECDSA(true))
	if err != nil {
		return nil, nil, nil, err
	}

	serverIdentity, err := x509.NewKeyPair(ca,
		x509.Organization("test"),
		x509.CommonName("server"),
		x509.IPAddresses([]net.IP{airgappedFlags.advertisedAddress}),
		x509.ExtKeyUsage([]stdx509.ExtKeyUsage{stdx509.ExtKeyUsageServerAuth}),
	)
	if err != nil {
		return nil, nil, nil, err
	}

	return ca.CrtPEM, serverIdentity.CrtPEM, serverIdentity.KeyPEM, nil
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

	var src conn

	if src, ok = clientConn.(conn); !ok {
		if tlsConn, ok := clientConn.(*tls.Conn); ok {
			src = &tlsConnWrapper{
				Conn:           tlsConn,
				closeReadWrite: tlsConn.NetConn().(*net.TCPConn),
			}
		} else {
			log.Printf("HTTP CONNECT: tunneling to %s: failed: connection is not a net.Conn: %T", addr, clientConn)
			http.Error(w, "Connection is not a net.Conn", http.StatusInternalServerError)

			return
		}
	}

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

type conn interface {
	io.Reader
	io.Writer
	io.Closer
	CloseRead() error
	CloseWrite() error
}

type tlsConnWrapper struct {
	net.Conn

	closeReadWrite *net.TCPConn
}

func (t *tlsConnWrapper) CloseRead() error {
	return t.closeReadWrite.CloseRead()
}

func (t *tlsConnWrapper) CloseWrite() error {
	return t.closeReadWrite.CloseWrite()
}

func transfer(destination conn, source conn, label string) error {
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

func runHTTPSProxy(ctx context.Context, certPEM, keyPEM []byte) error {
	certificate, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return err
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{certificate},
	}

	srv := &http.Server{
		Addr: net.JoinHostPort("", strconv.Itoa(airgappedFlags.httpsProxyPort)),
		Handler: loggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodConnect {
				handleTunneling(ctx, w, r)
			} else {
				handleHTTP(w, r)
			}
		})),
		// Disable HTTP/2.
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler)),
		// Secure
		TLSConfig: tlsConfig,
	}

	log.Printf("starting HTTPS proxy on %s", srv.Addr)

	go srv.ListenAndServeTLS("", "") //nolint:errcheck

	<-ctx.Done()

	return srv.Close()
}

func runHTTPSReverseProxy(ctx context.Context, certPEM, keyPEM []byte) error {
	certificate, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return err
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{certificate},
	}

	target, err := url.Parse(airgappedFlags.httpsReverseProxyTarget)
	if err != nil {
		return fmt.Errorf("error parsing reverse proxy target %q: %w", airgappedFlags.httpsReverseProxyTarget, err)
	}

	reverseProxy := httputil.NewSingleHostReverseProxy(target)

	srv := &http.Server{
		Addr:    net.JoinHostPort("", strconv.Itoa(airgappedFlags.httpsReverseProxyPort)),
		Handler: loggingMiddleware(reverseProxy),
		// Secure
		TLSConfig: tlsConfig,
	}

	log.Printf("starting HTTPS reverse proxy on %s to %s", srv.Addr, target.String())

	go srv.ListenAndServeTLS("", "") //nolint:errcheck

	<-ctx.Done()

	return srv.Close()
}

func init() {
	airgappedCmd.Flags().IPVar(&airgappedFlags.advertisedAddress, "advertised-address", net.IPv4(10, 5, 0, 2), "The address to advertise to the cluster.")
	airgappedCmd.Flags().IntVar(&airgappedFlags.httpsPort, "https-port", 8001, "The HTTPS server port.")
	airgappedCmd.Flags().IntVar(&airgappedFlags.proxyPort, "proxy-port", 8002, "The HTTP proxy port.")
	airgappedCmd.Flags().IntVar(&airgappedFlags.httpsProxyPort, "https-proxy-port", 8003, "The HTTPS proxy port.")
	airgappedCmd.Flags().BoolVar(&airgappedFlags.useSecureProxy, "use-secure-proxy", false, "Whether to use HTTPS proxy.")
	airgappedCmd.Flags().BoolVar(&airgappedFlags.injectHTTPProxy, "inject-http-proxy", true, "Whether to inject HTTP proxy configuration.")
	airgappedCmd.Flags().IntVar(&airgappedFlags.httpsReverseProxyPort, "https-reverse-proxy-port", 8004, "The HTTPS reverse proxy port.")
	airgappedCmd.Flags().StringVar(&airgappedFlags.httpsReverseProxyTarget, "https-reverse-proxy-target", "http://localhost/", "The HTTPS reverse proxy target (URL).")

	Cmd.AddCommand(airgappedCmd)
}
