// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor

import (
	"context"
	"errors"
	"fmt"
	"net"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"unicode/utf8"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"
	"libvirt.org/go/libvirtxml"

	diskhelpers "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/hypervisor/internal/disks"
	domainbuilder "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/hypervisor/internal/domain"
	vmprofile "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/hypervisor/internal/profile"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/internal/cleanup"
	libvirtdomain "github.com/siderolabs/talos/internal/pkg/libvirt/domain"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

// VirtualMachineDomainSpecController renders backend-neutral specs into libvirt XML.
// It reads no machine configuration and performs no libvirt operations; host links are
// observed only to resolve interface link names and aliases.
type VirtualMachineDomainSpecController struct{}

// Name implements controller.Controller interface.
func (ctrl *VirtualMachineDomainSpecController) Name() string {
	return "hypervisor.VirtualMachineDomainSpecController"
}

// Inputs implements controller.Controller interface.
func (ctrl *VirtualMachineDomainSpecController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: hypervisor.NamespaceName,
			Type:      hypervisor.VirtualMachineSpecType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: hypervisor.NamespaceName,
			Type:      hypervisor.VirtualMachinePowerOverrideType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: hypervisor.NamespaceName,
			Type:      hypervisor.VirtualMachineProfileSpecType,
			Kind:      controller.InputWeak,
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
		{
			Namespace: hypervisor.NamespaceName,
			Type:      hypervisor.CloudInitSpecType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: hypervisor.NamespaceName,
			Type:      hypervisor.CloudInitStatusType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: storage.NamespaceName,
			Type:      storage.StoragePoolVolumeSpecType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: storage.NamespaceName,
			Type:      storage.StoragePoolVolumeStatusType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: hypervisor.NamespaceName,
			Type:      hypervisor.VirtualMachineDomainSpecType,
			Kind:      controller.InputDestroyReady,
		},
		{
			Namespace: network.NamespaceName,
			Type:      network.LinkStatusType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *VirtualMachineDomainSpecController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: hypervisor.VirtualMachineDomainSpecType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *VirtualMachineDomainSpecController) Run(ctx context.Context, runtime controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-runtime.EventCh():
		}

		if err := ctrl.reconcile(ctx, runtime, logger); err != nil {
			return err
		}

		runtime.ResetRestartBackoff()
	}
}

func (ctrl *VirtualMachineDomainSpecController) reconcile(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger) error {
	specs, err := safe.ReaderListAll[*hypervisor.VirtualMachineSpec](ctx, r)
	if err != nil {
		return fmt.Errorf("failed to list virtual machine specs: %w", err)
	}

	overrides, err := safe.ReaderListAll[*hypervisor.VirtualMachinePowerOverride](ctx, r)
	if err != nil {
		return fmt.Errorf("failed to list virtual machine power overrides: %w", err)
	}

	byName := make(map[resource.ID]*hypervisor.VirtualMachinePowerOverride, overrides.Len())
	for override := range overrides.All() {
		byName[override.Metadata().ID()] = override
	}

	linkStatuses, err := safe.ReaderListAll[*network.LinkStatus](ctx, r)
	if err != nil {
		return fmt.Errorf("failed to list link statuses: %w", err)
	}

	links := newHostLinks(linkStatuses)

	resolvedDisks, err := listResolvedDisks(ctx, r)
	if err != nil {
		return err
	}

	desired := make(map[resource.ID]struct{}, specs.Len())

	var errs []error

	for vm := range specs.All() {
		name := vm.Metadata().ID()
		desired[name] = struct{}{}

		// COSI tracks attempted modifications even when the callback fails. Keep
		// validation inside the callback so an invalid update preserves the last
		// good domain without preventing unrelated renders and output cleanup.
		if err := safe.WriterModify(
			ctx, r,
			hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, name),
			func(res *hypervisor.VirtualMachineDomainSpec) error {
				return updateRenderedDomain(ctx, r, logger, vm, byName[name], res, links, resolvedDisks)
			},
		); err != nil {
			if isHeldBack(err) {
				logger.Info("virtual machine is held back by its host resources", zap.Error(err))

				continue
			}

			errs = append(errs, fmt.Errorf("failed to write virtual machine domain spec %q: %w", name, err))
		}
	}

	// VirtualMachineController holds a finalizer on every domain spec it has claimed, so an unwanted
	// spec is torn down to ask for that hold back, and destroyed only once it comes.
	return errors.Join(append(errs,
		cleanup.Outputs[*hypervisor.VirtualMachineDomainSpec](ctx, r, "virtual machine domain spec", desired))...)
}

