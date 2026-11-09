// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	libvirtstorage "github.com/siderolabs/talos/internal/pkg/libvirt/storage"
	"github.com/siderolabs/talos/internal/pkg/libvirt/storage/copyowner"
	configcfg "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

const privateCopyCleanupInterval = time.Minute

// configuredCopyBindings distinguishes unavailable intent from a known empty configuration.
// Runtime requests alone are not an absence proof during startup or reconstruction.
func configuredCopyBindings(ctx context.Context, r controller.Reader) (map[string]map[string]struct{}, string, error) {
	cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.ActiveID)
	if state.IsNotFoundError(err) {
		return nil, "", nil
	}

	if err != nil {
		return nil, "", err
	}

	if cfg.Metadata().Phase() != resource.PhaseRunning || cfg.Config() == nil {
		return nil, "", nil
	}

	configID := cfg.Metadata().Created().String() + "/" + cfg.Metadata().Version().String()

	bindings := map[string]map[string]struct{}{}

	for _, vm := range cfg.Config().VirtualMachineConfigs() {
		for _, disk := range vm.Disks() {
			projected := configuredCopyDisk(disk)

			key := projected.ImageBindingKey(vm.Name())
			if key == "" {
				continue
			}

			if bindings[disk.Pool()] == nil {
				bindings[disk.Pool()] = map[string]struct{}{}
			}

			bindings[disk.Pool()][key] = struct{}{}
		}
	}

	return bindings, configID, nil
}

func configuredCopyDisk(disk configcfg.VirtualMachineDiskConfig) hypervisor.VirtualMachineDiskSpec {
	result := hypervisor.VirtualMachineDiskSpec{
		Name:   disk.Name(),
		Pool:   disk.Pool(),
		Type:   disk.Type().String(),
		Format: disk.Format().String(),
	}
	if image, ok := disk.Provision().FromImage().Get(); ok {
		result.Provision.FromImage = &hypervisor.VirtualMachineDiskFromImageSpec{
			Library: image.Library(),
			File:    image.File(),
			Digest:  image.Digest(),
		}
	}

	return result
}

func copyRetentionFor(retention map[string]*copyowner.Retention, pool string) *copyowner.Retention {
	if existing := retention[pool]; existing != nil {
		return existing
	}

	result := &copyowner.Retention{
		Bindings: map[string]struct{}{},
		Volumes:  map[string]struct{}{},
	}
	retention[pool] = result

	return result
}

func retainVirtualMachineCopies(ctx context.Context, r controller.Reader, retention map[string]*copyowner.Retention) error {
	vms, err := safe.ReaderListAll[*hypervisor.VirtualMachineSpec](ctx, r)
	if err != nil {
		return err
	}

	for vm := range vms.All() {
		for _, disk := range vm.TypedSpec().Disks {
			if key := disk.ImageBindingKey(vm.Metadata().ID()); key != "" {
				copyRetentionFor(retention, disk.Pool).Bindings[key] = struct{}{}
			}
		}
	}

	return nil
}

func (ctrl *StoragePoolVolumeController) privateCopyRetention(ctx context.Context, r controller.Reader,
	configured map[string]map[string]struct{},
) (map[string]*copyowner.Retention, error) {
	retention := map[string]*copyowner.Retention{}
	for pool, keys := range configured {
		current := copyRetentionFor(retention, pool)
		for key := range keys {
			current.Bindings[key] = struct{}{}
		}
	}

	if err := retainVirtualMachineCopies(ctx, r, retention); err != nil {
		return nil, err
	}

	specs, err := safe.ReaderListAll[*storage.StoragePoolVolumeSpec](ctx, r)
	if err != nil {
		return nil, err
	}

	for spec := range specs.All() {
		copyRetentionFor(retention, spec.TypedSpec().Pool).Volumes[spec.TypedSpec().Name] = struct{}{}
	}

	statuses, err := safe.ReaderListAll[*storage.StoragePoolVolumeStatus](ctx, r)
	if err != nil {
		return nil, err
	}

	for status := range statuses.All() {
		copyRetentionFor(retention, status.TypedSpec().Pool).Volumes[status.TypedSpec().Name] = struct{}{}
	}

	if ctrl.copies != nil {
		for _, job := range ctrl.copies.jobs {
			copyRetentionFor(retention, job.spec.Pool).Volumes[job.spec.Name] = struct{}{}
		}
	}

	return retention, nil
}

