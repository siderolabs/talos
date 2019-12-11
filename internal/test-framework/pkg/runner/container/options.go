// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package container

import (
	// How dumb, moby/moby still uses the following import
	"context"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"

	"github.com/talos-systems/talos/pkg/constants"
)

// Option allows for the configuration of the ContainerRunner.
type Option func(*ContainerRunner) error

// defaultOptions sets a default set of options to be used for the container.
func defaultOptions() *ContainerRunner {
	return &ContainerRunner{
		Container: &container.Config{
			Image: constants.KubernetesImage + ":v" + constants.DefaultKubernetesVersion,
		},
		Host: &container.HostConfig{},
	}
}

// WithImage specifies the image to be used.
func WithImage(o string) Option {
	return func(cr *ContainerRunner) (err error) {
		cr.Container.Image = o
		return err
	}
}

// WithTTY enables or disables the TTY.
func WithTTY(o bool) Option {
	return func(cr *ContainerRunner) (err error) {
		cr.Container.Tty = o
		return err
	}
}

// WithMount configures volume mounts to the container.
func WithMount(o mount.Mount) Option {
	return func(cr *ContainerRunner) (err error) {
		cr.Host.Mounts = append(cr.Host.Mounts, o)
		return err
	}
}

// WithEnv sets environment variables for the container.
func WithEnv(o string) Option {
	return func(cr *ContainerRunner) (err error) {
		cr.Container.Env = append(cr.Container.Env, o)
		return err
	}
}

// WithLabel sets labels on the container.
func WithLabel(k string, v string) Option {
	return func(cr *ContainerRunner) (err error) {
		if cr.Container.Labels == nil {
			cr.Container.Labels = make(map[string]string)
		}

		cr.Container.Labels[k] = v

		return err
	}
}

// WithClusterName sets the cluster name.
func WithClusterName(o string) Option {
	return func(cr *ContainerRunner) (err error) {
		cr.ClusterName = o
		return err
	}
}

// WithClient instantiates a container runtime client with the specified client
// options.
func WithClient(o ...client.Opt) Option {
	return func(cr *ContainerRunner) (err error) {
		var cli *client.Client
		cli, err = client.NewClientWithOpts(o...)
		if err != nil {
			return err
		}

		cr.Client = cli

		// Ensure our client can talk to docker daemon
		cr.Client.NegotiateAPIVersion(context.Background())

		return err
	}
}