func updateRenderedDomain(ctx context.Context, r controller.Reader, logger *zap.Logger, vm *hypervisor.VirtualMachineSpec,
	override *hypervisor.VirtualMachinePowerOverride,
	domain *hypervisor.VirtualMachineDomainSpec, links hostLinks, disks map[resource.ID]hypervisor.VirtualMachineDiskStatusSpec,
) error {
	name := vm.Metadata().ID()

	powerState, stopRequest := vm.TypedSpec().PowerState, ""
	if override != nil {
		powerState, stopRequest = override.TypedSpec().PowerState, override.TypedSpec().StopRequest
	}

	generation := hypervisor.VirtualMachineEffectiveGeneration(vm, override)

	rendered, renderErr := renderVirtualMachineDomainWithSeed(ctx, r, name, vm.TypedSpec(), links, disks)
	if renderErr != nil {
		retain, checkErr := retainDomainOnStorageOutage(ctx, r, vm, powerState, generation, domain, disks, renderErr)
		if checkErr != nil {
			return checkErr
		}

		if retain {
			return nil
		}

		if domain.TypedSpec().DomainXML == "" {
			return renderErr
		}

		// Keep the attached disks held until the runtime has stopped.
		domain.TypedSpec().PowerState = hypervisorhelpers.PowerStateStopped.String()
		domain.TypedSpec().StopRequest = stopRequest

		// Whatever was asked for, this stop is not one the operator asked for at all: it is one the
		// controller demands to clear a definition it cannot render off the host, and a guest may
		// not refuse it.
		domain.TypedSpec().StopMode = hypervisorhelpers.StopModeForced.String()

		logger.Error("stopping virtual machine: spec cannot be rendered", zap.String("virtual_machine", name), zap.Error(renderErr))

		return nil
	}

	*domain.TypedSpec() = hypervisor.VirtualMachineDomainSpecSpec{
		DomainXML:          rendered.DomainXML,
		PowerState:         powerState,
		Disks:              rendered.Disks,
		CloudInit:          rendered.CloudInit,
		ObservedGeneration: generation,
		StopMode:           stopModeOf(stopRequest),
		StopRequest:        stopRequest,
	}

	return nil
}

// retainDomainOnStorageOutage preserves only an unchanged running definition whose
// disk or seed observation was lost because its pool daemon could not be contacted.
// Readiness remains false: the starter must not use this definition for a new start.
func retainDomainOnStorageOutage(ctx context.Context, r controller.Reader, vm *hypervisor.VirtualMachineSpec, powerState, generation string,
	domain *hypervisor.VirtualMachineDomainSpec, disks map[resource.ID]hypervisor.VirtualMachineDiskStatusSpec, renderErr error,
) (bool, error) {
	if !errors.Is(renderErr, errDiskNotReady) || !runningDomainMatchesVM(vm, powerState, generation, domain) {
		return false, nil
	}

	unavailable, valid, err := outageDisksMatchDomain(ctx, r, vm, domain, disks)
	if err != nil || !valid {
		return false, err
	}

	if vm.TypedSpec().CloudInit == nil {
		return unavailable, nil
	}

	seedUnavailable, seedCurrent, err := retainedCloudInitCurrent(ctx, r, vm, domain)

	return seedCurrent && (unavailable || seedUnavailable), err
}

func runningDomainMatchesVM(vm *hypervisor.VirtualMachineSpec, powerState, generation string, domain *hypervisor.VirtualMachineDomainSpec) bool {
	return vm.Metadata().Phase() == resource.PhaseRunning && powerState == "running" &&
		domain.TypedSpec().PowerState == "running" && domain.TypedSpec().DomainXML != "" &&
		domain.TypedSpec().ObservedGeneration != "" && domain.TypedSpec().ObservedGeneration == generation &&
		len(domain.TypedSpec().Disks) == len(vm.TypedSpec().Disks)
}

// outageDisksMatchDomain checks every attached disk, not only the disk which made rendering fail.
func outageDisksMatchDomain(ctx context.Context, r controller.Reader, vm *hypervisor.VirtualMachineSpec,
	domain *hypervisor.VirtualMachineDomainSpec, disks map[resource.ID]hypervisor.VirtualMachineDiskStatusSpec,
) (unavailable, valid bool, err error) {
	var definition libvirtxml.Domain

	decoded := definition.Unmarshal(domain.TypedSpec().DomainXML) == nil
	if !decoded || !retainedDiskCountMatches(definition, len(vm.TypedSpec().Disks)) {
		return false, false, nil
	}

	attached := make(map[string]struct{}, len(domain.TypedSpec().Disks))
	for _, id := range domain.TypedSpec().Disks {
		attached[id] = struct{}{}
	}

	for i, disk := range vm.TypedSpec().Disks {
		id := hypervisor.VirtualMachineDiskStatusID(vm.Metadata().ID(), disk)
		if _, ok := attached[id]; !ok {
			return false, false, nil
		}

		status, exists := disks[id]

		diskUnavailable, diskValid, diskErr := outageDiskMatchesDomain(ctx, r, vm.Metadata().ID(), disk, status, exists, definition.Devices.Disks[i])
		if diskErr != nil || !diskValid {
			return false, false, diskErr
		}

		unavailable = unavailable || diskUnavailable
	}

	return unavailable, true, nil
}

