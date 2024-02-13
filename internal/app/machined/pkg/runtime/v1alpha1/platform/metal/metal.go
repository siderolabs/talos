// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package metal contains the metal implementation of the [platform.Platform].
package metal

import (
	"context"
	stderrors "errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/channel"
	"github.com/siderolabs/go-blockdevice/blockdevice/filesystem"
	"github.com/siderolabs/go-blockdevice/blockdevice/probe"
	"github.com/siderolabs/go-pointer"
	"github.com/siderolabs/go-procfs/procfs"
	"github.com/siderolabs/go-retry/retry"
	"golang.org/x/sys/unix"
	"gopkg.in/yaml.v3"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/internal/netutils"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/metal/oauth2"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/metal/url"
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
	return constants.PlatformMetal
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

	getURL := func(ctx context.Context) (string, error) {
		// give a shorter timeout to populate the URL, leave the rest of the time to the actual download
		ctx, cancel := context.WithTimeout(ctx, constants.ConfigLoadAttemptTimeout/2)
		defer cancel()

		downloadEndpoint, err := url.Populate(ctx, *option, r)
		if err != nil {
			log.Printf("failed to populate talos.config fetch URL %q: %s", *option, err.Error())
		}

		log.Printf("fetching machine config from: %q", downloadEndpoint)

		return downloadEndpoint, nil
	}

	switch *option {
	case constants.MetalConfigISOLabel:
		return readConfigFromISO()
	default:
		if err := netutils.Wait(ctx, r); err != nil {
			return nil, err
		}

		oauth2Cfg, err := oauth2.NewConfig(procfs.ProcCmdline(), *option)
		if err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to parse OAuth2 config: %w", err)
		}

		var extraHeaders map[string]string

		// perform OAuth2 device auth flow first to acquire extra headers
		if oauth2Cfg != nil {
			if err = retry.Constant(constants.ConfigLoadTimeout, retry.WithUnits(30*time.Second)).RetryWithContext(ctx, func(ctx context.Context) error {
				return oauth2Cfg.DeviceAuthFlow(ctx, r)
			}); err != nil {
				return nil, fmt.Errorf("OAuth2 device auth flow failed: %w", err)
			}

			extraHeaders = oauth2Cfg.ExtraHeaders()
		}

		return download.Download(
			ctx,
			*option,
			download.WithEndpointFunc(getURL),
			download.WithTimeout(constants.ConfigLoadTimeout),
			download.WithRetryOptions(
				// give a timeout per attempt, max 50% of that is dedicated for URL interpolation, the rest is for the actual download
				retry.WithAttemptTimeout(constants.ConfigLoadAttemptTimeout),
			),
			download.WithHeaders(extraHeaders),
		)
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
		return nil, stderrors.New("error while substituting filesystem type")
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
		cfg.Metadata = pointer.To(metadata.DeepCopy())

		if !channel.SendWithContext(ctx, ch, &cfg) {
			return ctx.Err()
		}
	}
}
