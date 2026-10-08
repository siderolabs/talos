// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/hypervisor/internal/disks"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/hypervisor/internal/volumerequest"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/internal/cleanup"
	libvirtstorage "github.com/siderolabs/talos/internal/pkg/libvirt/storage"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/resources/storage"
	"github.com/siderolabs/talos/pkg/machinery/storagehelpers"
)

const diskControllerName = "hypervisor.VirtualMachineDiskController"

// blankDiskFormats are the formats a volume can be made in: the enum's members less its zero one.
//
// Not VirtualMachineDiskFormatStrings(), and not VirtualMachineDiskFormatString(): enumer puts the
// zero member in its name map, so both accept "unknown", which names no format and which libvirt
// would be handed verbatim.
var blankDiskFormats = []string{
	hypervisorhelpers.VirtualMachineDiskFormatRaw.String(),
	hypervisorhelpers.VirtualMachineDiskFormatQCOW2.String(),
}

// errHoldFailed marks the controller's own failure to hold a library.
var errHoldFailed = errors.New("failed to hold content library")

var errCopyBindingChanged = errors.New("copy binding publication changed; reconcile again")

var errCopyBindingUnavailable = errors.New("copy binding observation unavailable")

// VirtualMachineDiskController resolves each disk of a virtual machine to a host source.
type VirtualMachineDiskController struct {
	images      disks.ImageVerifier
	retrySource bool
}

// diskReconciliation records the dependencies wanted or held during one event. It is rebuilt
// every pass: Ready sources still need observation, and obsolete held statuses are considered
// separately during teardown before any backing-resource holds are released.
type diskReconciliation struct {
	libraries map[string]*hypervisor.ContentLibraryStatus
	wanted    map[resource.ID]struct{}
	held      map[string]struct{}
	volumes   map[resource.ID]struct{}
	bindings  map[resource.ID]copyBindingObservation
	protected map[string]struct{}
}

type copyBindingObservation struct {
	target string
	key    string
	name   string
}

// Name implements controller.Controller interface.
func (ctrl *VirtualMachineDiskController) Name() string {
	return diskControllerName
}

// Inputs implements controller.Controller interface.
func (ctrl *VirtualMachineDiskController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: hypervisor.NamespaceName,
			Type:      hypervisor.VirtualMachineSpecType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: hypervisor.NamespaceName,
			Type:      hypervisor.ContentLibraryStatusType,
			Kind:      controller.InputStrong,
		},
		{
			Namespace: hypervisor.NamespaceName,
			Type:      hypervisor.VirtualMachineDiskStatusType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: storage.NamespaceName,
			Type:      storage.StoragePoolStatusType,
			Kind:      controller.InputWeak,
		},
		// Copy observations and requests whose consumers have detached.
		{
			Namespace: storage.NamespaceName,
			Type:      storage.StoragePoolVolumeStatusType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: storage.NamespaceName,
			Type:      storage.StoragePoolVolumeSpecType,
			Kind:      controller.InputDestroyReady,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *VirtualMachineDiskController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: hypervisor.VirtualMachineDiskStatusType,
			Kind: controller.OutputExclusive,
		},
		{
			Type: storage.StoragePoolVolumeSpecType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *VirtualMachineDiskController) Run(ctx context.Context, runtime controller.Runtime, logger *zap.Logger) error {
	ctrl.images = disks.ImageVerifier{}

	timer := time.NewTimer(time.Hour)
	if !timer.Stop() {
		<-timer.C
	}
	defer timer.Stop()

	var retry <-chan time.Time

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-runtime.EventCh():
		case <-retry:
			retry = nil
		}

		if err := ctrl.reconcile(ctx, runtime, logger); err != nil {
			return err
		}

		retry = scheduleSourceRetry(timer, retry, ctrl.retrySource)

		runtime.ResetRestartBackoff()
	}
}

func scheduleSourceRetry(timer *time.Timer, retry <-chan time.Time, needed bool) <-chan time.Time {
	if needed {
		if retry == nil {
			timer.Reset(time.Second)
		}

		return timer.C
	}

	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}

	return nil
}

func (ctrl *VirtualMachineDiskController) reconcile(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger) error {
	ctrl.retrySource = false

	specs, err := safe.ReaderListAll[*hypervisor.VirtualMachineSpec](ctx, r)
	if err != nil {
		return fmt.Errorf("failed to list virtual machine specs: %w", err)
	}

	libraryStatuses, err := safe.ReaderListAll[*hypervisor.ContentLibraryStatus](ctx, r)
	if err != nil {
		return fmt.Errorf("failed to list content library statuses: %w", err)
	}

	libraries := make(map[string]*hypervisor.ContentLibraryStatus, libraryStatuses.Len())

	for library := range libraryStatuses.All() {
		libraries[library.Metadata().ID()] = library
	}

	pass := &diskReconciliation{
		libraries: libraries,
		wanted:    make(map[resource.ID]struct{}, specs.Len()),
		held:      map[string]struct{}{},
		volumes:   map[resource.ID]struct{}{},
		bindings:  map[resource.ID]copyBindingObservation{},
		protected: map[string]struct{}{},
	}

	var errs []error

	for vm := range specs.All() {
		errs = append(errs, ctrl.reconcileVMDisks(ctx, r, logger, vm, pass)...)
	}

	return errors.Join(append(errs, ctrl.teardown(ctx, r, logger, pass))...)
}