// outageDiskMatchesDomain checks one attached disk of a definition retained through an outage.
func outageDiskMatchesDomain(ctx context.Context, r controller.Reader, name string, disk hypervisor.VirtualMachineDiskSpec,
	status hypervisor.VirtualMachineDiskStatusSpec, exists bool, attached libvirtxml.DomainDisk,
) (unavailable, valid bool, err error) {
	if !retainedDiskMatchesVM(name, disk, status, exists, attached) {
		return false, false, nil
	}

	if disk.IsEmptyDrive() {
		// An empty drive has nothing an outage can invalidate.
		return false, status.Phase == hypervisor.VirtualMachineDiskPhaseReady, nil
	}

	valid, err = outageDiskBackingCurrent(ctx, r, disk, status)
	if err != nil || !valid {
		return false, false, err
	}

	return status.Phase == hypervisor.VirtualMachineDiskPhaseObservationUnavailable, true, nil
}

func retainedDiskCountMatches(definition libvirtxml.Domain, count int) bool {
	return definition.Devices != nil && len(definition.Devices.Disks) >= count
}

func retainedDiskMatchesVM(name string, disk hypervisor.VirtualMachineDiskSpec,
	status hypervisor.VirtualMachineDiskStatusSpec, exists bool, attached libvirtxml.DomainDisk,
) bool {
	return exists && status.VirtualMachine == name && status.Name == disk.Name && retainedDiskAttached(attached, disk, status)
}

// retainedDiskAttached reports whether an attached drive is the one a disk and its status resolve to.
func retainedDiskAttached(attached libvirtxml.DomainDisk, disk hypervisor.VirtualMachineDiskSpec, status hypervisor.VirtualMachineDiskStatusSpec) bool {
	return (status.SourcePath != "" || disk.IsEmptyDrive()) &&
		attachedSourcePath(attached) == status.SourcePath && attached.Driver != nil && attached.Driver.Type == status.Format &&
		attached.Device == disk.Type && attached.Target != nil && attached.Target.Bus == disk.Bus &&
		(attached.ReadOnly != nil) == status.ReadOnly
}

// attachedSourcePath is the host file an attached drive reads, empty when it has none.
func attachedSourcePath(attached libvirtxml.DomainDisk) string {
	if attached.Source == nil || attached.Source.File == nil {
		return ""
	}

	return attached.Source.File.File
}

func outageDiskBackingCurrent(ctx context.Context, r controller.Reader, disk hypervisor.VirtualMachineDiskSpec,
	status hypervisor.VirtualMachineDiskStatusSpec,
) (bool, error) {
	if (status.Phase != hypervisor.VirtualMachineDiskPhaseReady && status.Phase != hypervisor.VirtualMachineDiskPhaseObservationUnavailable) ||
		status.Pool != disk.Pool {
		return false, nil
	}

	current, err := outagePoolBackingCurrent(ctx, r, disk.Pool, status.Volume)
	if err != nil || !current {
		return false, err
	}

	if disk.Provision.Blank {
		return true, nil
	}

	return outageCopyBackingCurrent(ctx, r, disk, status)
}

func outagePoolBackingCurrent(ctx context.Context, r controller.Reader, id, volume string) (bool, error) {
	pool, err := safe.ReaderGetByID[*storage.StoragePoolStatus](ctx, r, id)
	if state.IsNotFoundError(err) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	return pool.Metadata().Phase() == resource.PhaseRunning &&
		(pool.TypedSpec().Phase == storage.StoragePoolPhaseReady || pool.TypedSpec().Phase == storage.StoragePoolPhaseObservationUnavailable) &&
		!pool.Metadata().Finalizers().Has("storage.StoragePoolController/mutating/backing") &&
		!pool.Metadata().Finalizers().Has(storage.StoragePoolVolumeMutationFinalizer(volume)), nil
}

func outageCopyBackingCurrent(ctx context.Context, r controller.Reader, disk hypervisor.VirtualMachineDiskSpec,
	status hypervisor.VirtualMachineDiskStatusSpec,
) (bool, error) {
	if !retainedCopyMatchesDisk(disk, status) {
		return false, nil
	}

	id := storage.StoragePoolVolumeID(disk.Pool, status.Volume)

	request, err := safe.ReaderGetByID[*storage.StoragePoolVolumeSpec](ctx, r, id)
	if state.IsNotFoundError(err) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	requestCurrent := diskhelpers.ValidatePersistedImageRequest(request, diskhelpers.ControllerName, disk, status.Volume, disk.ImageBindingKey(status.VirtualMachine)) == nil
	if !requestCurrent {
		return false, nil
	}

	volume, err := safe.ReaderGetByID[*storage.StoragePoolVolumeStatus](ctx, r, id)
	if state.IsNotFoundError(err) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	pool, err := safe.ReaderGetByID[*storage.StoragePoolStatus](ctx, r, disk.Pool)
	if err != nil {
		return false, err
	}

	return retainedCopyCompleted(request, volume, pool) && retainedCopyBackingMatches(volume.TypedSpec(), pool.TypedSpec(), status), nil
}

func retainedCopyMatchesDisk(disk hypervisor.VirtualMachineDiskSpec, status hypervisor.VirtualMachineDiskStatusSpec) bool {
	return disk.Provision.FromImage != nil && status.Volume != "" && status.Format == disk.Format &&
		((disk.IsWritableImage() && !status.ReadOnly) ||
			(disk.Type == "cdrom" && status.ReadOnly && status.Image == *disk.Provision.FromImage))
}

