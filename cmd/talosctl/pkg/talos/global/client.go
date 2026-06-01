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
func (args *Args) NodeList() []string {
	return args.Nodes
}

// WithClientNoNodes wraps common code to initialize Talos client and provide cancellable context.
//
// WithClientNoNodes doesn't set any node information on the request context.
func (args *Args) WithClientNoNodes(ctx context.Context, action func(context.Context, *client.Client) error, dialOptions ...grpc.DialOption) error {
	cfg, err := clientconfig.Open(args.Talosconfig)
	if err != nil {
		return fmt.Errorf("failed to open config file %q: %w", args.Talosconfig, err)
	}

	opts := []client.OptionFunc{
		client.WithConfig(cfg),
		client.WithDefaultGRPCDialOptions(),
		client.WithGRPCDialOptions(dialOptions...),
		client.WithSideroV1KeysDir(clientconfig.CustomSideroV1KeysDirPath(args.SideroV1KeysDir)),
	}

	if args.CmdContext != "" {
		opts = append(opts, client.WithContextName(args.CmdContext))
	}

	if len(args.Endpoints) > 0 {
		// override endpoints from command-line flags
		opts = append(opts, client.WithEndpoints(args.Endpoints...))
	}

	if args.Cluster != "" {
		opts = append(opts, client.WithCluster(args.Cluster))
	}

	c, err := client.New(ctx, opts...)
	if err != nil {
		return fmt.Errorf("error constructing client: %w", err)
	}
	//nolint:errcheck
	defer c.Close()

	return action(ctx, c)
}

// ErrConfigContext is returned when config context cannot be resolved.
var ErrConfigContext = errors.New("failed to resolve config context")

func (args *Args) getNodes(cli *client.Client) ([]string, error) {
	if len(args.Nodes) < 1 {
		configContext := cli.GetConfigContext()
		if configContext == nil {
			return nil, ErrConfigContext
		}

		args.Nodes = configContext.Nodes
	}

	if len(args.Nodes) < 1 {
		return nil, errors.New("nodes are not set for the command: please use `--nodes` flag or configuration file to set the nodes to run the command against")
	}

	return args.Nodes, nil
}

// WithClient builds upon WithClientNoNodes to provide set of nodes on request context based on config & flags.
func (args *Args) WithClient(ctx context.Context, action func(context.Context, *client.Client) error, dialOptions ...grpc.DialOption) error {
	return args.WithClientNoNodes(
		ctx,
		func(ctx context.Context, cli *client.Client) error {
			nodes, err := args.getNodes(cli)
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
func (args *Args) WithClientAndNodes(ctx context.Context, action func(context.Context, *client.Client, []string) error, dialOptions ...grpc.DialOption) error {
	return args.WithClientNoNodes(
		ctx,
		func(ctx context.Context, cli *client.Client) error {
			nodes, err := args.getNodes(cli)
			if err != nil {
				return err
			}

			return action(ctx, cli, nodes)
		},
		dialOptions...,
	)
}

// WithClientMaintenance wraps common code to initialize Talos client in maintenance (insecure mode).
func (args *Args) WithClientMaintenance(ctx context.Context, enforceFingerprints []string, action func(context.Context, *client.Client) error) error {
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

	cl, err := client.New(ctx, client.WithDefaultGRPCDialOptions(), client.WithTLSConfig(tlsConfig), client.WithEndpoints(args.Nodes...))
	if err != nil {
		return err
	}

	//nolint:errcheck
	defer cl.Close()

	return action(ctx, cl)
}