func (ctrl *VirtualMachineDiskController) reconcileVMDisks(
	ctx context.Context, r controller.ReaderWriter, logger *zap.Logger,
	vm *hypervisor.VirtualMachineSpec, pass *diskReconciliation,
) []error {
	if vm.Metadata().Phase() != resource.PhaseRunning {
		return nil
	}

	name := vm.Metadata().ID()

	var errs []error

	for _, disk := range vm.TypedSpec().Disks {
		id := hypervisor.VirtualMachineDiskStatusID(name, disk)
		pass.wanted[id] = struct{}{}

		if err := ctrl.reconcileDisk(ctx, r, logger, name, id, disk, pass); err != nil {
			if errors.Is(err, errCopyBindingUnavailable) || errors.Is(err, errCopyBindingChanged) {
				errs = append(errs, ctrl.protectUnresolvedBinding(ctx, r, logger, name, disk, pass)...)

				if errors.Is(err, errCopyBindingChanged) {
					continue
				}
			}

			errs = append(errs, err)
		}
	}

	return errs
}

func (ctrl *VirtualMachineDiskController) protectUnresolvedBinding(
	ctx context.Context, r controller.ReaderWriter, logger *zap.Logger,
	name string, disk hypervisor.VirtualMachineDiskSpec, pass *diskReconciliation,
) []error {
	image := disk.Provision.FromImage
	if image == nil {
		return nil
	}

	key := imageBindingKey(name, disk)
	pass.protected[key] = struct{}{}

	if err := retainUnresolvedCopyRequests(ctx, r, logger, key, pass); err != nil {
		if library := pass.libraries[image.Library]; library != nil && library.Metadata().Finalizers().Has(ctrl.Name()) {
			pass.held[image.Library] = struct{}{}
		}

		return []error{err}
	}

	return nil
}

// teardown first asks domains to release obsolete disk statuses, then withdraws volume requests,
// then releases libraries. Held statuses continue protecting both kinds of backing resource.
func (ctrl *VirtualMachineDiskController) teardown(
	ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, pass *diskReconciliation,
) error {
	if err := cleanup.Outputs[*hypervisor.VirtualMachineDiskStatus](ctx, r, "virtual machine disk status", pass.wanted); err != nil {
		return err
	}

	if err := ctrl.releaseVolumeSpecs(ctx, r, pass); err != nil {
		return err
	}

	if err := retainOutstandingCopySources(ctx, r, logger, pass); err != nil {
		return err
	}

	return ctrl.releaseLibraries(ctx, r, logger, pass)
}

// releaseVolumeSpecs withdraws requests no longer present in desired VM intent.
// Attached consumers hold the corresponding statuses until confirmed detach.
func (ctrl *VirtualMachineDiskController) releaseVolumeSpecs(
	ctx context.Context, r controller.ReaderWriter, pass *diskReconciliation,
) error {
	requests, err := safe.ReaderListAll[*storage.StoragePoolVolumeSpec](ctx, r)
	if err != nil {
		return fmt.Errorf("failed to list storage pool volume specs: %w", err)
	}

	for request := range requests.All() {
		if requestHasProtectedBinding(request, pass.protected, ctrl.Name()) {
			pass.volumes[request.Metadata().ID()] = struct{}{}
		}
	}

	for request := range requests.All() {
		md := request.Metadata()
		if !volumerequest.Owned(request, ctrl.Name()) {
			continue
		}

		if _, keep := pass.volumes[md.ID()]; keep && md.Phase() == resource.PhaseRunning {
			continue
		}

		if err := teardownPoolVolumeRequest(ctx, r, request); err != nil {
			return err
		}
	}

	return nil
}

func requestHasProtectedBinding(request *storage.StoragePoolVolumeSpec, protected map[string]struct{}, owner resource.Owner) bool {
	key, _ := request.Metadata().Labels().Get(libvirtstorage.CopyBindingLabel)
	_, held := protected[key]

	return held && request.Metadata().Owner() == owner && request.Metadata().Phase() == resource.PhaseRunning
}

func teardownPoolVolumeRequest(ctx context.Context, r controller.ReaderWriter, request *storage.StoragePoolVolumeSpec) error {
	if !volumerequest.Owned(request, diskControllerName) {
		return nil
	}

	ready, err := r.Teardown(ctx, request.Metadata())
	if state.IsNotFoundError(err) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("failed to tear down storage pool volume spec %q: %w", request.Metadata().ID(), err)
	}

	if !ready {
		return nil
	}

	if err := r.Destroy(ctx, request.Metadata()); err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("failed to destroy storage pool volume spec %q: %w", request.Metadata().ID(), err)
	}

	return nil
}

