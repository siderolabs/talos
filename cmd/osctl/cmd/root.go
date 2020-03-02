// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/talos-systems/talos/cmd/osctl/pkg/client"
	"github.com/talos-systems/talos/cmd/osctl/pkg/client/config"
	"github.com/talos-systems/talos/cmd/osctl/pkg/helpers"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/grpc/tls"
	"github.com/talos-systems/talos/pkg/version"
)

var (
	ca             string
	crt            string
	additionalSANs []string
	csr            string
	caHours        int
	crtHours       int
	ip             string
	key            string
	kubernetes     bool
	useCRI         bool
	name           string
	organization   string
	rsa            bool
	talosconfig    string
	endpoints      []string
	nodes          []string
	cmdcontext     string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:               "osctl",
	Short:             "A CLI for out-of-band management of Kubernetes nodes created by Talos",
	Long:              ``,
	SilenceErrors:     true,
	SilenceUsage:      true,
	DisableAutoGenTag: true,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() error {
	defaultTalosConfig, err := config.GetDefaultPath()
	if err != nil {
		return err
	}

	rootCmd.PersistentFlags().StringVar(&talosconfig, "talosconfig", defaultTalosConfig, "The path to the Talos configuration file")
	rootCmd.PersistentFlags().StringVar(&cmdcontext, "context", "", "Context to be used in command")
	rootCmd.PersistentFlags().StringSliceVarP(&nodes, "nodes", "n", []string{}, "target the specified nodes")
	rootCmd.PersistentFlags().StringSliceVarP(&endpoints, "endpoints", "e", []string{}, "override default endpoints in Talos configuration")

	cmd, err := rootCmd.ExecuteC()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())

		errorString := err.Error()
		// TODO: this is a nightmare, but arg-flag related validation returns simple `fmt.Errorf`, no way to distinguish
		//       these errors
		if strings.Contains(errorString, "arg(s)") || strings.Contains(errorString, "flag") || strings.Contains(errorString, "command") {
			fmt.Fprintln(os.Stderr)
			fmt.Fprintln(os.Stderr, cmd.UsageString())
		}
	}

	return err
}

// WithClient wraps common code to initialize Talos client and provide cancellable context.
func WithClient(action func(context.Context, *client.Client) error) error {
	return helpers.WithCLIContext(context.Background(), func(ctx context.Context) error {
		configContext, creds, err := client.NewClientContextAndCredentialsFromConfig(talosconfig, cmdcontext)
		if err != nil {
			return fmt.Errorf("error getting client credentials: %w", err)
		}

		configEndpoints := configContext.Endpoints

		if len(endpoints) > 0 {
			// override endpoints from command-line flags
			configEndpoints = endpoints
		}

		targetNodes := configContext.Nodes

		if len(nodes) > 0 {
			targetNodes = nodes
		}

		// Update context with grpc metadata for proxy/relay requests
		ctx = client.WithNodes(ctx, targetNodes...)

		tlsconfig, err := tls.New(
			tls.WithKeypair(creds.Crt),
			tls.WithClientAuthType(tls.Mutual),
			tls.WithCACertPEM(creds.CA),
		)
		if err != nil {
			return err
		}

		c, err := client.NewClient(tlsconfig, configEndpoints, constants.ApidPort)
		if err != nil {
			return fmt.Errorf("error constructing client: %w", err)
		}
		// nolint: errcheck
		defer c.Close()

		return action(ctx, c)
	})
}

func defaultImage(image string) string {
	return fmt.Sprintf("%s:%s", image, getEnv("TAG", version.Tag))
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}

	return fallback
}
