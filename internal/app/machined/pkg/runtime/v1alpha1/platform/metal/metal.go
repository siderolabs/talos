// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package metal contains the metal implementation of the [platform.Platform].
package metal

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/channel"
	"github.com/siderolabs/go-blockdevice/blockdevice/filesystem"
	"github.com/siderolabs/go-blockdevice/blockdevice/probe"
	"github.com/siderolabs/go-procfs/procfs"
	"golang.org/x/sys/unix"
	"gopkg.in/yaml.v3"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/internal/netutils"
	"github.com/siderolabs/talos/internal/pkg/meta"
	"github.com/siderolabs/talos/pkg/download"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

const (
	mnt = "/mnt"
)

// Metal is a discoverer for non-cloud environments.
type Metal struct{}

// Name implements the platform.Platform interface.
func (m *Metal) Name() string {
	return "metal"
}

// Configuration implements the platform.Platform interface.
func (m *Metal) Configuration(ctx context.Context, r state.State) ([]byte, error) {
	var option *string
	if option = procfs.ProcCmdline().Get(constants.KernelParamConfig).First(); option == nil {
		return nil, errors.ErrNoConfigSource
	}

	if *option == constants.ConfigNone {
		return nil, errors.ErrNoConfigSource
	}

	getURL := func() string {
		downloadEndpoint, err := PopulateURLParameters(ctx, *option, r)
		if err != nil {
			log.Printf("failed to populate talos.config fetch URL: %q ; %s", *option, err.Error())
		}

		log.Printf("fetching machine config from: %q", downloadEndpoint)

		return downloadEndpoint
	}

	switch *option {
	case constants.MetalConfigISOLabel:
		return readConfigFromISO()
	default:
		if err := netutils.Wait(ctx, r); err != nil {
			return nil, err
		}

		return download.Download(ctx, *option, download.WithEndpointFunc(getURL))
	}
}

// Mode implements the platform.Platform interface.
func (m *Metal) Mode() runtime.Mode {
	return runtime.ModeMetal
}

func readConfigFromISO() ([]byte, error) {
	dev, err := probe.GetDevWithFileSystemLabel(constants.MetalConfigISOLabel)
	if err != nil {
		return nil, fmt.Errorf("failed to find %s iso: %w", constants.MetalConfigISOLabel, err)
	}

	//nolint:errcheck
	defer dev.Close()

	sb, err := filesystem.Probe(dev.Device().Name())
	if err != nil {
		return nil, err
	}

	if sb == nil {
		return nil, fmt.Errorf("error while substituting filesystem type")
	}

	if err = unix.Mount(dev.Device().Name(), mnt, sb.Type(), unix.MS_RDONLY, ""); err != nil {
		return nil, fmt.Errorf("failed to mount iso: %w", err)
	}

	b, err := os.ReadFile(filepath.Join(mnt, filepath.Base(constants.ConfigPath)))
	if err != nil {
		return nil, fmt.Errorf("read config: %s", err.Error())
	}

	if err = unix.Unmount(mnt, 0); err != nil {
		return nil, fmt.Errorf("failed to unmount: %w", err)
	}

	return b, nil
}

// KernelArgs implements the runtime.Platform interface.
func (m *Metal) KernelArgs() procfs.Parameters {
	return []*procfs.Parameter{
		procfs.NewParameter("console").Append("ttyS0").Append("tty0"),
	}
}

// NetworkConfiguration implements the runtime.Platform interface.
//
//nolint:gocyclo
func (m *Metal) NetworkConfiguration(ctx context.Context, st state.State, ch chan<- *runtime.PlatformNetworkConfig) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	watchCh := make(chan state.Event)

	if err := st.Watch(ctx, hardware.NewSystemInformation(hardware.SystemInformationID).Metadata(), watchCh); err != nil {
		return err
	}

	if err := st.Watch(ctx, runtimeres.NewMetaKey(runtimeres.NamespaceName, runtimeres.MetaKeyTagToID(meta.MetalNetworkPlatformConfig)).Metadata(), watchCh); err != nil {
		return err
	}

	// network config from META partition
	var metaCfg runtime.PlatformNetworkConfig

	// fixed metadata filled by this function
	metadata := &runtimeres.PlatformMetadataSpec{}
	metadata.Platform = m.Name()

	if option := procfs.ProcCmdline().Get(constants.KernelParamHostname).First(); option != nil {
		metadata.Hostname = *option
	}

	for {
		var event state.Event

		select {
		case <-ctx.Done():
			return ctx.Err()
		case event = <-watchCh:
		}

		switch event.Type {
		case state.Errored:
			return fmt.Errorf("watch failed: %w", event.Error)
		case state.Bootstrapped:
			// ignored, should not happen
		case state.Created, state.Updated:
			switch r := event.Resource.(type) {
			case *hardware.SystemInformation:
				metadata.InstanceID = r.TypedSpec().UUID
			case *runtimeres.MetaKey:
				metaCfg = runtime.PlatformNetworkConfig{}

				if err := yaml.Unmarshal([]byte(r.TypedSpec().Value), &metaCfg); err != nil {
					return fmt.Errorf("failed to unmarshal metal network config from META: %w", err)
				}
			}
		case state.Destroyed:
			switch event.Resource.(type) {
			case *hardware.SystemInformation:
				metadata.InstanceID = ""
			case *runtimeres.MetaKey:
				metaCfg = runtime.PlatformNetworkConfig{}
			}
		}

		cfg := metaCfg
		cfg.Metadata = metadata

		if !channel.SendWithContext(ctx, ch, &cfg) {
			return ctx.Err()
		}
	}
}
