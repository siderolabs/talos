// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package container contains the Container implementation of the [platform.Platform].
package container

import (
	"context"
	"encoding/base64"
	"log"
	"os"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-procfs/procfs"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/container/internal/files"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// Container is a platform for installing Talos via an Container image.
type Container struct{}

// Name implements the platform.Platform interface.
func (c *Container) Name() string {
	return "container"
}

// Configuration implements the platform.Platform interface.
func (c *Container) Configuration(context.Context, state.State) ([]byte, error) {
	log.Printf("fetching machine config from: USERDATA environment variable")

	s := os.Getenv("USERDATA")
	if s == "" {
		return nil, errors.ErrNoConfigSource
	}

	decoded, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}

	return decoded, nil
}

// Mode implements the platform.Platform interface.
func (c *Container) Mode() runtime.Mode {
	return runtime.ModeContainer
}

// KernelArgs implements the runtime.Platform interface.
func (c *Container) KernelArgs(string, quirks.Quirks) procfs.Parameters {
	return nil
}

// NetworkConfiguration implements the runtime.Platform interface.
func (c *Container) NetworkConfiguration(ctx context.Context, _ state.State, ch chan<- *runtime.PlatformNetworkConfig) error {
	networkConfig := &runtime.PlatformNetworkConfig{}

	hostnameSpec, err := files.ReadHostname("/etc/hostname")
	if err != nil {
		return err
	}

	networkConfig.Hostnames = append(networkConfig.Hostnames, hostnameSpec)

	resolverSpec, err := files.ReadResolvConf("/etc/resolv.conf")
	if err != nil {
		return err
	}

	if len(resolverSpec.DNSServers) > 0 {
		networkConfig.Resolvers = append(networkConfig.Resolvers, resolverSpec)
	}

	networkConfig.Metadata = &runtimeres.PlatformMetadataSpec{
		Platform:     c.Name(),
		Hostname:     hostnameSpec.FQDN(),
		InstanceType: os.Getenv("TALOSSKU"),
	}

	select {
	case ch <- networkConfig:
	case <-ctx.Done():
		return ctx.Err()
	}

	return nil
}
