// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package inmemhttp implements temporary HTTP server which is based off memory fs.
package inmemhttp

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"time"
)

// Server is an in-memory http web server.
type Server interface {
	AddFile(filename string, contents []byte) error
	AddHandler(pattern string, handler http.Handler)

	GetAddr() net.Addr
	Serve()
	Shutdown(ctx context.Context) error
}

type server struct {
	l    net.Listener
	addr net.Addr

	srv *http.Server
	mux *http.ServeMux
}

// NewServer creates in-mem HTTP server.
func NewServer(address string) (Server, error) {
	s := &server{
		mux: http.NewServeMux(),
	}

	var err error

	s.l, err = net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}

	s.addr = s.l.Addr()

	s.srv = &http.Server{
		Handler: s.mux,
	}

	return s, nil
}

func (s *server) AddFile(filename string, contents []byte) error {
	contentsCopy := append([]byte(nil), contents...)

	s.mux.Handle("/"+filename, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodHead:
			w.Header().Add("Content-Length", strconv.Itoa(len(contentsCopy)))
			w.WriteHeader(http.StatusOK)
		case http.MethodGet:
			w.Header().Add("Content-Length", strconv.Itoa(len(contentsCopy)))
			w.WriteHeader(http.StatusOK)

			w.Write(contentsCopy) //nolint:errcheck
		default:
			w.WriteHeader(http.StatusNotImplemented)
		}
	}))

	return nil
}

func (s *server) AddHandler(pattern string, handler http.Handler) {
	s.mux.Handle(pattern, handler)
}

func (s *server) GetAddr() net.Addr {
	return s.addr
}

func (s *server) Serve() {
	go s.srv.Serve(s.l) //nolint:errcheck
}

func (s *server) Shutdown(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	return s.srv.Shutdown(ctx)
}