func retainedCopyCompleted(request *storage.StoragePoolVolumeSpec, volume *storage.StoragePoolVolumeStatus, pool *storage.StoragePoolStatus) bool {
	observed := volume.TypedSpec().Copy

	return volume.Metadata().Phase() == resource.PhaseRunning && storage.StoragePoolVolumeCopyCompletionMatches(request, pool, observed)
}

func retainedCopyBackingMatches(volume *storage.StoragePoolVolumeStatusSpec, pool *storage.StoragePoolStatusSpec, disk hypervisor.VirtualMachineDiskStatusSpec) bool {
	return (volume.Phase == storage.StoragePoolVolumePhaseReady || volume.Phase == storage.StoragePoolVolumePhaseObservationUnavailable) &&
		volume.Pool == disk.Pool && volume.Name == disk.Volume && volume.Path == disk.SourcePath &&
		pool.TargetPath != "" && volume.Path == filepath.Join(pool.TargetPath, disk.Volume) && volume.Format == disk.Format
}

func retainedCloudInitCurrent(ctx context.Context, r controller.Reader, vm *hypervisor.VirtualMachineSpec,
	domain *hypervisor.VirtualMachineDomainSpec,
) (unavailable, current bool, err error) {
	// The base renderer may have exited on a disk before checking the seed. Do not keep
	// a domain running when its separately backed cloud-init media has changed.
	seed := hypervisor.CloudInitSpecSpec{
		Pool:          vm.TypedSpec().CloudInit.Pool,
		MetaData:      vm.TypedSpec().CloudInit.MetaData,
		UserData:      vm.TypedSpec().CloudInit.UserData,
		NetworkConfig: vm.TypedSpec().CloudInit.NetworkConfig,
	}

	status, pool, id, err := loadCloudInitBacking(ctx, r, vm.Metadata().ID(), seed)
	if err != nil {
		if errors.Is(err, errDiskNotReady) {
			return false, false, nil
		}

		return false, false, err
	}

	asset := status.TypedSpec()

	return pool.TypedSpec().Phase == storage.StoragePoolPhaseObservationUnavailable,
		id == domain.TypedSpec().CloudInit && asset.Phase == hypervisor.CloudInitPhaseReady &&
			pool.Metadata().Phase() == resource.PhaseRunning &&
			(pool.TypedSpec().Phase == storage.StoragePoolPhaseReady || pool.TypedSpec().Phase == storage.StoragePoolPhaseObservationUnavailable) &&
			cloudInitPoolIdentityMatches(pool, asset) &&
			domainAttachesCurrentSeed(domain.TypedSpec().DomainXML, asset), nil
}

func domainAttachesCurrentSeed(domainXML string, asset *hypervisor.CloudInitStatusSpec) bool {
	var definition libvirtxml.Domain
	if err := definition.Unmarshal(domainXML); err != nil {
		return false
	}

	path, err := cloudInitObjectPath(asset)
	if err != nil {
		return false
	}

	for _, disk := range definition.Devices.Disks {
		if seedDiskIsAttached(disk, path) {
			return true
		}
	}

	return false
}

// renderedDomain is one rendered definition and what the render decided about it.
type renderedDomain struct {
	// DomainXML is the definition libvirt is handed.
	DomainXML string
	// Disks are the IDs of the disk statuses the definition attaches, in configuration order.
	Disks []string
	// CloudInit is the ID of the seed status the definition attaches, if any.
	CloudInit string
}

// renderVirtualMachineDomainWithSeed resolves host resources before building and serializing once.
//
//nolint:gocyclo
func renderVirtualMachineDomainWithSeed(
	ctx context.Context, r controller.Reader, name string, spec *hypervisor.VirtualMachineSpecSpec,
	links hostLinks, resolvedDisks map[resource.ID]hypervisor.VirtualMachineDiskStatusSpec,
) (renderedDomain, error) {
	if err := validateVirtualMachineDomainSpec(name, spec); err != nil {
		return renderedDomain{}, err
	}

	profileName := spec.Profile
	if profileName == "" {
		profileName = "host"
	}

	profile, err := safe.ReaderGetByID[*hypervisor.VirtualMachineProfileSpec](ctx, r, profileName)
	if err != nil {
		if state.IsNotFoundError(err) {
			return renderedDomain{}, fmt.Errorf("virtual machine %q: profile %q not found: %w", name, profileName, vmprofile.ErrNotReady)
		}

		return renderedDomain{}, fmt.Errorf("virtual machine %q: profile %q: %w", name, profileName, err)
	}

	if profile.Metadata().Phase() != resource.PhaseRunning {
		return renderedDomain{}, fmt.Errorf("virtual machine %q: profile %q is not available: %w", name, profileName, vmprofile.ErrNotReady)
	}

	if err := vmprofile.Validate(goruntime.GOARCH, profileName, *profile.TypedSpec()); err != nil {
		return renderedDomain{}, fmt.Errorf("virtual machine %q: profile %q: %w", name, profileName, err)
	}

	resolvedLinks, err := resolveInterfaceLinks(name, spec.Interfaces, links)
	if err != nil {
		return renderedDomain{}, err
	}

	var (
		seedID, seedPath string
		seedErr          error
	)
	if spec.CloudInit != nil {
		seedID, seedPath, seedErr = resolveCloudInit(ctx, r, name, spec.CloudInit)
	}

	platform, err := domainbuilder.PlatformFor(goruntime.GOARCH)
	if err != nil {
		return renderedDomain{}, err
	}

	definition, attachedDisks, err := domainbuilder.Build(domainbuilder.Inputs{
		Platform: platform,
		Name:     name,
		Spec:     spec,
		Profile:  profile.TypedSpec(),
		Links:    resolvedLinks,
		Disks:    resolvedDisks,
		SeedPath: seedPath,
	})
	if err != nil {
		return renderedDomain{}, err
	}

	// Base-domain errors take precedence over waiting for the optional seed.
	if seedErr != nil {
		return renderedDomain{}, seedErr
	}

	text, err := definition.Marshal()
	if err != nil {
		return renderedDomain{}, fmt.Errorf("failed to marshal virtual machine %q: %w", name, err)
	}

	return renderedDomain{DomainXML: text, Disks: attachedDisks, CloudInit: seedID}, nil
}