// reconcileDisk publishes the status of one disk of one virtual machine.
func (ctrl *VirtualMachineDiskController) reconcileDisk(
	ctx context.Context,
	r controller.ReaderWriter,
	logger *zap.Logger,
	name string,
	id resource.ID,
	disk hypervisor.VirtualMachineDiskSpec,
	pass *diskReconciliation,
) error {
	blocked, err := diskStatusTearingDown(ctx, r, id)
	if err != nil {
		return err
	}

	if blocked {
		return nil
	}

	// Hold the provisioning source only until storage publishes an independent pool copy.
	resolved, resolveErr := ctrl.progressDisk(ctx, r, logger, name, disk, pass)
	if errors.Is(resolveErr, disks.ErrSourceRetryable) {
		ctrl.retrySource = true
	}

	if errors.Is(resolveErr, errHoldFailed) {
		return fmt.Errorf("failed to resolve virtual machine disk %q: %w", id, resolveErr)
	}

	if err := safe.WriterModify(ctx, r,
		hypervisor.NewVirtualMachineDiskStatus(hypervisor.NamespaceName, id),
		func(res *hypervisor.VirtualMachineDiskStatus) error {
			if res.Metadata().Finalizers().Has(libvirtstorage.CopyBindingPublicationFinalizer) {
				return errCopyBindingChanged
			}

			if err := validateCopyBindingObservation(pass, id); err != nil {
				return err
			}

			updateDiskStatus(res, name, disk, resolved, resolveErr)

			return nil
		},
	); err != nil {
		return fmt.Errorf("failed to write virtual machine disk status %q: %w", id, err)
	}

	if errors.Is(resolveErr, errCopyBindingUnavailable) {
		return resolveErr
	}

	return nil
}

// A status held by a domain while tearing down cannot be written or read as ready.
func diskStatusTearingDown(ctx context.Context, r controller.Reader, id resource.ID) (bool, error) {
	existing, err := safe.ReaderGetByID[*hypervisor.VirtualMachineDiskStatus](ctx, r, id)
	if err != nil {
		if state.IsNotFoundError(err) {
			return false, nil
		}

		return false, fmt.Errorf("failed to get virtual machine disk status %q: %w", id, err)
	}

	return existing.Metadata().Phase() != resource.PhaseRunning, nil
}

func updateDiskStatus(
	res *hypervisor.VirtualMachineDiskStatus, name string, disk hypervisor.VirtualMachineDiskSpec,
	resolved hypervisor.VirtualMachineDiskStatusSpec, resolveErr error,
) {
	previousBinding, _ := res.Metadata().Labels().Get(libvirtstorage.CopyBindingLabel)
	previousVolume := res.TypedSpec().Volume
	stampDiskStatus(res.TypedSpec(), name, disk, resolved, resolveErr)

	if disk.Provision.FromImage == nil {
		res.Metadata().Labels().Delete(libvirtstorage.CopyBindingLabel)

		return
	}

	binding := imageBindingKey(name, disk)
	res.Metadata().Labels().Set(libvirtstorage.CopyBindingLabel, binding)

	if res.TypedSpec().Volume == "" && previousBinding == binding {
		// Keep the observed intent for publication fencing, never as readiness proof.
		res.TypedSpec().Volume = previousVolume
	}
}

func validateCopyBindingObservation(pass *diskReconciliation, id resource.ID) error {
	observation, ok := pass.bindings[id]
	if !ok {
		return nil
	}

	name, err := libvirtstorage.ReadCopyBinding(observation.target, observation.key)
	if err != nil {
		return fmt.Errorf("%w: %w", errCopyBindingUnavailable, err)
	}

	if name == "" {
		return fmt.Errorf("%w: completed binding disappeared", errCopyBindingUnavailable)
	}

	if name != observation.name {
		return errCopyBindingChanged
	}

	return nil
}

// stampDiskStatus fills in what a disk status says about itself, resolved or not.
//
// What the disk was for is stamped outside the resolution, so a status which did not resolve still
// names its image and its volume. The volume matters doubly: it is what tells a pool its volume is
// still in use, and a status which did not resolve has no source path to go on.
func stampDiskStatus(
	status *hypervisor.VirtualMachineDiskStatusSpec,
	name string,
	disk hypervisor.VirtualMachineDiskSpec,
	resolved hypervisor.VirtualMachineDiskStatusSpec,
	resolveErr error,
) {
	*status = resolved
	status.VirtualMachine = name
	status.Name = disk.Name

	if image := disk.Provision.FromImage; image != nil {
		status.Image = *image
	}

	volume := provisionsVolume(disk)

	if volume || disk.Provision.FromImage != nil {
		status.Pool = disk.Pool
	}

	if volume {
		status.Blank = true

		if volumeName, err := blankVolumeName(name, disk); err == nil {
			status.Volume = volumeName
		}
	}

	if resolveErr != nil {
		status.Error = resolveErr.Error()
	}
}

// holdLibrary protects the provisioning source while storage copies it. A library already
// tearing down is refused rather than held, so a new copy cannot prevent its teardown.
func holdLibrary(
	ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, library *hypervisor.ContentLibraryStatus,
) error {
	if library.Metadata().Phase() != resource.PhaseRunning {
		return fmt.Errorf("content library %q is going away", library.Metadata().ID())
	}

	if library.Metadata().Finalizers().Has(diskControllerName) {
		return nil
	}

	if err := r.AddFinalizer(ctx, library.Metadata(), diskControllerName); err != nil {
		return fmt.Errorf("%w %q: %w", errHoldFailed, library.Metadata().ID(), err)
	}

	logger.Info("holding content library for a virtual machine disk", zap.String("content_library", library.Metadata().ID()))

	return nil
}

// releaseLibraries gives back the hold on every library nothing resolves against any more.
func (ctrl *VirtualMachineDiskController) releaseLibraries(
	ctx context.Context, r controller.ReaderWriter, logger *zap.Logger,
	pass *diskReconciliation,
) error {
	for id, library := range pass.libraries {
		if _, used := pass.held[id]; used || !library.Metadata().Finalizers().Has(diskControllerName) {
			continue
		}

		if err := r.RemoveFinalizer(ctx, library.Metadata(), diskControllerName); err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("failed to release content library %q: %w", id, err)
		}

		logger.Info("released content library held for a virtual machine disk", zap.String("content_library", id))
	}

	return nil
}

