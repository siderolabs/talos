// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package global provides global flags for talosctl.
package global

import (
	"context"
	"crypto/tls"
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
}

// NodeList returns the list of nodes to run the command against.
func (c *Args) NodeList() []string {
	return c.Nodes
}

// WithClientNoNodes wraps common code to initialize Talos client and provide cancellable context.
//
// WithClientNoNodes doesn't set any node information on the request context.
func (c *Args) WithClientNoNodes(action func(context.Context, *client.Client) error, dialOptions ...grpc.DialOption) error {
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

// WithClient builds upon WithClientNoNodes to provide set of nodes on request context based on config & flags.
func (c *Args) WithClient(action func(context.Context, *client.Client) error, dialOptions ...grpc.DialOption) error {
	return c.WithClientNoNodes(
		func(ctx context.Context, cli *client.Client) error {
			if len(c.Nodes) < 1 {
				configContext := cli.GetConfigContext()
				if configContext == nil {
					return ErrConfigContext
				}

				c.Nodes = configContext.Nodes
			}

			if len(c.Nodes) < 1 {
				return errors.New("nodes are not set for the command: please use `--nodes` flag or configuration file to set the nodes to run the command against")
			}

			ctx = client.WithNodes(ctx, c.Nodes...)

			return action(ctx, cli)
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