// resolveCloudInit requires the exact projected seed and its current pool backing before rendering.
func resolveCloudInit(ctx context.Context, r controller.Reader, name string, desired *hypervisor.VirtualMachineCloudInitSpec) (string, string, error) {
	seed := hypervisor.CloudInitSpecSpec{Pool: desired.Pool, MetaData: desired.MetaData, UserData: desired.UserData, NetworkConfig: desired.NetworkConfig}
	if seed.Pool == "" || strings.ContainsAny(seed.Pool, "/\\\x00") {
		return "", "", fmt.Errorf("virtual machine %q: invalid cloud-init pool", name)
	}

	status, pool, id, err := loadCloudInitBacking(ctx, r, name, seed)
	if err != nil {
		return "", "", err
	}

	asset := status.TypedSpec()
	if !cloudInitPoolBackingMatches(pool, asset) {
		return "", "", fmt.Errorf("virtual machine %q: cloud-init seed is %w", name, errDiskNotReady)
	}

	path, err := cloudInitObjectPath(asset)
	if err != nil {
		return "", "", fmt.Errorf("virtual machine %q: cloud-init seed: %w: %w", name, errDiskNotReady, err)
	}

	return id, path, nil
}

// loadCloudInitBacking reads the projection, seed and pool backing in dependency order.
func loadCloudInitBacking(
	ctx context.Context, r controller.Reader, name string, seed hypervisor.CloudInitSpecSpec,
) (*hypervisor.CloudInitStatus, *storage.StoragePoolStatus, string, error) {
	pending := func() error {
		return fmt.Errorf("virtual machine %q: cloud-init seed is %w", name, errDiskNotReady)
	}

	projected, err := safe.ReaderGetByID[*hypervisor.CloudInitSpec](ctx, r, name)
	if state.IsNotFoundError(err) {
		return nil, nil, "", pending()
	}

	if err != nil {
		return nil, nil, "", err
	}

	id := hypervisor.CloudInitStatusID(name, seed)

	status, err := safe.ReaderGetByID[*hypervisor.CloudInitStatus](ctx, r, id)
	if state.IsNotFoundError(err) {
		return nil, nil, "", pending()
	}

	if err != nil {
		return nil, nil, "", err
	}

	pool, err := safe.ReaderGetByID[*storage.StoragePoolStatus](ctx, r, seed.Pool)
	if state.IsNotFoundError(err) {
		return nil, nil, "", pending()
	}

	if err != nil {
		return nil, nil, "", err
	}

	if !cloudInitProjectionReady(projected, seed) || !cloudInitAssetCurrent(status, seed, projected.Metadata().Version().String(), name) {
		return nil, nil, "", pending()
	}

	return status, pool, id, nil
}

func cloudInitProjectionReady(projected *hypervisor.CloudInitSpec, seed hypervisor.CloudInitSpecSpec) bool {
	return projected.Metadata().Phase() == resource.PhaseRunning && *projected.TypedSpec() == seed
}

func cloudInitAssetCurrent(status *hypervisor.CloudInitStatus, seed hypervisor.CloudInitSpecSpec, generation, name string) bool {
	asset := status.TypedSpec()

	return status.Metadata().Phase() == resource.PhaseRunning && asset.Phase == hypervisor.CloudInitPhaseReady &&
		asset.InputDigest == seed.InputDigest() && asset.ObservedGeneration == generation &&
		asset.VirtualMachine == name && asset.Pool == seed.Pool && cloudInitAssetNameValid(asset)
}

func cloudInitPoolBackingMatches(pool *storage.StoragePoolStatus, asset *hypervisor.CloudInitStatusSpec) bool {
	return pool != nil && pool.Metadata().Phase() == resource.PhaseRunning &&
		pool.TypedSpec().Phase == storage.StoragePoolPhaseReady && cloudInitPoolIdentityMatches(pool, asset)
}