// progressDisk validates a disk, publishes its backing-volume intent, then observes storage.
// Alias resolution chooses immutable destinations. A recorded completed binding permits
// source-free recovery; only an incomplete initial copy needs a held library.
func (ctrl *VirtualMachineDiskController) progressDisk(
	ctx context.Context,
	r controller.ReaderWriter,
	logger *zap.Logger,
	name string,
	disk hypervisor.VirtualMachineDiskSpec,
	pass *diskReconciliation,
) (resolved hypervisor.VirtualMachineDiskStatusSpec, resolveErr error) {
	if err := checkVirtualMachineDiskSupported(disk); err != nil {
		return hypervisor.VirtualMachineDiskStatusSpec{
			Phase: hypervisor.VirtualMachineDiskPhaseNotReady,
		}, err
	}

	// An empty drive resolves to no host source at all, so it holds neither a library nor a volume:
	// the guest is shown a cdrom with its tray open. Checked before both lookups, which have nothing
	// to look up.
	if isEmptyDrive(disk) {
		return disks.ObserveEmptyDrive(), nil
	}

	if provisionsVolume(disk) {
		request, err := ctrl.requestBlankVolume(ctx, r, name, disk, pass)
		if err != nil {
			return hypervisor.VirtualMachineDiskStatusSpec{
				Phase: hypervisor.VirtualMachineDiskPhaseNotReady,
			}, err
		}

		return disks.ObserveVolume(ctx, r, request)
	}

	image := disk.Provision.FromImage
	binding := imageBindingKey(name, disk)

	knownDestination := false
	defer func() {
		if !knownDestination {
			resolveErr = errors.Join(resolveErr, retainUnresolvedCopyRequests(ctx, r, logger, binding, pass))
		}
	}()

	var (
		source    hypervisor.VirtualMachineDiskStatusSpec
		sourceErr error
	)
	if library := pass.libraries[image.Library]; library != nil {
		source, sourceErr = ctrl.images.Observe(disk, *library.TypedSpec())
	} else {
		sourceErr = fmt.Errorf("%w: content library %q is not configured", disks.ErrSourceWaiting, image.Library)
	}

	volumeName, nameErr := ctrl.imageVolumeName(ctx, r, logger, name, disk, source, sourceErr, binding, pass)
	if nameErr != nil {
		return hypervisor.VirtualMachineDiskStatusSpec{
			Phase: hypervisor.VirtualMachineDiskPhaseNotReady,
		}, nameErr
	}

	knownDestination = true

	request, err := ctrl.requestImageVolume(ctx, r, disk, volumeName, binding, pass)
	if err != nil {
		return hypervisor.VirtualMachineDiskStatusSpec{Phase: hypervisor.VirtualMachineDiskPhaseNotReady, Volume: volumeName}, err
	}

	resolved, err = ctrl.progressImageSource(ctx, r, logger, disk, request, source, sourceErr, pass)
	resolved.Volume = volumeName

	return resolved, err
}

// imageVolumeName selects either the verified source or a recorded completed binding.
func (ctrl *VirtualMachineDiskController) imageVolumeName(
	ctx context.Context, r controller.ReaderWriter, logger *zap.Logger,
	name string, disk hypervisor.VirtualMachineDiskSpec, source hypervisor.VirtualMachineDiskStatusSpec,
	sourceErr error, binding string, pass *diskReconciliation,
) (string, error) {
	if sourceErr == nil {
		return diskVolumeNameForDigest(name, disk, "sha256:"+filepath.Base(source.SourcePath))
	}

	volumeName, err := readImageCopyBinding(ctx, r, name, disk, binding, pass)
	if err != nil || volumeName != "" {
		return volumeName, err
	}

	// Completed copies are independent of their source. Without a binding,
	// permanent rejection must not acquire a new provisioning hold.
	if errors.Is(sourceErr, disks.ErrSourceRejected) {
		return "", sourceErr
	}

	if library := pass.libraries[disk.Provision.FromImage.Library]; library != nil {
		if err := holdLibrary(ctx, r, logger, library); err != nil {
			return "", err
		}

		pass.held[disk.Provision.FromImage.Library] = struct{}{}
	}

	return "", sourceErr
}

func readImageCopyBinding(
	ctx context.Context, r controller.Reader, name string,
	disk hypervisor.VirtualMachineDiskSpec, binding string, pass *diskReconciliation,
) (string, error) {
	// A source-free restart may select only the explicitly recorded completed binding.
	pool, err := safe.ReaderGetByID[*storage.StoragePoolStatus](ctx, r, disk.Pool)
	if err != nil {
		if state.IsNotFoundError(err) {
			return "", nil
		}

		return "", err
	}

	if pool.TypedSpec().TargetPath == "" {
		return "", nil
	}

	volumeName, err := libvirtstorage.ReadCopyBinding(pool.TypedSpec().TargetPath, binding)
	if err != nil {
		return "", fmt.Errorf("%w: %w", errCopyBindingUnavailable, err)
	}

	if volumeName != "" {
		pass.bindings[hypervisor.VirtualMachineDiskStatusID(name, disk)] = copyBindingObservation{
			target: pool.TypedSpec().TargetPath,
			key:    binding,
			name:   volumeName,
		}
	}

	return volumeName, nil
}

