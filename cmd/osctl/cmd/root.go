/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/talos-systems/talos/cmd/osctl/pkg/client"
	"github.com/talos-systems/talos/cmd/osctl/pkg/helpers"
	"github.com/talos-systems/talos/internal/pkg/constants"
)

var (
	ca           string
	crt          string
	csr          string
	hours        int
	ip           string
	key          string
	kubernetes   bool
	useCRI       bool
	name         string
	organization string
	rsa          bool
	talosconfig  string
	target       string
	userdataFile string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "osctl",
	Short: "A CLI for out-of-band management of Kubernetes nodes created by Talos",
	Long:  ``,
}

// Global context to be used in the commands.
//
// Cobra doesn't have a way to pass it around, so we have to use global variable.
// Context is initialized in Execute, and initial value is failsafe default.
var globalCtx = context.Background()

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	var globalCtxCancel context.CancelFunc
	globalCtx, globalCtxCancel = context.WithCancel(context.Background())
	defer globalCtxCancel()

	// listen for ^C and SIGTERM and abort context
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	exited := make(chan struct{})
	defer close(exited)

	go func() {
		select {
		case <-sigCh:
			globalCtxCancel()
		case <-globalCtx.Done():
			return
		}

		select {
		case <-sigCh:
			signal.Stop(sigCh)
			fmt.Fprintln(os.Stderr, "Signal received, aborting, press Ctrl+C once again to abort immediately...")
		case <-exited:
		}
	}()

	var (
		defaultTalosConfig string
		ok                 bool
	)
	if defaultTalosConfig, ok = os.LookupEnv(constants.TalosConfigEnvVar); !ok {
		home, err := os.UserHomeDir()
		if err != nil {
			return
		}
		defaultTalosConfig = path.Join(home, ".talos", "config")
	}
	rootCmd.PersistentFlags().StringVar(&talosconfig, "talosconfig", defaultTalosConfig, "The path to the Talos configuration file")
	if err := rootCmd.Execute(); err != nil {
		helpers.Fatalf("%s", err)
	}
}

// setupClient wraps common code to initialize osd client
func setupClient(action func(*client.Client)) {
	creds, err := client.NewDefaultClientCredentials(talosconfig)
	if err != nil {
		helpers.Fatalf("error getting client credentials: %s", err)
	}
	if target != "" {
		creds.Target = target
	}
	c, err := client.NewClient(constants.OsdPort, creds)
	if err != nil {
		helpers.Fatalf("error constructing client: %s", err)
	}
	// nolint: errcheck
	defer c.Close()

	action(c)
}
