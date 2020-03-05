// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package firecracker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/firecracker-microvm/firecracker-go-sdk"

	"github.com/talos-systems/talos/internal/pkg/provision/providers/firecracker/inmemhttp"
)

// LaunchConfig is passed in to the Launch function over stdin.
type LaunchConfig struct {
	GatewayAddr         string
	Config              string
	BootloaderEmulation bool
	FirecrackerConfig   firecracker.Config
}

// Launch a control process around firecracker VM manager.
//
// This function is invoked from 'osctl firecracker-launch' hidden command
// and wraps starting, controlling and restarting 'firecracker' VM process.
//
// Launch restarts VM forever until control process is stopped itself with a signal.
//
// Process is expected to receive configuration on stdin. Current working directory
// should be cluster state directory, process output should be redirected to the
// logfile in state directory.
//
// When signals SIGINT, SIGTERM are received, control process stops firecracker and exits.
//
//nolint: gocyclo
func Launch() error {
	var config LaunchConfig

	d := json.NewDecoder(os.Stdin)

	if err := d.Decode(&config); err != nil {
		return fmt.Errorf("error decoding config from stdin: %w", err)
	}

	if d.More() {
		return fmt.Errorf("extra unexpected input on stdin")
	}

	if err := os.Stdin.Close(); err != nil {
		return err
	}

	signal.Ignore(syscall.SIGHUP)

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGTERM, syscall.SIGINT)

	ctx := context.Background()

	httpServer, err := inmemhttp.NewServer(fmt.Sprintf("%s:0", config.GatewayAddr))
	if err != nil {
		return fmt.Errorf("error launching in-memory HTTP server: %w", err)
	}

	if err = httpServer.AddFile("config.yaml", []byte(config.Config)); err != nil {
		return err
	}

	// patch kernel args
	config.FirecrackerConfig.KernelArgs = strings.ReplaceAll(config.FirecrackerConfig.KernelArgs, "{TALOS_CONFIG_URL}", fmt.Sprintf("http://%s/config.yaml", httpServer.GetAddr()))

	// save original kernel/initrd asset paths, so that we can re-use them if assets can't be extracted with NewBootLoader
	origKernelImagePath, origInitrdPath := config.FirecrackerConfig.KernelImagePath, config.FirecrackerConfig.InitrdPath

	httpServer.Serve()
	defer httpServer.Shutdown(ctx) //nolint: errcheck

	for {
		err := func() error {
			var (
				err        error
				bootLoader *BootLoader
			)

			// reset kernel/initrd assets to default values
			// bootloader (if enabled) might overwrite them with extracted assets
			config.FirecrackerConfig.KernelImagePath, config.FirecrackerConfig.InitrdPath = origKernelImagePath, origInitrdPath

			bootLoader, err = NewBootLoader(*config.FirecrackerConfig.Drives[0].PathOnHost)
			if err != nil {
				// print err but continue boot process
				fmt.Fprintf(os.Stderr, "error initializing bootloader: %s\n", err.Error())
			} else {
				defer bootLoader.Close() //nolint: errcheck

				var assets BootAssets

				assets, err = bootLoader.ExtractAssets()
				if err != nil {
					fmt.Fprintf(os.Stderr, "error extracting kernel assets: %s\n", err.Error())
				} else if config.BootloaderEmulation {
					// boot partition found, pass kernel & initrd from boot partition to Firecracker
					config.FirecrackerConfig.KernelImagePath = assets.KernelPath
					config.FirecrackerConfig.InitrdPath = assets.InitrdPath

					fmt.Fprintf(os.Stderr, "successfully extracted boot assets from the disk image\n")
				}
			}

			cmd := firecracker.VMCommandBuilder{}.
				WithBin("firecracker").
				WithSocketPath(config.FirecrackerConfig.SocketPath).
				WithStdin(os.Stdin).
				WithStdout(os.Stdout).
				WithStderr(os.Stderr).
				Build(ctx)

			// reset static configuration, as it gets set each time CNI runs
			config.FirecrackerConfig.NetworkInterfaces[0].StaticConfiguration = nil

			m, err := firecracker.NewMachine(ctx, config.FirecrackerConfig, firecracker.WithProcessRunner(cmd))
			if err != nil {
				return fmt.Errorf("failed to create new machine: %w", err)
			}

			if err := m.Start(ctx); err != nil {
				return fmt.Errorf("failed to initialize machine: %w", err)
			}

			waitCh := make(chan error)

			go func() {
				waitCh <- m.Wait(ctx)
			}()

			select {
			case err := <-waitCh:
				if err != nil {
					return fmt.Errorf("failed running VM: %w", err)
				}

				select {
				case sig := <-c:
					fmt.Fprintf(os.Stderr, "exiting VM as signal %s was received\n", sig)

					return fmt.Errorf("process stopped")

				case <-time.After(500 * time.Millisecond): // wait a bit to prevent crash loop
				}
			case sig := <-c:
				fmt.Fprintf(os.Stderr, "stopping VM as signal %s was received\n", sig)

				m.StopVMM() //nolint: errcheck

				<-waitCh // wait for process to exit

				return fmt.Errorf("process stopped")
			}

			// restart the vm by proceeding with the for loop
			os.Remove(config.FirecrackerConfig.SocketPath) //nolint: errcheck

			return nil
		}()
		if err != nil {
			return err
		}
	}
}
