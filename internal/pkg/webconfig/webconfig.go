// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package webconfig

import (
	"context"
	"crypto/tls"
	stdx509 "crypto/x509"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/talos-systems/crypto/x509"
	tnet "github.com/talos-systems/net"

	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/config/configloader"
)

const indexPage = `
<html>
	<head>
		<title>Talos Interactive Configurator</title>
	</head>
	<body>
	  <h3>Talos Interactive Configuration</h3>
	  <form action="/" method="POST" enctype="multipart/form-data">
	  <p>Upload YAML configuration: <input type="file" name="config.yaml" /></p>
	  <p><input type=submit value="Upload"></button></p>
	  </form>
	</body>
</html>
`

const configurationRejectedPage = `
<html>
  <head>
    <title>Talos Interactive Configurtor: configuration rejected</title>
  </head>
  <body>
    <p>The configuration was rejected:</p>
	 <pre>
	 {{ html . }}
	 </pre>
	 <p>Please try again from the <a href="/">main page</a>.</p>
  </body>
</html>
`

const configurationAcceptedPage = `
<html>
	<head>
		<title>Talos Interactive Configurator: configuration accepted</title>
	</head>
	<body>
		<p>Configuration accepted.  Continuing with new configuration...</p>
	</body>
</html>
`

// Server defines an http-based configuration receiver service.
type Server struct {
	logger *log.Logger

	configBytes []byte

	mode config.RuntimeMode

	returnSignal chan struct{}

	serverError error

	svr *http.Server

	mu sync.Mutex
}

// Stop tells the server to shut down.
func (s *Server) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	s.mu.Lock()
	if s.returnSignal != nil {
		close(s.returnSignal)

		s.returnSignal = nil
	}

	if s.svr != nil {
		s.svr.Shutdown(ctx) // nolint: errcheck
	}
	s.mu.Unlock()
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		s.handleGet(w, r)
	case "POST":
		s.handlePost(w, r)
	default:
		w.WriteHeader(http.StatusBadRequest)
	}
}

// nolint: unparam
func (s *Server) handleGet(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)

	_, err := w.Write([]byte(indexPage))
	if err != nil {
		s.logger.Printf("webconfig: failed to write index page: %s", err.Error())
	}
}

func (s *Server) handlePost(w http.ResponseWriter, r *http.Request) {
	writeFailure := func(status int, err error) {
		s.logger.Printf("webconfig: failed to handle POST request: %s", err.Error())

		tmpl, err := template.New("postErr").Parse(configurationRejectedPage)
		if err != nil {
			s.logger.Println("failed to parse configuration rejection template:", err)
			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		w.WriteHeader(status)

		if err = tmpl.Execute(w, err.Error()); err != nil {
			s.logger.Println("failed to write response to client:", err)
		}
	}

	f, header, err := r.FormFile("config.yaml")
	if err != nil {
		writeFailure(http.StatusBadRequest, fmt.Errorf("failed to read POST data: %w", err))
		return
	}

	if header.Size < 1 {
		writeFailure(http.StatusBadRequest, fmt.Errorf("invalid header size"))

		return
	}

	s.configBytes, err = ioutil.ReadAll(f)
	if err != nil {
		writeFailure(http.StatusInternalServerError, fmt.Errorf("failed to read file from config POST: %w", err))

		return
	}

	cfgProvider, err := configloader.NewFromBytes(s.configBytes)
	if err != nil {
		writeFailure(http.StatusBadRequest, fmt.Errorf("failed to parse file from POST as a config: %w", err))

		return
	}

	if err = cfgProvider.Validate(s.mode); err != nil {
		writeFailure(http.StatusBadRequest, fmt.Errorf("configuration validation failed: %w", err))

		return
	}

	if _, err = w.Write([]byte(configurationAcceptedPage)); err != nil {
		writeFailure(http.StatusGone, fmt.Errorf("failed to write response to client: %w", err))

		return
	}

	// Got a config
	s.Stop()
}

func redirect(w http.ResponseWriter, req *http.Request) {
	url := "https://" + req.Host + req.URL.Path

	if len(req.URL.RawQuery) > 0 {
		url += "?" + req.URL.RawQuery
	}

	http.Redirect(w, req, url, http.StatusPermanentRedirect)
}

// Run executes the configuration receiver, returning any configuration it receives.
func (s *Server) Run(ctx context.Context) ([]byte, error) {
	if s.returnSignal == nil {
		s.returnSignal = make(chan struct{})
	}

	ips, err := tnet.IPAddrs()
	if err != nil {
		return nil, fmt.Errorf("failed to get list of IPs: %w", err)
	}

	tlsConfig, err := genKeypair(ips)
	if err != nil {
		s.logger.Println("failed to generate keypair:", err)
		return nil, fmt.Errorf("failed to generate self-signed keypair: %w", err)
	}

	s.svr = &http.Server{
		Handler:   s,
		ErrorLog:  s.logger,
		TLSConfig: tlsConfig,
	}

	go func() {
		if err = http.ListenAndServe(":80", http.HandlerFunc(redirect)); err != nil {
			s.logger.Println("webconfig: HTTP service stopped:", err)
		}

		s.Stop()
	}()

	go func() {
		if err = s.svr.ListenAndServeTLS("", ""); err != nil {
			s.logger.Println("webconfig: HTTPS service stopped:", err)
		}

		s.Stop()
	}()

	s.logger.Println("Webconfig started. You may POST a configuration to:")

	for _, ip := range ips {
		s.logger.Printf("\thttps://%s\n", ip.String())
	}

	s.logger.Println("You may also visit any of the above in your browser.")

	<-s.returnSignal

	return s.configBytes, s.serverError
}

// Run executes the configuration receiver on the supplied address,. returning any configuration it receives.
func Run(ctx context.Context, logger *log.Logger, mode config.RuntimeMode) ([]byte, error) {
	s := new(Server)
	s.mode = mode
	s.logger = logger

	s.logger.Println("starting webconfig")

	return s.Run(ctx)
}

func genKeypair(ips []net.IP) (*tls.Config, error) {
	ca, err := x509.NewSelfSignedCertificateAuthority(x509.RSA(true))
	if err != nil {
		return nil, fmt.Errorf("failed to generate self-signed CA: %w", err)
	}

	ips = append(ips, net.ParseIP("127.0.0.1"), net.ParseIP("::1"))

	keypair, err := x509.NewKeyPair(ca, x509.RSA(true), x509.IPAddresses(ips))
	if err != nil {
		return nil, err
	}

	pool := stdx509.NewCertPool()
	pool.AppendCertsFromPEM(ca.CrtPEM)

	tlsConfig := &tls.Config{
		RootCAs:      pool,
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{*keypair.Certificate},
	}

	return tlsConfig, nil
}
