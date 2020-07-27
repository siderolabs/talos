// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package provision

import (
	"io"
	"os"

	"github.com/talos-systems/talos/pkg/client"
	"github.com/talos-systems/talos/pkg/client/config"
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
func WithTalosConfig(talosConfig *config.Config) Option {
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

// WithBootladerEmulation enables bootloader emulation.
func WithBootladerEmulation() Option {
	return func(o *Options) error {
		o.BootloaderEmulation = true

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
	TalosConfig   *config.Config
	TalosClient   *client.Client
	ForceEndpoint string

	// Enable bootloader by booting from disk image assets.
	BootloaderEmulation bool

	// Expose ports to worker machines in docker provisioner
	DockerPorts       []string
	DockerPortsHostIP string
}

// DefaultOptions returns default options.
func DefaultOptions() Options {
	return Options{
		LogWriter:         os.Stderr,
		DockerPortsHostIP: "0.0.0.0",
	}
}
