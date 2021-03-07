// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package client

import (
	"crypto/tls"
	"fmt"

	"google.golang.org/grpc"

	"github.com/talos-systems/talos/pkg/machinery/client/config"
)

// Options contains the set of client configuration options.
type Options struct {
	endpointsOverride []string
	config            *config.Config
	configContext     *config.Context
	tlsConfig         *tls.Config
	grpcDialOptions   []grpc.DialOption

	contextOverride    string
	contextOverrideSet bool

	unixSocketPath string
}

// OptionFunc sets an option for the creation of the Client.
type OptionFunc func(*Options) error

// WithConfig configures the Client with the configuration provided.
// Additionally use WithContextName to override the default context in the Config.
func WithConfig(cfg *config.Config) OptionFunc {
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
func WithConfigContext(cfg *config.Context) OptionFunc {
	return func(o *Options) error {
		o.configContext = cfg

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
		defaultConfigPath, err := config.GetDefaultPath()
		if err != nil {
			return fmt.Errorf("no client configuration provided and no default path found: %w", err)
		}

		return WithConfigFromFile(defaultConfigPath)(o)
	}
}

// WithConfigFromFile creates a Client with its configuration extracted from the given file.
// Additionally use WithContextName to select a context other than the default.
func WithConfigFromFile(fn string) OptionFunc {
	return func(o *Options) (err error) {
		cfg, err := config.Open(fn)
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
