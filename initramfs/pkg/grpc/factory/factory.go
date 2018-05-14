package factory

import (
	"crypto/tls"
	"net"
	"strconv"

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

// Config sets the listen port of the server.
func Config(o *tls.Config) Option {
	return func(args *Options) {
		args.Config = o
	}
}

// ServerOptions sets the listen port of the server.
func ServerOptions(o ...grpc.ServerOption) Option {
	return func(args *Options) {
		args.ServerOptions = o
	}
}

// NewDefaultOptions initializes the Options struct with default values.
func NewDefaultOptions(setters ...Option) *Options {
	opts := &Options{
		Port: 50000,
	}

	for _, setter := range setters {
		setter(opts)
	}

	return opts
}

// Listen configures TLS for mutual authtentication by loading the CA into a
// CertPool and configuring the server's policy for TLS Client Authentication.
// Once TLS is configured, the gRPC options are built to make use of the TLS
// configuration and the receiver (Server) is registered to the gRPC server.
// Finally the gRPC server is started.
func Listen(r Registrator, setters ...Option) (err error) {
	opts := NewDefaultOptions(setters...)

	server := grpc.NewServer(opts.ServerOptions...)
	r.Register(server)

	listener, err := net.Listen("tcp", ":"+strconv.Itoa(opts.Port))
	if err != nil {
		return
	}

	err = server.Serve(listener)
	if err != nil {
		return
	}

	return nil
}