func cloudInitPoolIdentityMatches(pool *storage.StoragePoolStatus, asset *hypervisor.CloudInitStatusSpec) bool {
	return pool != nil && pool.TypedSpec().VolumeID != "" && pool.TypedSpec().VolumeID == asset.VolumeID &&
		pool.TypedSpec().TargetPath != "" && asset.Pool == pool.Metadata().ID() && asset.Path == pool.TypedSpec().TargetPath &&
		!pool.Metadata().Finalizers().Has("storage.StoragePoolController/mutating/backing") &&
		!pool.Metadata().Finalizers().Has(storage.StoragePoolVolumeMutationFinalizer(asset.Name))
}

func cloudInitAssetNameValid(asset *hypervisor.CloudInitStatusSpec) bool {
	return asset.Name != "" && asset.Name != "." && filepath.Base(asset.Name) == asset.Name && asset.Digest != ""
}

func cloudInitObjectPath(asset *hypervisor.CloudInitStatusSpec) (string, error) {
	if asset.Path == "" || !cloudInitAssetNameValid(asset) {
		return "", fmt.Errorf("invalid pool seed path")
	}

	return filepath.Join(asset.Path, asset.Name), nil
}

// stopModeOf is the stop mode a committed stop request asked for, and empty, which destroys the
// domain, for a stop nothing asked anything of.
func stopModeOf(request string) string {
	mode := hypervisorhelpers.VirtualMachineStopRequestMode(request)
	if mode == hypervisorhelpers.StopModeUnknown {
		return ""
	}

	return mode.String()
}

// listResolvedDisks indexes the published disk statuses by their resource ID, the key
// renderVirtualMachineDisks looks a disk up under.
func listResolvedDisks(ctx context.Context, reader controller.Reader) (map[resource.ID]hypervisor.VirtualMachineDiskStatusSpec, error) {
	diskStatuses, err := safe.ReaderListAll[*hypervisor.VirtualMachineDiskStatus](ctx, reader)
	if err != nil {
		return nil, fmt.Errorf("failed to list virtual machine disk statuses: %w", err)
	}

	resolvedDisks := make(map[resource.ID]hypervisor.VirtualMachineDiskStatusSpec, diskStatuses.Len())

	for status := range diskStatuses.All() {
		// A status which is tearing down is one a domain has not let go of yet: downstream reads it as absent.
		if status.Metadata().Phase() != resource.PhaseRunning {
			continue
		}

		resolvedDisks[status.Metadata().ID()] = *status.TypedSpec()
	}

	return resolvedDisks, nil
}

// errLinkNotFound marks a render waiting on a host link that has not appeared yet, as opposed
// to one that failed. See the call site in reconcile.
var errLinkNotFound = errors.New("host link not found")

// errLinkNotEthernet marks a host link that exists but cannot carry a macvtap.
var errLinkNotEthernet = errors.New("host link is not an Ethernet link")

// errLinkEnslaved marks a host link that is a port of another link, and so cannot carry a
// macvtap: its master already owns the traffic it receives.
var errLinkEnslaved = errors.New("host link is a port of another link")

// errLinkNotBridge marks a host link a bridge attachment cannot add a port to.
var errLinkNotBridge = errors.New("host link is not a bridge")

// errDiskNotReady marks a render waiting on a disk that has not been resolved or provisioned yet.
var errDiskNotReady = domainbuilder.ErrDiskNotReady

// errDiskUnsupported marks a disk nothing on the host will ever make attachable. Unlike
// errDiskNotReady, waiting does not help: the configuration has to change.
var errDiskUnsupported = diskhelpers.ErrDiskUnsupported

// isHeldBack reports whether a render failed on something this controller should wait out rather
// than fail on. It is wider than what renderStage grades as pending: a configuration the host
// cannot satisfy is a permanent obstacle to report, but still no reason to restart the controller.
func isHeldBack(err error) bool {
	return errors.Is(err, errLinkNotFound) || errors.Is(err, errLinkNotEthernet) ||
		errors.Is(err, errLinkEnslaved) || errors.Is(err, errLinkNotBridge) ||
		errors.Is(err, errDiskNotReady) || errors.Is(err, errDiskUnsupported) || errors.Is(err, vmprofile.ErrNotReady)
}

// hostLinks carries what an interface needs to know about the host's links: how to resolve a
// name or alias, and whether what it resolves to can carry the requested attachment.
type hostLinks struct {
	resolver *network.LinkResolver
	links    map[string]hostLink
}

// hostLink is what attachment checks need of a single host link.
type hostLink struct {
	typ         nethelpers.LinkType
	kind        string
	slaveKind   string
	masterIndex uint32
}

// newHostLinks indexes the observed host links for interface resolution.
func newHostLinks(links safe.List[*network.LinkStatus]) hostLinks {
	index := make(map[string]hostLink, links.Len())

	for link := range links.All() {
		index[link.Metadata().ID()] = hostLink{
			typ:         link.TypedSpec().Type,
			kind:        link.TypedSpec().Kind,
			slaveKind:   link.TypedSpec().SlaveKind,
			masterIndex: link.TypedSpec().MasterIndex,
		}
	}

	return hostLinks{
		resolver: network.NewLinkResolver(links.All),
		links:    index,
	}
}