// retainUnresolvedCopyRequests preserves lifetime without adopting a guessed destination.
func retainUnresolvedCopyRequests(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, binding string, pass *diskReconciliation) error {
	specs, err := safe.ReaderListAll[*storage.StoragePoolVolumeSpec](ctx, r)
	if err != nil {
		return err
	}

	for spec := range specs.All() {
		key, _ := spec.Metadata().Labels().Get(libvirtstorage.CopyBindingLabel)
		if key == binding && spec.Metadata().Owner() == diskControllerName &&
			spec.Metadata().Phase() == resource.PhaseRunning && spec.TypedSpec().Copy != nil {
			pass.volumes[spec.Metadata().ID()] = struct{}{}
			if err := retainIncompleteCopySource(ctx, r, logger, spec, pass); err != nil {
				return err
			}
		}
	}

	return nil
}

func retainIncompleteCopySource(
	ctx context.Context,
	r controller.ReaderWriter,
	logger *zap.Logger,
	request *storage.StoragePoolVolumeSpec,
	pass *diskReconciliation,
) error {
	if request.TypedSpec().Copy.Source == "" {
		return nil
	}

	status, err := safe.ReaderGetByID[*storage.StoragePoolVolumeStatus](ctx, r, request.Metadata().ID())
	if err != nil && !state.IsNotFoundError(err) {
		return err
	}

	if err == nil {
		ready, readyErr := copyPublicationReady(ctx, r, request, status)
		if readyErr != nil {
			return readyErr
		}

		if ready {
			return nil
		}
	}

	library := pass.libraries[request.TypedSpec().Copy.SourceLibrary]
	if library == nil {
		return fmt.Errorf("cannot establish source library for pending copy %q", request.Metadata().ID())
	}

	if err := holdImageCopySource(ctx, r, logger, library, request); err != nil {
		return err
	}

	pass.held[library.Metadata().ID()] = struct{}{}

	return nil
}

// imageBindingKey identifies stable configured intent, independent of the alias's current object.
func imageBindingKey(name string, disk hypervisor.VirtualMachineDiskSpec) string {
	image := disk.Provision.FromImage

	return fmt.Sprintf("%x", sha256.Sum256([]byte(name+"\x00"+disk.Name+"\x00"+disk.Pool+"\x00"+disk.Format+"\x00"+image.Library+"\x00"+image.File+"\x00"+image.Digest)))
}

func (ctrl *VirtualMachineDiskController) requestImageVolume(
	ctx context.Context, r controller.ReaderWriter, disk hypervisor.VirtualMachineDiskSpec,
	volumeName, binding string, pass *diskReconciliation,
) (*storage.StoragePoolVolumeSpec, error) {
	request := storage.NewStoragePoolVolumeSpec(storage.NamespaceName, storage.StoragePoolVolumeID(disk.Pool, volumeName))
	*request.TypedSpec() = storage.StoragePoolVolumeSpecSpec{
		Pool:   disk.Pool,
		Name:   volumeName,
		Format: disk.Format,
		Copy:   &storage.StoragePoolVolumeCopySpec{},
	}

	pass.volumes[request.Metadata().ID()] = struct{}{}

	if err := safe.WriterModify(ctx, r, request, func(current *storage.StoragePoolVolumeSpec) error {
		if current.Metadata().Phase() != resource.PhaseRunning {
			delete(pass.volumes, request.Metadata().ID())

			return fmt.Errorf("waiting for previous copy request %q to withdraw", request.Metadata().ID())
		}

		if err := volumerequest.EnsureAdoption(current, ctrl.Name(), volumerequest.Destination{
			Pool:   disk.Pool,
			Name:   volumeName,
			Format: disk.Format,
		}); err != nil {
			return fmt.Errorf("volume %q belongs to another request", request.Metadata().ID())
		}

		current.Metadata().Labels().Set(libvirtstorage.CopyBindingLabel, binding)

		return nil
	}); err != nil {
		return nil, err
	}

	persisted, err := safe.ReaderGetByID[*storage.StoragePoolVolumeSpec](ctx, r, request.Metadata().ID())
	if err != nil {
		return nil, err
	}

	if err := validatePersistedImageRequest(persisted, ctrl.Name(), disk, volumeName, binding); err != nil {
		return nil, err
	}

	return persisted, nil
}

func validatePersistedImageRequest(
	persisted *storage.StoragePoolVolumeSpec, owner resource.Owner,
	disk hypervisor.VirtualMachineDiskSpec, volumeName, binding string,
) error {
	if persisted.Metadata().Phase() != resource.PhaseRunning || !volumerequest.Owned(persisted, owner) ||
		persisted.TypedSpec().Copy == nil || persisted.TypedSpec().Pool != disk.Pool ||
		persisted.TypedSpec().Name != volumeName || persisted.TypedSpec().Format != disk.Format {
		return fmt.Errorf("volume %q belongs to another request", persisted.Metadata().ID())
	}

	persistedBinding, _ := persisted.Metadata().Labels().Get(libvirtstorage.CopyBindingLabel)
	if persistedBinding != binding {
		return fmt.Errorf("volume %q has changed copy binding", persisted.Metadata().ID())
	}

	return nil
}

