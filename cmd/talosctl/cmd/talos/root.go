// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"github.com/talos-systems/crypto/x509"
	"google.golang.org/grpc/codes"

	"github.com/talos-systems/talos/pkg/cli"
	machineapi "github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/client"
	clientconfig "github.com/talos-systems/talos/pkg/machinery/client/config"
)

var kubernetes bool

// Common options set on root command.
var (
	Talosconfig string
	Endpoints   []string
	Nodes       []string
	Cmdcontext  string
)

const pathAutoCompleteLimit = 500

// WithClientNoNodes wraps common code to initialize Talos client and provide cancellable context.
//
// WithClientNoNodes doesn't set any node information on request context.
func WithClientNoNodes(action func(context.Context, *client.Client) error) error {
	return cli.WithContext(context.Background(), func(ctx context.Context) error {
		cfg, err := clientconfig.Open(Talosconfig)
		if err != nil {
			return fmt.Errorf("failed to open config file %q: %w", Talosconfig, err)
		}

		opts := []client.OptionFunc{
			client.WithConfig(cfg),
		}

		if Cmdcontext != "" {
			opts = append(opts, client.WithContextName(Cmdcontext))
		}

		if len(Endpoints) > 0 {
			// override endpoints from command-line flags
			opts = append(opts, client.WithEndpoints(Endpoints...))
		}

		c, err := client.New(ctx, opts...)
		if err != nil {
			return fmt.Errorf("error constructing client: %w", err)
		}
		//nolint:errcheck
		defer c.Close()

		return action(ctx, c)
	})
}

// WithClient builds upon WithClientNoNodes to provide set of nodes on request context based on config & flags.
func WithClient(action func(context.Context, *client.Client) error) error {
	return WithClientNoNodes(func(ctx context.Context, c *client.Client) error {
		if len(Nodes) < 1 {
			configContext := c.GetConfigContext()
			if configContext == nil {
				return fmt.Errorf("failed to resolve config context")
			}

			Nodes = configContext.Nodes
		}

		if len(Nodes) < 1 {
			return fmt.Errorf("nodes are not set for the command: please use `--nodes` flag or configuration file to set the nodes to run the command against")
		}

		ctx = client.WithNodes(ctx, Nodes...)

		return action(ctx, c)
	})
}

// WithClientMaintenance wraps common code to initialize Talos client in maintenance (insecure mode).
func WithClientMaintenance(enforceFingerprints []string, action func(context.Context, *client.Client) error) error {
	return cli.WithContext(context.Background(), func(ctx context.Context) error {
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

		c, err := client.New(ctx, client.WithTLSConfig(tlsConfig), client.WithEndpoints(Nodes...))
		if err != nil {
			return err
		}

		//nolint:errcheck
		defer c.Close()

		return action(ctx, c)
	})
}

// Commands is a list of commands published by the package.
var Commands []*cobra.Command

func addCommand(cmd *cobra.Command) {
	Commands = append(Commands, cmd)
}

// completeResource represents tab complete options for `ls` and `ls *` commands.
func completePathFromNode(inputPath string) []string {
	pathToSearch := inputPath

	// If the pathToSearch is empty, use root '/'
	if pathToSearch == "" {
		pathToSearch = "/"
	}

	var paths map[string]struct{}

	// search up one level to find possible completions
	if pathToSearch != "/" && !strings.HasSuffix(pathToSearch, "/") {
		index := strings.LastIndex(pathToSearch, "/")
		// we need a trailing slash to search for items in a directory
		pathToSearch = pathToSearch[:index] + "/"
	}

	paths = getPathFromNode(pathToSearch, inputPath)

	result := make([]string, 0, len(paths))

	for k := range paths {
		result = append(result, k)
	}

	return result
}

//nolint:gocyclo
func getPathFromNode(path, filter string) map[string]struct{} {
	paths := make(map[string]struct{})

	if WithClient(func(ctx context.Context, c *client.Client) error {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		stream, err := c.LS(ctx, &machineapi.ListRequest{
			Root: path,
		})
		if err != nil {
			return err
		}

		for {
			resp, err := stream.Recv()
			if err != nil {
				if err == io.EOF || client.StatusCode(err) == codes.Canceled {
					return nil
				}

				return fmt.Errorf("error streaming results: %s", err)
			}

			if resp.Metadata != nil && resp.Metadata.Error != "" {
				continue
			}

			if resp.Error != "" {
				continue
			}

			// skip reference to the same directory
			if resp.RelativeName == "." {
				continue
			}

			// limit the results to a reasonable amount
			if len(paths) > pathAutoCompleteLimit {
				return nil
			}

			// directories have a trailing slash
			if resp.IsDir {
				fullPath := path + resp.RelativeName + "/"

				if relativeTo(fullPath, filter) {
					paths[fullPath] = struct{}{}
				}
			} else {
				fullPath := path + resp.RelativeName

				if relativeTo(fullPath, filter) {
					paths[fullPath] = struct{}{}
				}
			}
		}
	}) != nil {
		return paths
	}

	return paths
}

func relativeTo(fullPath string, filter string) bool {
	return strings.HasPrefix(fullPath, filter)
}
