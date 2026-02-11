// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package global provides global flags for talosctl.
package global

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/siderolabs/crypto/x509"
	"google.golang.org/grpc"

	"github.com/siderolabs/talos/pkg/cli"
	"github.com/siderolabs/talos/pkg/machinery/client"
	clientconfig "github.com/siderolabs/talos/pkg/machinery/client/config"
)

// Args is a context for the Talos command line client.
type Args struct {
	Talosconfig     string
	CmdContext      string
	Cluster         string
	Nodes           []string
	Endpoints       []string
	SideroV1KeysDir string
	SkipVerify      bool
}

// NodeList returns the list of nodes to run the command against.
func (c *Args) NodeList() []string {
	return c.Nodes
}

// WithClientNoNodes wraps common code to initialize Talos client and provide cancellable context.
//
// WithClientNoNodes doesn't set any node information on the request context.
func (c *Args) WithClientNoNodes(action func(context.Context, *client.Client) error, dialOptions ...grpc.DialOption) error {
	// If SkipVerify is set, use WithClientSkipVerify instead
	if c.SkipVerify {
		return c.WithClientSkipVerify(action)
	}

	return cli.WithContext(
		context.Background(), func(ctx context.Context) error {
			cfg, err := clientconfig.Open(c.Talosconfig)
			if err != nil {
				return fmt.Errorf("failed to open config file %q: %w", c.Talosconfig, err)
			}

			opts := []client.OptionFunc{
				client.WithConfig(cfg),
				client.WithDefaultGRPCDialOptions(),
				client.WithGRPCDialOptions(dialOptions...),
				client.WithSideroV1KeysDir(clientconfig.CustomSideroV1KeysDirPath(c.SideroV1KeysDir)),
			}

			if c.CmdContext != "" {
				opts = append(opts, client.WithContextName(c.CmdContext))
			}

			if len(c.Endpoints) > 0 {
				// override endpoints from command-line flags
				opts = append(opts, client.WithEndpoints(c.Endpoints...))
			}

			if c.Cluster != "" {
				opts = append(opts, client.WithCluster(c.Cluster))
			}

			c, err := client.New(ctx, opts...)
			if err != nil {
				return fmt.Errorf("error constructing client: %w", err)
			}
			//nolint:errcheck
			defer c.Close()

			return action(ctx, c)
		},
	)
}

// ErrConfigContext is returned when config context cannot be resolved.
var ErrConfigContext = errors.New("failed to resolve config context")

func (c *Args) getNodes(cli *client.Client) ([]string, error) {
	if len(c.Nodes) < 1 {
		configContext := cli.GetConfigContext()
		if configContext == nil {
			return nil, ErrConfigContext
		}

		c.Nodes = configContext.Nodes
	}

	if len(c.Nodes) < 1 {
		return nil, errors.New("nodes are not set for the command: please use `--nodes` flag or configuration file to set the nodes to run the command against")
	}

	return c.Nodes, nil
}

// WithClient builds upon WithClientNoNodes to provide set of nodes on request context based on config & flags.
func (c *Args) WithClient(action func(context.Context, *client.Client) error, dialOptions ...grpc.DialOption) error {
	return c.WithClientNoNodes(
		func(ctx context.Context, cli *client.Client) error {
			nodes, err := c.getNodes(cli)
			if err != nil {
				return err
			}

			ctx = client.WithNodes(ctx, nodes...)

			return action(ctx, cli)
		},
		dialOptions...,
	)
}

// WithClientAndNodes builds upon WithClientNoNodes to provide a list of nodes to the function.
func (c *Args) WithClientAndNodes(action func(context.Context, *client.Client, []string) error, dialOptions ...grpc.DialOption) error {
	return c.WithClientNoNodes(
		func(ctx context.Context, cli *client.Client) error {
			nodes, err := c.getNodes(cli)
			if err != nil {
				return err
			}

			return action(ctx, cli, nodes)
		},
		dialOptions...,
	)
}

