// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package client

import (
	"crypto/tls"
	"fmt"

	"google.golang.org/grpc"

	clientconfig "github.com/siderolabs/talos/pkg/machinery/client/config"
	"github.com/siderolabs/talos/pkg/machinery/client/dialer"
)

var defaultDialOptions = []grpc.DialOption{
	grpc.WithContextDialer(dialer.DynamicProxyDialer),
}

// Options contains the set of client configuration options.
type Options struct {
	endpointsOverride []string
	config            *clientconfig.Config
	configContext     *clientconfig.Context
	tlsConfig         *tls.Config
	grpcDialOptions   []grpc.DialOption

	contextOverride    string
	contextOverrideSet bool

	unixSocketPath      string
	clusterNameOverride string
	sideroV1KeysDir     string
	skipVerify          bool
}

// OptionFunc sets an option for the creation of the Client.
type OptionFunc func(*Options) error

// WithConfig configures the Client with the configuration provided.
// Additionally use WithContextName to override the default context in the Config.
func WithConfig(cfg *clientconfig.Config) OptionFunc {
	return func(o *Options) error {
		o.config = cfg

		return nil
	}
}

// WithContextName overrides the default context inside a provided client Config.
func WithContextName(name string) OptionFunc {
	return func(o *Options) error {
		o.contextOverride = name

		o.contextOverrideSet = true

		return nil
	}
}

// WithConfigContext configures the Client with the configuration context provided.
func WithConfigContext(cfg *clientconfig.Context) OptionFunc {
	return func(o *Options) error {
		o.configContext = cfg

		return nil
	}
}

// WithDefaultGRPCDialOptions adds the default grpc.DialOptions to a Client.
func WithDefaultGRPCDialOptions() OptionFunc {
	return func(o *Options) error {
		o.grpcDialOptions = append(o.grpcDialOptions, defaultDialOptions...)

		return nil
	}
}

// WithGRPCDialOptions adds the given grpc.DialOptions to a Client.
func WithGRPCDialOptions(opts ...grpc.DialOption) OptionFunc {
	return func(o *Options) error {
		o.grpcDialOptions = append(o.grpcDialOptions, opts...)

		return nil
	}
}

// WithTLSConfig overrides the default TLS configuration with the one provided.
func WithTLSConfig(tlsConfig *tls.Config) OptionFunc {
	return func(o *Options) error {
		o.tlsConfig = tlsConfig

		return nil
	}
}

// WithEndpoints overrides the default endpoints with the provided list.
func WithEndpoints(endpoints ...string) OptionFunc {
	return func(o *Options) error {
		o.endpointsOverride = endpoints

		return nil
	}
}

// WithDefaultConfig creates a Client with its configuration sourced from the
// default config file location.
// Additionally use WithContextName to select a context other than the default.
func WithDefaultConfig() OptionFunc {
	return func(o *Options) (err error) {
		return WithConfigFromFile("")(o)
	}
}

// WithConfigFromFile creates a Client with its configuration extracted from the given file.
// Additionally use WithContextName to select a context other than the default.
func WithConfigFromFile(fn string) OptionFunc {
	return func(o *Options) (err error) {
		cfg, err := clientconfig.Open(fn)
		if err != nil {
			return fmt.Errorf("failed to read config from %q: %w", fn, err)
		}

		o.config = cfg

		return nil
	}
}

// WithUnixSocket creates a Client which connects to apid over local file socket.
//
// This option disables config parsing and TLS.
//
// Connection over unix socket is only used within the Talos node.
func WithUnixSocket(path string) OptionFunc {
	return func(o *Options) error {
		o.unixSocketPath = path

		return nil
	}
}

// WithCluster creates a Client which connects to the named cluster.
func WithCluster(cluster string) OptionFunc {
	return func(o *Options) error {
		o.clusterNameOverride = cluster

		return nil
	}
}

// WithSideroV1KeysDir overrides the default SideroV1KeysDir configuration with the one provided.
func WithSideroV1KeysDir(keysDir string) OptionFunc {
	return func(o *Options) error {
		o.sideroV1KeysDir = keysDir

		return nil
	}
}

// WithSkipVerify disables TLS certificate verification while preserving client authentication.
// This is useful when connecting to nodes via IP addresses not listed in the server certificate's SANs,
// or when the server certificate is signed by an unknown CA.
func WithSkipVerify() OptionFunc {
	return func(o *Options) error {
		o.skipVerify = true

		return nil
	}
}