// resolveInterfaceLinks resolves the host link of each interface, in order. It fails on the
// first link the host does not have, or has but cannot attach the interface to: rendering fewer
// interfaces than requested would silently alter the VM definition, so the whole render fails
// and VirtualMachineDomainSpecController withdraws the power intent.
//
// VirtualMachineStatusController renders the same spec to report the obstacle.
func resolveInterfaceLinks(name string, interfaces []hypervisor.VirtualMachineInterfaceSpec, links hostLinks) ([]string, error) {
	resolved := make([]string, 0, len(interfaces))

	for _, iface := range interfaces {
		link, ok := links.resolver.ResolveChecked(iface.Link)
		if !ok {
			return nil, fmt.Errorf("virtual machine %q: interface %q: %w: %q", name, iface.Name, errLinkNotFound, iface.Link)
		}

		if err := checkInterfaceLink(iface, links.links[link]); err != nil {
			return nil, fmt.Errorf("virtual machine %q: interface %q: %w: %q", name, iface.Name, err, iface.Link)
		}

		resolved = append(resolved, link)
	}

	return resolved, nil
}

// checkInterfaceLink reports whether link can carry the interface's attachment.
func checkInterfaceLink(iface hypervisor.VirtualMachineInterfaceSpec, link hostLink) error {
	if iface.Attachment == hypervisorhelpers.VirtualMachineInterfaceAttachmentBridge.String() {
		if link.kind != network.LinkKindBridge {
			return errLinkNotBridge
		}

		return nil
	}

	// Only the type is checked, not the kind: a bond or a VLAN is as good a macvtap lower
	// link as a physical interface, while a loopback or an L3 tunnel is not one at all.
	if link.typ != nethelpers.LinkEther {
		return errLinkNotEthernet
	}

	// A bridge or bond port has its receive handler taken by its master, so a macvtap on it
	// fails at domain start. A VRF port does not: VRF steers traffic without one.
	if link.masterIndex != 0 && link.slaveKind != network.LinkKindVRF {
		return errLinkEnslaved
	}

	return nil
}

func validateVirtualMachineInterfaces(name string, interfaces []hypervisor.VirtualMachineInterfaceSpec) error {
	names := make(map[string]struct{}, len(interfaces))

	for _, iface := range interfaces {
		if err := hypervisorhelpers.ValidateName(iface.Name); err != nil {
			return fmt.Errorf("virtual machine %q: interface %w", name, err)
		}

		if _, exists := names[iface.Name]; exists {
			return fmt.Errorf("virtual machine %q: duplicate interface name %q", name, iface.Name)
		}

		names[iface.Name] = struct{}{}

		if iface.Link == "" {
			return fmt.Errorf("virtual machine %q: interface %q: link is required", name, iface.Name)
		}

		if iface.HardwareAddr != "" {
			addr, err := net.ParseMAC(iface.HardwareAddr)
			if err != nil {
				return fmt.Errorf("virtual machine %q: interface %q: %w", name, iface.Name, err)
			}

			if len(addr) != 6 || addr[0]&1 != 0 {
				return fmt.Errorf("virtual machine %q: interface %q: hardware address %q must be a 6 byte unicast address",
					name, iface.Name, iface.HardwareAddr)
			}
		}
	}

	return nil
}

func validateVirtualMachineDomainName(name string) error {
	// The domain schema's objectNameWithSlash is [^\n]+; slash is allowed.
	if name == "" || strings.ContainsRune(name, '\n') {
		return fmt.Errorf("virtual machine %q: name must be nonempty and contain no newline", name)
	}

	// libvirtxml replaces invalid UTF-8 and XML 1.0 characters with U+FFFD.
	// Reject those inputs instead of silently changing the resource's identity.
	if !utf8.ValidString(name) {
		return fmt.Errorf("virtual machine %q: name must be valid UTF-8", name)
	}

	for _, char := range name {
		if char != '	' && char != '\r' && (char < 0x20 || char == 0xfffe || char == 0xffff) {
			return fmt.Errorf("virtual machine %q: name contains an invalid XML character", name)
		}
	}

	return nil
}

func validateVirtualMachineFirmware(name string, firmware hypervisor.VirtualMachineFirmwareSpec) error {
	switch firmware.Type {
	case "bios":
		platform, err := domainbuilder.PlatformFor(goruntime.GOARCH)
		if err != nil {
			return err
		}

		if platform.RequiresUEFI() {
			return fmt.Errorf("virtual machine %q: arm64 guests require UEFI", name)
		}

		if firmware.SecureBoot {
			return fmt.Errorf("virtual machine %q: secure boot requires UEFI", name)
		}
	case "uefi":
	default:
		return fmt.Errorf("virtual machine %q: unsupported firmware type %q", name, firmware.Type)
	}

	return nil
}