// WithClientMaintenance wraps common code to initialize Talos client in maintenance (insecure mode).
func (c *Args) WithClientMaintenance(enforceFingerprints []string, action func(context.Context, *client.Client) error) error {
	return cli.WithContext(
		context.Background(), func(ctx context.Context) error {
			tlsConfig := &tls.Config{
				InsecureSkipVerify: true,
			}

			if len(enforceFingerprints) > 0 {
				fingerprints := make([]x509.Fingerprint, len(enforceFingerprints))

				for i, stringFingerprint := range enforceFingerprints {
					var err error

					fingerprints[i], err = x509.ParseFingerprint(stringFingerprint)
					if err != nil {
						return fmt.Errorf("error parsing certificate fingerprint %q: %v", stringFingerprint, err)
					}
				}

				tlsConfig.VerifyConnection = x509.MatchSPKIFingerprints(fingerprints...)
			}

			c, err := client.New(ctx, client.WithTLSConfig(tlsConfig), client.WithEndpoints(c.Nodes...))
			if err != nil {
				return err
			}

			//nolint:errcheck
			defer c.Close()

			return action(ctx, c)
		},
	)
}

// WithClientSkipVerify wraps common code to initialize Talos client with TLS verification disabled
// but with client certificate authentication preserved.
// This is useful when connecting to nodes via IP addresses not listed in the server certificate's SANs.
func (c *Args) WithClientSkipVerify(action func(context.Context, *client.Client) error) error {
	return cli.WithContext(
		context.Background(), func(ctx context.Context) error {
			cfg, err := clientconfig.Open(c.Talosconfig)
			if err != nil {
				return fmt.Errorf("failed to open config file %q: %w", c.Talosconfig, err)
			}

			// Get context name - use override if specified, otherwise use default
			contextName := c.CmdContext
			if contextName == "" {
				contextName = cfg.Context
			}

			configContext, ok := cfg.Contexts[contextName]
			if !ok {
				return fmt.Errorf("context %q not found in config", contextName)
			}

			// Build TLS config with InsecureSkipVerify but preserve client certificate
			tlsConfig := &tls.Config{
				InsecureSkipVerify: true,
			}

			// Add client certificate if available
			if configContext.Crt != "" && configContext.Key != "" {
				crtBytes, err := base64.StdEncoding.DecodeString(configContext.Crt)
				if err != nil {
					return fmt.Errorf("error decoding certificate: %w", err)
				}

				keyBytes, err := base64.StdEncoding.DecodeString(configContext.Key)
				if err != nil {
					return fmt.Errorf("error decoding key: %w", err)
				}

				cert, err := tls.X509KeyPair(crtBytes, keyBytes)
				if err != nil {
					return fmt.Errorf("could not load client key pair: %w", err)
				}

				tlsConfig.Certificates = []tls.Certificate{cert}
			}

			opts := []client.OptionFunc{
				client.WithTLSConfig(tlsConfig),
				client.WithDefaultGRPCDialOptions(),
			}

			// Use endpoints from command-line flags or config
			if len(c.Endpoints) > 0 {
				opts = append(opts, client.WithEndpoints(c.Endpoints...))
			} else if len(configContext.Endpoints) > 0 {
				opts = append(opts, client.WithEndpoints(configContext.Endpoints...))
			}

			cli, err := client.New(ctx, opts...)
			if err != nil {
				return fmt.Errorf("error constructing client: %w", err)
			}
			//nolint:errcheck
			defer cli.Close()

			// Set nodes on context
			if len(c.Nodes) > 0 {
				ctx = client.WithNodes(ctx, c.Nodes...)
			} else if len(configContext.Nodes) > 0 {
				ctx = client.WithNodes(ctx, configContext.Nodes...)
			}

			return action(ctx, cli)
		},
	)
}