// cleanupPrivateCopies only uses a held, freshly rechecked backing and known canonical intent.
// Public volumes remain pool-owned even after the VM configuration is withdrawn.
func (ctrl *StoragePoolVolumeController) cleanupPrivateCopies(ctx context.Context, r controller.ReaderWriter,
	logger *zap.Logger, pools map[string]*storage.StoragePoolStatus, held map[string]struct{},
) (bool, error) {
	configured, configID, err := configuredCopyBindings(ctx, r)
	if err != nil {
		return false, err
	}

	now, due := ctrl.privateCopyCleanupDue(configID)
	if !due {
		return false, nil
	}

	retention, err := ctrl.privateCopyRetention(ctx, r, configured)
	if err != nil {
		return false, err
	}

	maintenance := false
	attempted := false

	var errs []error

	for name, pool := range pools {
		if !privateCopyPoolReady(pool) {
			continue
		}

		attempted = true

		collected, collectErr := ctrl.cleanupPrivateCopyPool(ctx, r, logger, pool, *copyRetentionFor(retention, name), held, configID)
		maintenance = maintenance || collected

		if collectErr != nil {
			if !errors.Is(collectErr, errVolumePending) {
				return false, collectErr
			}

			errs = append(errs, collectErr)
		}
	}

	if attempted {
		ctrl.nextPrivateCopyCleanup = now.Add(privateCopyCleanupInterval)
		ctrl.privateCopyConfigID = configID
	} else {
		ctrl.nextPrivateCopyCleanup = time.Time{}
	}

	return maintenance, errors.Join(errs...)
}

// sameCanonicalCopyConfig validates the original resource incarnation/revision,
// not the already captured retention set. No config writer is locked after this read.
func sameCanonicalCopyConfig(ctx context.Context, r controller.Reader, original string) (bool, error) {
	cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.ActiveID)
	if state.IsNotFoundError(err) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	return cfg.Metadata().Phase() == resource.PhaseRunning && cfg.Config() != nil &&
		cfg.Metadata().Created().String()+"/"+cfg.Metadata().Version().String() == original, nil
}

// privateCopyCleanupDue suppresses filesystem sweeps until the deadline or a canonical revision.
// An unavailable canonical configuration disables the maintenance timer entirely.
func (ctrl *StoragePoolVolumeController) privateCopyCleanupDue(configID string) (time.Time, bool) {
	if configID == "" {
		ctrl.nextPrivateCopyCleanup = time.Time{}

		return time.Time{}, false
	}

	now := time.Now()
	if ctrl.privateCopyNow != nil {
		now = ctrl.privateCopyNow()
	}

	return now, configID != ctrl.privateCopyConfigID || !now.Before(ctrl.nextPrivateCopyCleanup)
}

func (ctrl *StoragePoolVolumeController) cleanupPrivateCopyPool(ctx context.Context, r controller.ReaderWriter,
	logger *zap.Logger, pool *storage.StoragePoolStatus, keep copyowner.Retention, held map[string]struct{}, configID string,
) (bool, error) {
	name := pool.Metadata().ID()

	// Skip the hold (and the pool status writes it costs) on an idle pool. A failed
	// hint falls through to the held path, which reports the error properly.
	if collectable, err := copyowner.Collectable(pool.TypedSpec().TargetPath, keep); err == nil && !collectable {
		return false, nil
	}

	// An already excluded backing cannot be collected. Avoid writing a hold
	// that releasePools would immediately remove on every input event.
	if poolHasMutationExclusion(pool) {
		return false, pending("storage pool %q has an active mutation exclusion; private cleanup waits", name)
	}

	if err := ctrl.holdPool(ctx, r, logger, pool); err != nil {
		return false, err
	}

	if err := recheckHeldPool(ctx, r, name, "", pool); err != nil {
		return false, err
	}

	held[name] = struct{}{}

	same, err := sameCanonicalCopyConfig(ctx, r, configID)
	if err != nil {
		return false, err
	}

	if !same {
		return false, pending("canonical configuration changed before private collection in pool %q", name)
	}

	if err := copyowner.Collect(ctx, pool.TypedSpec().TargetPath, keep); err != nil {
		return true, pending("private copies in pool %q: %w", name, err)
	}

	return true, nil
}

func poolHasMutationExclusion(pool *storage.StoragePoolStatus) bool {
	for _, finalizer := range *pool.Metadata().Finalizers() {
		if strings.HasPrefix(finalizer, storage.StoragePoolVolumeMutationFinalizerPrefix) ||
			strings.HasPrefix(finalizer, "storage.StoragePoolController/mutating/") {
			return true
		}
	}

	return false
}