func holdImageCopySource(
	ctx context.Context, r controller.ReaderWriter, logger *zap.Logger,
	library *hypervisor.ContentLibraryStatus, request *storage.StoragePoolVolumeSpec,
) error {
	if request.TypedSpec().Copy.Source != "" {
		if request.TypedSpec().Copy.SourceLibrary == "" || request.TypedSpec().Copy.SourceLibrary != library.Metadata().ID() {
			return fmt.Errorf("cannot rebind source library for pending copy %q", request.Metadata().ID())
		}
	}

	if request.TypedSpec().Copy.Source != "" && library.Metadata().Finalizers().Has(diskControllerName) {
		return nil
	}

	return holdLibrary(ctx, r, logger, library)
}

func (ctrl *VirtualMachineDiskController) progressImageSource(
	ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, disk hypervisor.VirtualMachineDiskSpec,
	request *storage.StoragePoolVolumeSpec, source hypervisor.VirtualMachineDiskStatusSpec, sourceErr error,
	pass *diskReconciliation,
) (hypervisor.VirtualMachineDiskStatusSpec, error) {
	image := disk.Provision.FromImage
	resolved, observeErr := disks.ObserveVolume(ctx, r, request)

	resolved.ReadOnly = disk.Type == hypervisorhelpers.VirtualMachineDiskTypeCDROM.String()

	if observeErr == nil {
		return resolved, nil
	}

	if resolved.Phase == hypervisor.VirtualMachineDiskPhaseObservationUnavailable {
		if sourceErr == nil {
			if err := holdObservedImageSource(ctx, r, logger, request, pass.libraries[image.Library], pass); err != nil {
				return resolved, err
			}
		}

		return resolved, observeErr
	}

	if sourceErr != nil {
		return resolved, sourceErr
	}

	return progressBoundImageSource(ctx, r, logger, disk, request, source, resolved, observeErr, pass)
}

func progressBoundImageSource(
	ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, disk hypervisor.VirtualMachineDiskSpec,
	request *storage.StoragePoolVolumeSpec, source, resolved hypervisor.VirtualMachineDiskStatusSpec,
	observeErr error, pass *diskReconciliation,
) (hypervisor.VirtualMachineDiskStatusSpec, error) {
	image := disk.Provision.FromImage

	current, err := safe.ReaderGetByID[*storage.StoragePoolVolumeSpec](ctx, r, request.Metadata().ID())
	if err != nil {
		return resolved, err
	}

	library := pass.libraries[image.Library]
	if library == nil {
		return resolved, fmt.Errorf("content library %q is not configured", image.Library)
	}

	if err := holdImageCopySource(ctx, r, logger, library, current); err != nil {
		return hypervisor.VirtualMachineDiskStatusSpec{Phase: hypervisor.VirtualMachineDiskPhaseNotReady}, err
	}

	pass.held[image.Library] = struct{}{}

	if current.TypedSpec().Copy.Source != "" && current.TypedSpec().Copy.Source == source.SourcePath {
		return resolved, observeErr
	}

	if current.TypedSpec().Copy.Source != "" {
		return resolved, retireFailedImageCopy(ctx, r, disk, request, current, observeErr, pass)
	}

	if err := bindImageCopySource(ctx, r, request, disk, source); err != nil {
		return hypervisor.VirtualMachineDiskStatusSpec{Phase: hypervisor.VirtualMachineDiskPhaseNotReady}, err
	}

	return resolved, observeErr
}

func holdObservedImageSource(
	ctx context.Context, r controller.ReaderWriter, logger *zap.Logger,
	request *storage.StoragePoolVolumeSpec, library *hypervisor.ContentLibraryStatus, pass *diskReconciliation,
) error {
	if library == nil {
		return nil
	}

	current, err := safe.ReaderGetByID[*storage.StoragePoolVolumeSpec](ctx, r, request.Metadata().ID())
	if err != nil {
		return err
	}

	if err := holdImageCopySource(ctx, r, logger, library, current); err != nil {
		return err
	}

	pass.held[library.Metadata().ID()] = struct{}{}

	return nil
}

func retireFailedImageCopy(
	ctx context.Context, r controller.Reader, disk hypervisor.VirtualMachineDiskSpec,
	request, current *storage.StoragePoolVolumeSpec, observeErr error, pass *diskReconciliation,
) error {
	outcome, readErr := safe.ReaderGetByID[*storage.StoragePoolVolumeStatus](ctx, r, current.Metadata().ID())
	if readErr != nil || !failedCurrentCopy(outcome) {
		return observeErr
	}

	// Retire the old request instead of changing its source under a running copy.
	// If publication races this check, the old request still drains first and the
	// new request adopts the completed immutable copy without overwriting it.
	pool, poolErr := safe.ReaderGetByID[*storage.StoragePoolStatus](ctx, r, disk.Pool)
	if poolErr != nil || pool.TypedSpec().Phase != storage.StoragePoolPhaseReady || pool.TypedSpec().TargetPath == "" {
		return fmt.Errorf("cannot establish copy publication state for %q", request.TypedSpec().Name)
	}

	if outcome.TypedSpec().Copy.Intent != storage.StoragePoolVolumeCopyIntent(current, pool) {
		return observeErr
	}

	absent, err := libvirtstorage.CopyPublicationAbsent(pool.TypedSpec().TargetPath, request.TypedSpec().Name)
	if err != nil {
		return err
	}

	if !absent {
		return volumerequest.ErrPublished
	}

	delete(pass.volumes, request.Metadata().ID())

	return fmt.Errorf("waiting to replace failed copy source for %q", request.Metadata().ID())
}