func validateVirtualMachineMemory(name string, memory hypervisor.VirtualMachineMemorySpec) error {
	// On 64-bit libvirt, virDomainParseMemory requires memory below 2^63 bytes.
	// Talos supports only 64-bit targets, so the checked value also fits uint.
	if memory.Size == 0 || memory.Size > 1<<63-1024 {
		return fmt.Errorf("virtual machine %q: memory size must be between 1024 and 9223372036854774784 bytes", name)
	}

	// libvirt stores memory in KiB, even when XML specifies bytes. Reject values
	// that would otherwise be silently rounded up by virDomainParseMemory.
	if memory.Size%1024 != 0 {
		return fmt.Errorf("virtual machine %q: memory size must be a multiple of 1024 bytes", name)
	}

	if memory.NUMA == nil {
		return nil
	}

	switch memory.NUMA.Mode {
	case "strict", "preferred", "interleave":
	default:
		return fmt.Errorf("virtual machine %q: unsupported NUMA mode %q", name, memory.NUMA.Mode)
	}

	return nil
}

func validateVirtualMachineCPU(name string, cpu hypervisor.VirtualMachineCPUSpec) error {
	// The domain schema defines countCPU as a nonzero unsigned short.
	if cpu.Count == 0 || cpu.Count > 65535 {
		return fmt.Errorf("virtual machine %q: CPU count must be between 1 and 65535", name)
	}

	if cpu.Limit != 0 && (cpu.Limit < hypervisorhelpers.MinCPULimitMillicores || cpu.Limit > hypervisorhelpers.MaxCPULimitMillicores) {
		return fmt.Errorf("virtual machine %q: CPU limit must be between %d and %d millicores",
			name, hypervisorhelpers.MinCPULimitMillicores, hypervisorhelpers.MaxCPULimitMillicores)
	}

	return errors.Join(validateVirtualMachineCPUTopology(name, cpu), validateVirtualMachineCPUPins(name, cpu))
}

func validateVirtualMachineCPUTopology(name string, cpu hypervisor.VirtualMachineCPUSpec) error {
	topology := cpu.Topology
	if topology == nil {
		return nil
	}

	if topology.Sockets == 0 || topology.Cores == 0 || topology.Threads == 0 {
		return fmt.Errorf("virtual machine %q: CPU topology sockets, cores and threads must all be positive", name)
	}

	// Check the two-factor product first so multiplying by threads cannot overflow.
	cores := uint64(topology.Sockets) * uint64(topology.Cores)
	if cores > uint64(cpu.Count) || cores*uint64(topology.Threads) != uint64(cpu.Count) {
		return fmt.Errorf("virtual machine %q: CPU topology sockets * cores * threads must equal CPU count %d", name, cpu.Count)
	}

	return nil
}

func validateVirtualMachineCPUPins(name string, cpu hypervisor.VirtualMachineCPUSpec) error {
	pinned := map[uint32]struct{}{}

	for _, pin := range cpu.Pins {
		if pin.VCPU >= cpu.Count {
			return fmt.Errorf("virtual machine %q: pinned vCPU %d must be less than CPU count %d", name, pin.VCPU, cpu.Count)
		}

		if _, exists := pinned[pin.VCPU]; exists {
			return fmt.Errorf("virtual machine %q: vCPU %d is pinned more than once", name, pin.VCPU)
		}

		pinned[pin.VCPU] = struct{}{}
	}

	return nil
}

func validateVirtualMachineDomainSpec(name string, spec *hypervisor.VirtualMachineSpecSpec) error {
	if err := validateVirtualMachineDomainName(name); err != nil {
		return err
	}

	if err := validateVirtualMachineCPU(name, spec.CPU); err != nil {
		return err
	}

	if err := validateVirtualMachineMemory(name, spec.Memory); err != nil {
		return err
	}

	switch spec.PowerState {
	case "running", "stopped", "suspended":
	default:
		return fmt.Errorf("virtual machine %q: unsupported power state %q", name, spec.PowerState)
	}

	if err := validateVirtualMachineFirmware(name, spec.Firmware); err != nil {
		return err
	}

	if err := hypervisorhelpers.ValidateVirtualMachineVideo(spec.Video.Model, spec.Video.VRAMMiB); err != nil {
		return fmt.Errorf("virtual machine %q: %w", name, err)
	}

	if err := validateVirtualMachineInterfaces(name, spec.Interfaces); err != nil {
		return err
	}

	return validateVirtualMachineDisks(name, spec.Disks)
}

// validateVirtualMachineDisks checks disk names, which become libvirt device aliases and must
// be unique as those are. Only what the alias needs is checked: a spec may be authored by other
// producers than the configuration, whose stricter naming rule is enforced there.
func validateVirtualMachineDisks(name string, disks []hypervisor.VirtualMachineDiskSpec) error {
	names := make(map[string]struct{}, len(disks))

	for _, disk := range disks {
		if err := libvirtdomain.ValidateAliasName(disk.Name); err != nil {
			return fmt.Errorf("virtual machine %q: disk %w", name, err)
		}

		if _, exists := names[disk.Name]; exists {
			return fmt.Errorf("virtual machine %q: duplicate disk name %q", name, disk.Name)
		}

		names[disk.Name] = struct{}{}
	}

	return nil
}
