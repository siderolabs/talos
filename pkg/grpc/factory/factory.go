// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package factory

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	grpclog "github.com/talos-systems/talos/pkg/grpc/middleware/log"
)

// Registrator describes the set of methods required in order for a concrete
// type to be used with the Listen function.
type Registrator interface {
	Register(*grpc.Server)
}

// Options is the functional options struct.
type Options struct {
	Port               int
	SocketPath         string
	Network            string
	Config             *tls.Config
	LogPrefix          string
	LogDestination     io.Writer
	ServerOptions      []grpc.ServerOption
	StreamInterceptors []grpc.StreamServerInterceptor
	UnaryInterceptors  []grpc.UnaryServerInterceptor
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

// ServerOptions appends to the gRPC server options of the server.
func ServerOptions(o ...grpc.ServerOption) Option {
	return func(args *Options) {
		args.ServerOptions = append(args.ServerOptions, o...)
	}
}

// WithStreamInterceptor appends to the list of gRPC server stream interceptors.
func WithStreamInterceptor(i grpc.StreamServerInterceptor) Option {
	return func(args *Options) {
		args.StreamInterceptors = append(args.StreamInterceptors, i)
	}
}

// WithUnaryInterceptor appends to the list of gRPC server unary interceptors.
func WithUnaryInterceptor(i grpc.UnaryServerInterceptor) Option {
	return func(args *Options) {
		args.UnaryInterceptors = append(args.UnaryInterceptors, i)
	}
}

// WithLog sets up request logging to specified destination.
func WithLog(prefix string, w io.Writer) Option {
	return func(args *Options) {
		args.LogPrefix = prefix
		args.LogDestination = w
	}
}

// WithDefaultLog sets up request logging to default destination.
func WithDefaultLog() Option {
	return func(args *Options) {
		args.LogDestination = log.Writer()
	}
}

func recoveryHandler(logger *log.Logger) grpc_recovery.RecoveryHandlerFunc {
	return func(p interface{}) error {
		if logger != nil {
			logger.Printf("panic:\n%s", string(debug.Stack()))
		}

		return status.Errorf(codes.Internal, "%v", p)
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

	var logger *log.Logger

	if opts.LogDestination != nil {
		logger = log.New(opts.LogDestination, opts.LogPrefix, log.Flags())

		logMiddleware := grpclog.NewMiddleware(logger)

		// Logging is installed as the first middleware so that request in the form it was received,
		// and status sent on the wire is logged (error/success). It also tracks whole duration of the
		// request, including other middleware overhead.
		opts.UnaryInterceptors = append([]grpc.UnaryServerInterceptor{logMiddleware.UnaryInterceptor()}, opts.UnaryInterceptors...)
		opts.StreamInterceptors = append([]grpc.StreamServerInterceptor{logMiddleware.StreamInterceptor()}, opts.StreamInterceptors...)
	}

	// Install default recovery interceptors.
	// Recovery is installed as the last middleware in the chain so that earlier middlewares in the chain
	// have a chance to process the error (e.g. logging middleware).
	opts.StreamInterceptors = append(opts.StreamInterceptors, grpc_recovery.StreamServerInterceptor(grpc_recovery.WithRecoveryHandler(recoveryHandler(logger))))
	opts.UnaryInterceptors = append(opts.UnaryInterceptors, grpc_recovery.UnaryServerInterceptor(grpc_recovery.WithRecoveryHandler(recoveryHandler(logger))))

	opts.ServerOptions = append(opts.ServerOptions,
		grpc.StreamInterceptor(grpc_middleware.ChainStreamServer(opts.StreamInterceptors...)),
		grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(opts.UnaryInterceptors...)),
	)

	return opts
}

// NewServer builds grpc server and binds it to the Registrator.
func NewServer(r Registrator, setters ...Option) *grpc.Server {
	opts := NewDefaultOptions(setters...)

	server := grpc.NewServer(opts.ServerOptions...)
	r.Register(server)

	return server
}

// NewListener builds listener for grpc server.
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
		if err := os.MkdirAll(filepath.Dir(address), 0o700); err != nil {
			return nil, fmt.Errorf("error creating containing directory for the file socket; %w", err)
		}
	case "tcp":
		address = ":" + strconv.Itoa(opts.Port)
	default:
		return nil, fmt.Errorf("unknown network: %s", opts.Network)
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