func failedCurrentCopy(outcome *storage.StoragePoolVolumeStatus) bool {
	return outcome.Metadata().Phase() == resource.PhaseRunning &&
		outcome.TypedSpec().Phase != storage.StoragePoolVolumePhaseReady &&
		outcome.TypedSpec().Copy != nil &&
		outcome.TypedSpec().Copy.Phase == storage.StoragePoolVolumeCopyPhaseFailed
}

func bindImageCopySource(ctx context.Context, r controller.ReaderWriter,
	request *storage.StoragePoolVolumeSpec, disk hypervisor.VirtualMachineDiskSpec,
	source hypervisor.VirtualMachineDiskStatusSpec,
) error {
	return safe.WriterModify(ctx, r, request, func(current *storage.StoragePoolVolumeSpec) error {
		if current.TypedSpec().Copy != nil && current.TypedSpec().Copy.Source != "" && current.TypedSpec().Copy.SourceLibrary != disk.Provision.FromImage.Library {
			return fmt.Errorf("cannot establish source library for pending copy %q", current.Metadata().ID())
		}

		if err := volumerequest.BindImage(current, diskControllerName, volumerequest.Destination{
			Pool: disk.Pool,
			Name: request.TypedSpec().Name,
		}, source.SourcePath, "sha256:"+filepath.Base(source.SourcePath)); err != nil {
			return fmt.Errorf("volume %q cannot bind image: %w", request.Metadata().ID(), err)
		}

		current.TypedSpec().Copy.SourceLibrary = disk.Provision.FromImage.Library

		return nil
	})
}

// checkVirtualMachineDiskSupported reports whether a disk is one this slice provisions at all, as
// opposed to one that is merely not resolved yet. Kept apart so the status and the render grade it alike.
//
// Machine configuration validation already rejects most of this. It is checked again because
// VirtualMachineSpec is a shared output: another producer may author one, and nothing binds it to
// the rules a machine configuration document is held to.
func checkVirtualMachineDiskSupported(disk hypervisor.VirtualMachineDiskSpec) error {
	if disk.Provision.Blank && disk.Provision.FromImage != nil {
		return fmt.Errorf("%w: provision: blank and fromImage are mutually exclusive", errDiskUnsupported)
	}

	switch disk.Type {
	case hypervisorhelpers.VirtualMachineDiskTypeCDROM.String():
		// A blank cdrom is an empty drive: no volume, no medium, its tray open.
		if isEmptyDrive(disk) {
			return nil
		}

		return checkCDROMDiskSupported(disk)
	case hypervisorhelpers.VirtualMachineDiskTypeDisk.String():
		if !disk.Provision.Blank {
			return fmt.Errorf("%w: a disk is only provisioned from provision.blank today", errDiskUnsupported)
		}

		return checkBlankDiskSupported(disk)
	default:
		return fmt.Errorf("%w: unsupported type %q", errDiskUnsupported, disk.Type)
	}
}

func checkCDROMDiskSupported(disk hypervisor.VirtualMachineDiskSpec) error {
	if disk.Provision.FromImage == nil {
		// Machine configuration validation already requires one of the two of a cdrom; check anyway,
		// so the status is the whole truth about a disk rather than a partial one.
		return fmt.Errorf("%w: a cdrom requires provision.fromImage or provision.blank", errDiskUnsupported)
	}

	if mode := disk.Provision.FromImage.Mode; mode != "" && mode != hypervisorhelpers.VirtualMachineDiskImageModeCopy.String() {
		return fmt.Errorf("%w: unsupported image mode %q for cdrom", errDiskUnsupported, mode)
	}

	if err := storagehelpers.ValidateStoragePoolName(disk.Pool); err != nil {
		return fmt.Errorf("%w: %w", errDiskUnsupported, err)
	}

	if disk.Format != hypervisorhelpers.VirtualMachineDiskFormatRaw.String() {
		return fmt.Errorf("%w: cdrom copy requires raw format", errDiskUnsupported)
	}

	return nil
}

// isEmptyDrive reports whether a disk is a cdrom with no medium in it: one provisioned blank.
func isEmptyDrive(disk hypervisor.VirtualMachineDiskSpec) bool {
	return disk.Type == hypervisorhelpers.VirtualMachineDiskTypeCDROM.String() && disk.Provision.Blank
}

// provisionsVolume reports whether a disk is backed by a blank volume in a storage pool. A blank
// cdrom is not: it is an empty drive, and there is nothing to provision for it.
func provisionsVolume(disk hypervisor.VirtualMachineDiskSpec) bool {
	return disk.Provision.Blank && !isEmptyDrive(disk)
}

// checkBlankDiskSupported reports whether a blank disk describes a volume that can be made.
func checkBlankDiskSupported(disk hypervisor.VirtualMachineDiskSpec) error {
	if disk.Size == 0 {
		return fmt.Errorf("%w: a blank disk requires a size", errDiskUnsupported)
	}

	if !slices.Contains(blankDiskFormats, disk.Format) {
		return fmt.Errorf("%w: unsupported format %q, expected one of %v",
			errDiskUnsupported, disk.Format, blankDiskFormats)
	}

	if err := storagehelpers.ValidateStoragePoolName(disk.Pool); err != nil {
		return fmt.Errorf("%w: %w", errDiskUnsupported, err)
	}

	return nil
}

