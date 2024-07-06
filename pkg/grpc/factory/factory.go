// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package factory

import (
	"context"
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

	grpc_recovery "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	_ "github.com/siderolabs/talos/pkg/grpc/codec" // register codec
	grpclog "github.com/siderolabs/talos/pkg/grpc/middleware/log"
)

// Registrator describes the set of methods required in order for a concrete
// type to be used with the Listen function.
type Registrator interface {
	Register(*grpc.Server)
}

// Options is the functional options struct.
type Options struct {
	Address            string
	Port               int
	SocketPath         string
	Network            string
	Config             *tls.Config
	ServerOptions      []grpc.ServerOption
	UnaryInterceptors  []grpc.UnaryServerInterceptor
	StreamInterceptors []grpc.StreamServerInterceptor
	Reflection         bool
	logPrefix          string
	logDestination     io.Writer
}

// Option is the functional option func.
type Option func(*Options)

// Address sets the listen address of the server.
func Address(a string) Option {
	return func(args *Options) {
		args.Address = a
	}
}

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

// WithUnaryInterceptor appends to the list of gRPC server unary interceptors.
func WithUnaryInterceptor(i grpc.UnaryServerInterceptor) Option {
	return func(args *Options) {
		args.UnaryInterceptors = append(args.UnaryInterceptors, i)
	}
}

// WithStreamInterceptor appends to the list of gRPC server stream interceptors.
func WithStreamInterceptor(i grpc.StreamServerInterceptor) Option {
	return func(args *Options) {
		args.StreamInterceptors = append(args.StreamInterceptors, i)
	}
}

// WithLog sets up request logging to specified destination.
func WithLog(prefix string, w io.Writer) Option {
	return func(args *Options) {
		args.logPrefix = prefix
		args.logDestination = w
	}
}

// WithDefaultLog sets up request logging to default destination.
func WithDefaultLog() Option {
	return func(args *Options) {
		args.logDestination = log.Writer()
	}
}

// WithReflection enables gRPC reflection APIs: https://github.com/grpc/grpc/blob/master/doc/server-reflection.md
func WithReflection() Option {
	return func(args *Options) {
		args.Reflection = true
	}
}

func recoveryHandler(logger *log.Logger) grpc_recovery.RecoveryHandlerFunc {
	return func(p any) error {
		if logger != nil {
			logger.Printf("panic: %v\n%s", p, string(debug.Stack()))
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

	if opts.logDestination != nil {
		logger = log.New(opts.logDestination, opts.logPrefix, log.Flags())
	}

	// Recovery is installed as the first middleware in the chain to handle panics (via defer and recover()) in all subsequent middlewares.
	recoveryOpt := grpc_recovery.WithRecoveryHandler(recoveryHandler(logger))
	opts.UnaryInterceptors = append(
		[]grpc.UnaryServerInterceptor{grpc_recovery.UnaryServerInterceptor(recoveryOpt)},
		opts.UnaryInterceptors...,
	)
	opts.StreamInterceptors = append(
		[]grpc.StreamServerInterceptor{grpc_recovery.StreamServerInterceptor(recoveryOpt)},
		opts.StreamInterceptors...,
	)

	if logger != nil {
		// Logging is installed as the first middleware (even before recovery middleware) in the chain
		// so that request in the form it was received and status sent on the wire is logged (error/success).
		// It also tracks the whole duration of the request, including other middleware overhead.
		logMiddleware := grpclog.NewMiddleware(logger)
		opts.UnaryInterceptors = append(
			[]grpc.UnaryServerInterceptor{logMiddleware.UnaryInterceptor()},
			opts.UnaryInterceptors...,
		)
		opts.StreamInterceptors = append(
			[]grpc.StreamServerInterceptor{logMiddleware.StreamInterceptor()},
			opts.StreamInterceptors...,
		)
	}

	opts.ServerOptions = append(
		opts.ServerOptions,
		grpc.InitialWindowSize(65535*32),
		grpc.InitialConnWindowSize(65535*16),
		grpc.ChainUnaryInterceptor(opts.UnaryInterceptors...),
		grpc.ChainStreamInterceptor(opts.StreamInterceptors...),
		grpc.SharedWriteBuffer(true),
	)

	return opts
}

// NewServer builds grpc server and binds it to the Registrator.
func NewServer(r Registrator, setters ...Option) *grpc.Server {
	opts := NewDefaultOptions(setters...)

	server := grpc.NewServer(opts.ServerOptions...)
	r.Register(server)

	if opts.Reflection {
		reflection.Register(server)
	}

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
		address = net.JoinHostPort(opts.Address, strconv.Itoa(opts.Port))
	default:
		return nil, fmt.Errorf("unknown network: %s", opts.Network)
	}

	return net.Listen(opts.Network, address)
}

// ListenAndServe configures TLS for mutual authentication by loading the CA into a
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

// ServerGracefulStop the server with a timeout.
//
// Core gRPC doesn't support timeouts.
func ServerGracefulStop(server *grpc.Server, shutdownCtx context.Context) { //nolint:revive
	stopped := make(chan struct{})

	go func() {
		server.GracefulStop()
		close(stopped)
	}()

	select {
	case <-shutdownCtx.Done():
		server.Stop()
	case <-stopped:
		server.Stop()
	}
}
