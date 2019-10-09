/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package factory

import (
	"crypto/tls"
	"net"
	"os"
	"path/filepath"
	"strconv"

	"github.com/pkg/errors"
	"google.golang.org/grpc"
)

// Registrator describes the set of methods required in order for a concrete
// type to be used with the Listen function.
type Registrator interface {
	Register(*grpc.Server)
}

// Options is the functional options struct.
type Options struct {
	Port          int
	SocketPath    string
	Network       string
	Config        *tls.Config
	ServerOptions []grpc.ServerOption
}

// Option is the functional option func.
type Option func(*Options)

// Port sets the listen port of the server.
func Port(o int) Option {
	return func(args *Options) {
		args.Port = o
	}
}

// SocketPath sets the listen unix file socket path of the server.
func SocketPath(o string) Option {
	return func(args *Options) {
		args.SocketPath = o
	}
}

// Network sets the network type of the listener.
func Network(o string) Option {
	return func(args *Options) {
		args.Network = o
	}
}

// Config sets the listen port of the server.
func Config(o *tls.Config) Option {
	return func(args *Options) {
		args.Config = o
	}
}

// ServerOptions sets the gRPC server options of the server.
func ServerOptions(o ...grpc.ServerOption) Option {
	return func(args *Options) {
		args.ServerOptions = o
	}
}

// NewDefaultOptions initializes the Options struct with default values.
func NewDefaultOptions(setters ...Option) *Options {
	opts := &Options{
		Network:    "tcp",
		SocketPath: "/run/factory/factory.sock",
	}

	for _, setter := range setters {
		setter(opts)
	}

	return opts
}

// NewServer builds grpc server and binds it to the Registrator
func NewServer(r Registrator, setters ...Option) *grpc.Server {
	opts := NewDefaultOptions(setters...)

	server := grpc.NewServer(opts.ServerOptions...)
	r.Register(server)

	return server
}

// NewListener builds listener for grpc server
func NewListener(setters ...Option) (net.Listener, error) {
	opts := NewDefaultOptions(setters...)

	if opts.Network == "tcp" && opts.Port == 0 {
		return nil, errors.New("a port is required for TCP listener")
	}

	var address string

	switch opts.Network {
	case "unix":
		address = opts.SocketPath

		// Unlink the address or we will get the error:
		// bind: address already in use.
		if _, err := os.Stat(address); err == nil {
			if err := os.Remove(address); err != nil {
				return nil, err
			}
		}

		// Make any dirs on the path to the listening socket.
		if err := os.MkdirAll(filepath.Dir(address), 0700); err != nil {
			return nil, errors.Wrap(err, "error creating containing directory for the file socket")
		}
	case "tcp":
		address = ":" + strconv.Itoa(opts.Port)
	default:
		return nil, errors.Errorf("unknown network: %s", opts.Network)
	}

	return net.Listen(opts.Network, address)
}

// ListenAndServe configures TLS for mutual authtentication by loading the CA into a
// CertPool and configuring the server's policy for TLS Client Authentication.
// Once TLS is configured, the gRPC options are built to make use of the TLS
// configuration and the receiver (Server) is registered to the gRPC server.
// Finally the gRPC server is started.
func ListenAndServe(r Registrator, setters ...Option) (err error) {
	server := NewServer(r, setters...)

	listener, err := NewListener(setters...)
	if err != nil {
		return err
	}

	return server.Serve(listener)
}