// blankVolumeName preserves filenames for configuration-valid VM and disk names.
// Resource IDs have no such alphabet or length bound. Those use a separate namespace
// (a leading underscore cannot occur in a legacy name) and the full SHA-256 of the
// ID. The disk name remains validated and separated unambiguously. Digest collisions
// are cryptographically improbable, not mathematically impossible; no ID is truncated.
func blankVolumeName(virtualMachine string, disk hypervisor.VirtualMachineDiskSpec) (string, error) {
	if err := hypervisorhelpers.ValidateName(disk.Name); err != nil {
		return "", fmt.Errorf("%w: disk %w", errDiskUnsupported, err)
	}

	if hypervisorhelpers.ValidateName(virtualMachine) == nil {
		return virtualMachine + "__" + disk.Name + "." + disk.Format, nil
	}

	return fmt.Sprintf("_vm-%x__%s.%s", sha256.Sum256([]byte(virtualMachine)), disk.Name, disk.Format), nil
}

func diskVolumeNameForDigest(virtualMachine string, disk hypervisor.VirtualMachineDiskSpec, resolvedDigest string) (string, error) {
	base, err := blankVolumeName(virtualMachine, disk)
	if err != nil {
		return "", err
	}

	image := disk.Provision.FromImage
	sum := sha256.Sum256([]byte(image.Library + "\x00" + image.File + "\x00" + image.Digest + "\x00" + resolvedDigest))

	return base[:len(base)-len(disk.Format)-1] + fmt.Sprintf("__%x.%s", sum, disk.Format), nil
}

// requestBlankVolume publishes the desired backing resource before any source observation.
// Storage alone provisions the volume; marking it wanted before writing also preserves it when
// the write fails. Nothing here takes ownership of storage's finalizers or the volume file.
func (ctrl *VirtualMachineDiskController) requestBlankVolume(
	ctx context.Context,
	r controller.ReaderWriter,
	name string,
	disk hypervisor.VirtualMachineDiskSpec,
	pass *diskReconciliation,
) (*storage.StoragePoolVolumeSpec, error) {
	volumeName, err := blankVolumeName(name, disk)
	if err != nil {
		return nil, err
	}

	request := disks.BlankVolumeSpec(disk, volumeName)
	id := request.Metadata().ID()
	pass.volumes[id] = struct{}{}

	if err = safe.WriterModify(ctx, r,
		request,
		func(spec *storage.StoragePoolVolumeSpec) error {
			*spec.TypedSpec() = *request.TypedSpec()

			return nil
		},
	); err != nil {
		return nil, fmt.Errorf("failed to ask for storage pool volume %q: %w", id, err)
	}

	return request, nil
}

// Withdrawal is acknowledged only by disappearance of the request or its consumer holds.
// Cancellation of the controller context says nothing about worker drain.
func retainOutstandingCopySources(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, pass *diskReconciliation) error {
	statuses, err := safe.ReaderListAll[*hypervisor.VirtualMachineDiskStatus](ctx, r)
	if err != nil {
		return err
	}

	for status := range statuses.All() {
		pass.retainHeldDiskStatusLibrary(status)
	}

	requests, err := safe.ReaderListAll[*storage.StoragePoolVolumeSpec](ctx, r)
	if err != nil {
		return err
	}

	var errs []error

	for request := range requests.All() {
		if !outstandingCopySource(request) {
			continue
		}

		if err := retainIncompleteCopySource(ctx, r, logger, request, pass); err != nil {
			// An unknown legacy dependency must never release an existing hold by guessing
			// that current VM intent names the original library.
			pass.retainExistingLibraryHolds()

			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

func outstandingCopySource(request *storage.StoragePoolVolumeSpec) bool {
	if !volumerequest.Owned(request, diskControllerName) || request.TypedSpec().Copy == nil || request.TypedSpec().Copy.Source == "" {
		return false
	}

	return request.Metadata().Phase() != resource.PhaseTearingDown || !request.Metadata().Finalizers().Empty()
}

func (pass *diskReconciliation) retainHeldDiskStatusLibrary(status *hypervisor.VirtualMachineDiskStatus) {
	if status.Metadata().Finalizers().Empty() || status.TypedSpec().Phase == hypervisor.VirtualMachineDiskPhaseReady {
		return
	}

	id := status.TypedSpec().Image.Library
	if library := pass.libraries[id]; library != nil && library.Metadata().Finalizers().Has(diskControllerName) {
		pass.held[id] = struct{}{}
	}
}

func (pass *diskReconciliation) retainExistingLibraryHolds() {
	for id, library := range pass.libraries {
		if library.Metadata().Finalizers().Has(diskControllerName) {
			pass.held[id] = struct{}{}
		}
	}
}

func copyPublicationReady(ctx context.Context, r controller.Reader, request *storage.StoragePoolVolumeSpec, status *storage.StoragePoolVolumeStatus) (bool, error) {
	pool, err := safe.ReaderGetByID[*storage.StoragePoolStatus](ctx, r, request.TypedSpec().Pool)
	if state.IsNotFoundError(err) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	return disks.CopyPublicationReady(request, status, pool), nil
}
