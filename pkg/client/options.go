// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package client

import (
	"crypto/tls"
	"fmt"

	"google.golang.org/grpc"

	"github.com/talos-systems/talos/pkg/client/config"
)

// Options contains the set of client configuration options
type Options struct {
	endpointsOverride []string
	config            *config.Context
	tlsConfig         *tls.Config
	grpcDialOptions   []grpc.DialOption
}

// OptionFunc sets an option for the creation of the Client
type OptionFunc func(*Options) error

// WithConfigContext configures the Client with the configuration context provided.
func WithConfigContext(cfg *config.Context) OptionFunc {
	return func(o *Options) error {
		o.config = cfg
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

func withDefaultConfig() OptionFunc {
	return func(o *Options) (err error) {
		defaultConfigPath, err := config.GetDefaultPath()
		if err != nil {
			return fmt.Errorf("no client configuration provided and no default path found: %w", err)
		}

		return WithConfigContextFromFile(defaultConfigPath, "")(o)
	}
}

// WithConfigContextFromFile creates a Client with its configuration extracted from the given file in the given context.
// Supplying an empty context will return the default context contained within the configuration file.
func WithConfigContextFromFile(fn, context string) OptionFunc {
	return func(o *Options) (err error) {
		fullConfig, err := config.Open(fn)
		if err != nil {
			return fmt.Errorf("failed to read config from %q: %w", fn, err)
		}

		cfgContext := fullConfig.GetContext(context)
		if cfgContext == nil {
			return fmt.Errorf("context %q not found in config file %q", context, fn)
		}

		return WithConfigContext(cfgContext)(o)
	}
}