func privateCopyPoolReady(pool *storage.StoragePoolStatus) bool {
	return pool.Metadata().Phase() == resource.PhaseRunning && pool.TypedSpec().Phase == storage.StoragePoolPhaseReady && pool.TypedSpec().TargetPath != ""
}

func privateCopyRetryDelay(delay time.Duration, deadline, now time.Time) time.Duration {
	if deadline.IsZero() {
		return delay
	}

	until := deadline.Sub(now)
	if until <= 0 {
		until = time.Nanosecond
	}

	if delay == 0 || delay > until {
		return until
	}

	return delay
}

const configuredDiskCopyProducer resource.Owner = "hypervisor.VirtualMachineDiskController"

func bindConfiguredCopyOwner(job *copyJob, op *libvirtstorage.CopyOperation) error {
	if job.spec.Copy.Image || job.bindingKey == "" {
		return nil
	}

	configured, err := configuredCopyOwner(job)
	if err != nil || !configured {
		return err
	}

	return op.SetBindingKey(job.bindingKey)
}

// configuredCopyOwner grants immutable recovery authority only to the current
// disk producer's request for the destination observed by its configured VM.
func configuredCopyOwner(job *copyJob) (bool, error) {
	request, err := safe.ReaderGetByID[*storage.StoragePoolVolumeSpec](job.ctx, job.reader, job.id)
	if state.IsNotFoundError(err) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	if !configuredCopyRequestMatches(job, request) {
		return false, nil
	}

	cfg, err := safe.ReaderGetByID[*config.MachineConfig](job.ctx, job.reader, config.ActiveID)
	if state.IsNotFoundError(err) {
		return false, pending("configured copy destination is not current for %q: active configuration is unavailable", job.id)
	}

	if err != nil {
		return false, err
	}

	if cfg.Metadata().Phase() != resource.PhaseRunning || cfg.Config() == nil {
		return false, nil
	}

	return configuredCopyFromConfig(job, cfg)
}

func configuredCopyFromConfig(job *copyJob, cfg *config.MachineConfig) (bool, error) {
	for _, vm := range cfg.Config().VirtualMachineConfigs() {
		for _, disk := range vm.Disks() {
			projected := configuredCopyDisk(disk)
			if projected.Pool != job.spec.Pool || projected.Format != job.spec.Format || projected.ImageBindingKey(vm.Name()) != job.bindingKey {
				continue
			}

			matches, statusErr := configuredCopyDestinationMatches(job, vm.Name(), projected)
			if statusErr != nil || matches {
				return matches, statusErr
			}

			// The request may be observed before its producer publishes disk status.
			// Retry rather than publishing an unbound, nonrecoverable configured copy.
			return false, pending("configured copy destination is not current for %q", job.id)
		}
	}

	return false, nil
}

func configuredCopyRequestMatches(job *copyJob, request *storage.StoragePoolVolumeSpec) bool {
	key, _ := request.Metadata().Labels().Get(libvirtstorage.CopyBindingLabel)

	return request.Metadata().Phase() == resource.PhaseRunning && request.Metadata().Owner() == configuredDiskCopyProducer &&
		request.Metadata().ID() == storage.StoragePoolVolumeID(job.spec.Pool, job.spec.Name) &&
		request.TypedSpec().Copy != nil && !request.TypedSpec().Copy.Image &&
		reflect.DeepEqual(request.TypedSpec(), &job.spec) && key == job.bindingKey
}

func configuredCopyDestinationMatches(job *copyJob, vm string, disk hypervisor.VirtualMachineDiskSpec) (bool, error) {
	status, err := safe.ReaderGetByID[*hypervisor.VirtualMachineDiskStatus](
		job.ctx, job.reader, hypervisor.VirtualMachineDiskStatusID(vm, disk),
	)
	if state.IsNotFoundError(err) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	statusKey, _ := status.Metadata().Labels().Get(libvirtstorage.CopyBindingLabel)

	return status.Metadata().Phase() == resource.PhaseRunning && status.Metadata().Owner() == configuredDiskCopyProducer &&
		statusKey == job.bindingKey && status.TypedSpec().VirtualMachine == vm && status.TypedSpec().Name == disk.Name &&
		status.TypedSpec().Pool == job.spec.Pool && status.TypedSpec().Volume == job.spec.Name, nil
}
