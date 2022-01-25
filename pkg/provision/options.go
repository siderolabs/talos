// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package provision

import (
	"io"
	"os"
	"runtime"

	"github.com/talos-systems/talos/pkg/machinery/client"
	clientconfig "github.com/talos-systems/talos/pkg/machinery/client/config"
)

// Option controls Provisioner.
type Option func(o *Options) error

// WithLogWriter sets logging destination.
func WithLogWriter(w io.Writer) Option {
	return func(o *Options) error {
		o.LogWriter = w

		return nil
	}
}

// WithEndpoint specifies endpoint to use when acessing Talos cluster.
func WithEndpoint(endpoint string) Option {
	return func(o *Options) error {
		o.ForceEndpoint = endpoint

		return nil
	}
}

// WithTalosConfig specifies talosconfig to use when acessing Talos cluster.
func WithTalosConfig(talosConfig *clientconfig.Config) Option {
	return func(o *Options) error {
		o.TalosConfig = talosConfig

		return nil
	}
}

// WithTalosClient specifies client to use when acessing Talos cluster.
func WithTalosClient(client *client.Client) Option {
	return func(o *Options) error {
		o.TalosClient = client

		return nil
	}
}

// WithBootlader enables or disables bootloader (bootloader is enabled by default).
func WithBootlader(enabled bool) Option {
	return func(o *Options) error {
		o.BootloaderEnabled = enabled

		return nil
	}
}

// WithUEFI enables or disables UEFI boot on amd64 (default for amd64 is BIOS boot).
func WithUEFI(enabled bool) Option {
	return func(o *Options) error {
		o.UEFIEnabled = enabled

		return nil
	}
}

// WithExtraUEFISearchPaths configures additional search paths to look for UEFI firmware.
func WithExtraUEFISearchPaths(extraUEFISearchPaths []string) Option {
	return func(o *Options) error {
		o.ExtraUEFISearchPaths = extraUEFISearchPaths

		return nil
	}
}

// WithTargetArch specifies target architecture for the cluster.
func WithTargetArch(arch string) Option {
	return func(o *Options) error {
		o.TargetArch = arch

		return nil
	}
}

// WithDockerPorts allows docker provisioner to expose ports on workers.
func WithDockerPorts(ports []string) Option {
	return func(o *Options) error {
		o.DockerPorts = ports

		return nil
	}
}

// WithDockerPortsHostIP sets host IP for docker provisioner to expose ports on workers.
func WithDockerPortsHostIP(hostIP string) Option {
	return func(o *Options) error {
		o.DockerPortsHostIP = hostIP

		return nil
	}
}

// Options describes Provisioner parameters.
type Options struct {
	LogWriter     io.Writer
	TalosConfig   *clientconfig.Config
	TalosClient   *client.Client
	ForceEndpoint string
	TargetArch    string

	// Enable bootloader by booting from disk image after install.
	BootloaderEnabled bool

	// Enable UEFI (for amd64), arm64 can only boot UEFI
	UEFIEnabled bool
	// Configure additional search paths to look for UEFI firmware.
	ExtraUEFISearchPaths []string

	// Expose ports to worker machines in docker provisioner
	DockerPorts       []string
	DockerPortsHostIP string
}

// DefaultOptions returns default options.
func DefaultOptions() Options {
	return Options{
		BootloaderEnabled: true,
		TargetArch:        runtime.GOARCH,
		LogWriter:         os.Stderr,
		DockerPortsHostIP: "0.0.0.0",
	}
}
