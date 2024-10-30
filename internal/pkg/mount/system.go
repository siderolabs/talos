// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mount

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/maps"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	mountv2 "github.com/siderolabs/talos/internal/pkg/mount/v2"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

var (
	unmounters       = map[string]func() error{}
	mountpointsMutex sync.RWMutex
)

// SystemPartitionMount mounts a system partition by the label.
func SystemPartitionMount(ctx context.Context, r runtime.Runtime, logger *log.Logger, label string, opts ...mountv2.NewPointOption) (err error) {
	volumeStatus, err := safe.StateGetByID[*block.VolumeStatus](ctx, r.State().V1Alpha2().Resources(), label)
	if err != nil {
		return fmt.Errorf("error getting volume status %q: %w", label, err)
	}

	if volumeStatus.TypedSpec().Phase != block.VolumePhaseReady {
		return fmt.Errorf("volume %q is not ready (%s)", label, volumeStatus.TypedSpec().Phase)
	}

	volumeConfig, err := safe.StateGetByID[*block.VolumeConfig](ctx, r.State().V1Alpha2().Resources(), label)
	if err != nil {
		return fmt.Errorf("error getting volume config %q: %w", label, err)
	}

	opts = append(opts, mountv2.WithSelinuxLabel(volumeConfig.TypedSpec().Mount.SelinuxLabel))

	mountpoint := mountv2.NewPoint(
		volumeStatus.TypedSpec().MountLocation,
		volumeConfig.TypedSpec().Mount.TargetPath,
		volumeStatus.TypedSpec().Filesystem.String(),
		opts...,
	)

	unmounter, err := mountpoint.Mount(mountv2.WithMountPrinter(logger.Printf))
	if err != nil {
		return err
	}

	// record mount as the resource
	mountStatus := runtimeres.NewMountStatus(v1alpha1.NamespaceName, label)
	mountStatus.TypedSpec().Source = volumeStatus.TypedSpec().MountLocation
	mountStatus.TypedSpec().Target = volumeConfig.TypedSpec().Mount.TargetPath
	mountStatus.TypedSpec().FilesystemType = volumeStatus.TypedSpec().Filesystem.String()
	mountStatus.TypedSpec().Encrypted = volumeStatus.TypedSpec().EncryptionProvider != block.EncryptionProviderNone

	if mountStatus.TypedSpec().Encrypted {
		encryptionProviders := make(map[string]struct{})

		for _, cfg := range volumeConfig.TypedSpec().Encryption.Keys {
			encryptionProviders[cfg.Type.String()] = struct{}{}
		}

		mountStatus.TypedSpec().EncryptionProviders = maps.Keys(encryptionProviders)
	}

	// ignore the error if the MountStatus already exists, as many mounts are silently skipped with the flag SkipIfMounted
	if err = r.State().V1Alpha2().Resources().Create(context.Background(), mountStatus); err != nil && !state.IsConflictError(err) {
		return fmt.Errorf("error creating mount status resource: %w", err)
	}

	mountpointsMutex.Lock()
	defer mountpointsMutex.Unlock()

	unmounters[label] = unmounter

	return nil
}

// SystemPartitionUnmount unmounts a system partition by the label.
func SystemPartitionUnmount(r runtime.Runtime, logger *log.Logger, label string) (err error) {
	mountpointsMutex.RLock()
	unmounter, ok := unmounters[label]
	mountpointsMutex.RUnlock()

	if !ok {
		if logger != nil {
			logger.Printf("unmount skipped")
		}

		return nil
	}

	err = unmounter()
	if err != nil {
		return err
	}

	if err = r.State().V1Alpha2().Resources().Destroy(context.Background(), runtimeres.NewMountStatus(v1alpha1.NamespaceName, label).Metadata()); err != nil {
		return fmt.Errorf("error destroying mount status resource: %w", err)
	}

	mountpointsMutex.Lock()
	delete(unmounters, label)
	mountpointsMutex.Unlock()

	return nil
}
