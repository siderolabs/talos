// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cmd

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"
	"google.golang.org/grpc/peer"

	"github.com/talos-systems/talos/cmd/osctl/pkg/client"
	"github.com/talos-systems/talos/cmd/osctl/pkg/helpers"
	"github.com/talos-systems/talos/pkg/constants"
)

var (
	ca             string
	crt            string
	additionalSANs []string
	csr            string
	hours          int
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
	Use:           "osctl",
	Short:         "A CLI for out-of-band management of Kubernetes nodes created by Talos",
	Long:          ``,
	SilenceErrors: true,
	SilenceUsage:  true,
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
	rootCmd.PersistentFlags().StringVar(&cmdcontext, "context", "", "Context to be used in command")
	rootCmd.PersistentFlags().StringSliceVarP(&nodes, "nodes", "", []string{}, "target the specified nodes")
	rootCmd.PersistentFlags().StringSliceVarP(&endpoints, "endpoints", "e", []string{}, "override default endpoints in Talos configuration")

	if err := rootCmd.Execute(); err != nil {
		helpers.Fatalf("%s", err)
	}
}

// setupClientE wraps common code to initialize osd client
func setupClientE(action func(*client.Client) error) error {
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
	globalCtx = client.WithNodes(globalCtx, targetNodes...)

	c, err := client.NewClient(creds, configEndpoints, constants.ApidPort)
	if err != nil {
		return fmt.Errorf("error constructing client: %w", err)
	}
	// nolint: errcheck
	defer c.Close()

	return action(c)
}

// setupClient is like setupClient, but without an error
func setupClient(action func(*client.Client)) {
	err := setupClientE(func(c *client.Client) error {
		action(c)

		return nil
	})
	if err != nil {
		helpers.Fatalf("%s", err)
	}
}

// nolint: gocyclo
func extractTarGz(localPath string, r io.ReadCloser) error {
	defer r.Close() //nolint: errcheck

	zr, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("error initializing gzip: %w", err)
	}

	tr := tar.NewReader(zr)

	for {
		hdr, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				break
			}

			return fmt.Errorf("error reading tar header: %s", err)
		}

		path := filepath.Clean(filepath.Join(localPath, hdr.Name))
		// TODO: do we need to clean up any '..' references?

		switch hdr.Typeflag {
		case tar.TypeDir:
			mode := hdr.FileInfo().Mode()
			mode |= 0700 // make rwx for the owner

			if err = os.Mkdir(path, mode); err != nil {
				return fmt.Errorf("error creating directory %q mode %s: %w", path, mode, err)
			}

			if err = os.Chmod(path, mode); err != nil {
				return fmt.Errorf("error updating mode %s for %q: %w", mode, path, err)
			}

		case tar.TypeSymlink:
			if err = os.Symlink(hdr.Linkname, path); err != nil {
				return fmt.Errorf("error creating symlink %q -> %q: %w", path, hdr.Linkname, err)
			}

		default:
			mode := hdr.FileInfo().Mode()

			fp, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_EXCL, mode)
			if err != nil {
				return fmt.Errorf("error creating file %q mode %s: %w", path, mode, err)
			}

			_, err = io.Copy(fp, tr)
			if err != nil {
				return fmt.Errorf("error copying data to %q: %w", path, err)
			}

			if err = fp.Close(); err != nil {
				return fmt.Errorf("error closing %q: %w", path, err)
			}

			if err = os.Chmod(path, mode); err != nil {
				return fmt.Errorf("error updating mode %s for %q: %w", mode, path, err)
			}
		}
	}

	return nil
}

func remotePeer(ctx context.Context) (peerHost string) {
	peerHost = "unknown"

	remote, ok := peer.FromContext(ctx)
	if ok {
		peerHost = addrFromPeer(remote)
	}

	return
}

func addrFromPeer(remote *peer.Peer) (peerHost string) {
	peerHost = remote.Addr.String()
	peerHost, _, _ = net.SplitHostPort(peerHost) //nolint: errcheck

	return peerHost
}
