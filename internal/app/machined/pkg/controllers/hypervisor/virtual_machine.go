// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/google/uuid"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"
	"libvirt.org/go/libvirtxml"

	machineruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	libvirtdomain "github.com/siderolabs/talos/internal/pkg/libvirt/domain"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/storage"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

const (
	virtqemudServiceID       = "ext-virtqemud"
	virtualMachineLogMountID = "runtime.LogPersistenceController-" + constants.LogMountPoint
)

// VirtualMachineController reconciles running transient domains with virtqemud.
type VirtualMachineController struct {
	V1Alpha1Mode machineruntime.Mode
	Open         func(context.Context) (libvirtdomain.Client, error)

	// verified is the set of disk status IDs whose image was hashed under the hold now on it, so
	// that a hold taken once costs one pass over the image rather than one per reconciliation. A
	// hold does not outlive the process, and an entry is dropped with the hold it stands for.
	verified      map[string]struct{}
	verifiedSeeds map[string]struct{}

	// Rejected acquisitions never reached Start. Keep failed releases distinct
	// from existing attachments across controller retries.
	rejectedVolumeHolds map[string]resource.Pointer
}

// Name implements controller.Controller interface.
func (ctrl *VirtualMachineController) Name() string {
	return "hypervisor.VirtualMachineController"
}

// Inputs implements controller.Controller interface.
func (ctrl *VirtualMachineController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: block.NamespaceName,
			Type:      block.VolumeMountStatusType,
			ID:        optional.Some(virtualMachineLogMountID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: hypervisor.NamespaceName,
			Type:      hypervisor.VirtualMachineDomainSpecType,
			Kind:      controller.InputStrong,
		},
		{
			Namespace: hypervisor.NamespaceName,
			Type:      hypervisor.VirtualMachineSpecType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: hypervisor.NamespaceName,
			Type:      hypervisor.CloudInitSpecType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: network.NamespaceName,
			Type:      network.LinkStatusType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: hardware.NamespaceName,
			Type:      hardware.SystemInformationType,
			ID:        optional.Some(hardware.SystemInformationID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: hardware.NamespaceName,
			Type:      hardware.NUMATopologyType,
			ID:        optional.Some(hardware.NUMATopologyID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: hypervisor.NamespaceName,
			Type:      hypervisor.VirtualMachineDomainStatusType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: hypervisor.NamespaceName,
			Type:      hypervisor.ContentLibraryStatusType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: storage.NamespaceName,
			Type:      storage.StoragePoolVolumeStatusType,
			Kind:      controller.InputStrong,
		},
		{
			Namespace: storage.NamespaceName,
			Type:      storage.StoragePoolStatusType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: hypervisor.NamespaceName,
			Type:      hypervisor.VirtualMachineDiskStatusType,
			Kind:      controller.InputStrong,
		},
		{
			Namespace: hypervisor.NamespaceName,
			Type:      hypervisor.CloudInitStatusType,
			Kind:      controller.InputStrong,
		},
		{
			Namespace: v1alpha1.NamespaceName,
			Type:      v1alpha1.ServiceType,
			ID:        optional.Some(virtqemudServiceID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *VirtualMachineController) Outputs() []controller.Output {
	return nil
}

// Run implements controller.Controller interface.
func (ctrl *VirtualMachineController) Run(ctx context.Context, runtime controller.Runtime, logger *zap.Logger) error {
	if ctrl.V1Alpha1Mode.InContainer() {
		return nil
	}

	// This controller holds nothing yet, whatever a previous Run left behind.
	ctrl.verified = map[string]struct{}{}
	ctrl.verifiedSeeds = map[string]struct{}{}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-runtime.EventCh():
		}

		ready, err := virtqemudReady(ctx, runtime)
		if err != nil {
			return err
		}

		if !ready {
			runtime.ResetRestartBackoff()

			continue
		}

		if err = ctrl.reconcile(ctx, runtime, logger); err != nil {
			return err
		}

		runtime.ResetRestartBackoff()
	}
}

func virtqemudReady(ctx context.Context, reader controller.Reader) (bool, error) {
	service, err := safe.ReaderGetByID[*v1alpha1.Service](ctx, reader, virtqemudServiceID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return false, nil
		}

		return false, fmt.Errorf("get %q service: %w", virtqemudServiceID, err)
	}

	return service.TypedSpec().Running && (service.TypedSpec().Healthy || service.TypedSpec().Unknown), nil
}

//nolint:gocyclo
func (ctrl *VirtualMachineController) reconcile(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger) error {
	specs, err := safe.ReaderListAll[*hypervisor.VirtualMachineDomainSpec](ctx, r)
	if err != nil {
		return fmt.Errorf("failed to list virtual machine domain specs: %w", err)
	}

	known := make(map[string]struct{}, specs.Len())

	for spec := range specs.All() {
		known[spec.Metadata().ID()] = struct{}{}
	}

	// Seed holds require a daemon inventory even when no domain specs remain.

	logReady, err := virtualMachineLogsReady(ctx, r)
	if err != nil {
		return err
	}

	machineUUID, err := getMachineUUID(ctx, r)
	if err != nil {
		return err
	}

	client, err := ctrl.Open(ctx)
	if err != nil {
		return fmt.Errorf("waiting for QEMU daemon: %w", err)
	}
	defer client.Close()

	domains, err := client.Domains()
	if err != nil {
		return fmt.Errorf("failed to list libvirt domains: %w", err)
	}

	if err := ctrl.retryRejectedVolumeHolds(ctx, r); err != nil {
		return err
	}

	byName := make(map[string]libvirtdomain.Domain, len(domains))
	for _, domain := range domains {
		byName[domain.Name] = domain
		known[domain.Name] = struct{}{}
	}

	// Only authoritative inventory can release orphan backing holds. Failure to
	// contact the daemon says nothing about whether a guest is still reading.
	if err = ctrl.releaseOrphanHolds(ctx, r, known); err != nil {
		return err
	}

	if err := ctrl.releaseOrphanSeeds(ctx, r, byName); err != nil {
		return err
	}

	var reconcileErrors error

	for spec := range specs.All() {
		switch err = ctrl.reconcileSpec(ctx, r, logger, client, machineUUID, byName, spec, logReady); {
		case errors.Is(err, errDiskNotReady):
			logger.Info("virtual machine is waiting for its disks",
				zap.String("virtual_machine", spec.Metadata().ID()), zap.Error(err))
		case isPlacementHeld(err):
			// Retried when the host NUMA topology or the definition changes.
			logger.Warn("virtual machine is held back by its host placement",
				zap.String("virtual_machine", spec.Metadata().ID()), zap.Error(err))
		case err != nil:
			reconcileErrors = errors.Join(reconcileErrors, err)
		}
	}

	return reconcileErrors
}

// The existing log persistence controller owns the LOG mount's lifetime. Observe
// it without taking a new mount hold: daemon shutdown must precede LOG teardown.
func virtualMachineLogsReady(ctx context.Context, reader controller.Reader) (bool, error) {
	mount, err := safe.ReaderGetByID[*block.VolumeMountStatus](ctx, reader, virtualMachineLogMountID)
	if state.IsNotFoundError(err) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("get virtual machine log mount: %w", err)
	}

	return mount.Metadata().Phase() == resource.PhaseRunning &&
		mount.TypedSpec().VolumeID == constants.LogVolumeID &&
		mount.TypedSpec().Target == constants.LogMountPoint &&
		!mount.TypedSpec().ReadOnly && !mount.TypedSpec().Detached, nil
}

func getMachineUUID(ctx context.Context, reader controller.Reader) (uuid.UUID, error) {
	machine, err := safe.ReaderGetByID[*hardware.SystemInformation](ctx, reader, hardware.SystemInformationID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to get system information: %w", err)
	}

	machineUUID, err := uuid.Parse(machine.TypedSpec().UUID)
	if err != nil || machineUUID == uuid.Nil {
		return uuid.Nil, fmt.Errorf("invalid machine UUID %q", machine.TypedSpec().UUID)
	}

	return machineUUID, nil
}

func (ctrl *VirtualMachineController) reconcileSpec(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger,
	client libvirtdomain.Client, machineUUID uuid.UUID, domains map[string]libvirtdomain.Domain,
	spec *hypervisor.VirtualMachineDomainSpec, logReady bool,
) error {
	name := spec.Metadata().ID()
	_, exists := domains[name]
	claimed := spec.Metadata().Finalizers().Has(ctrl.Name())

	if spec.Metadata().Phase() != resource.PhaseRunning || spec.TypedSpec().PowerState == "stopped" {
		return ctrl.stopClaimed(ctx, r, logger, client, machineUUID, spec, exists, claimed)
	}

	if spec.TypedSpec().PowerState != hypervisorhelpers.PowerStateRunning.String() {
		return fmt.Errorf("unsupported power state %q for domain %q", spec.TypedSpec().PowerState, name)
	}

	if exists && !claimed {
		return fmt.Errorf("refusing to adopt unclaimed domain %q", name)
	}

	if !logReady {
		return nil
	}

	return ctrl.startSpec(ctx, r, client, machineUUID, spec, claimed)
}

// startSpec brings one domain up on its definition.
func (ctrl *VirtualMachineController) startSpec(ctx context.Context, r controller.ReaderWriter,
	client libvirtdomain.Client, machineUUID uuid.UUID, spec *hypervisor.VirtualMachineDomainSpec, claimed bool,
) error {
	name := spec.Metadata().ID()

	// Held before libvirt is handed the definition: a domain reads its disks from the moment it starts.
	held, err := ctrl.holdDisks(ctx, r, name, spec.TypedSpec().Disks)
	if err != nil {
		return err
	}

	if err := ctrl.verifyHeldDisks(name, held); err != nil {
		return err
	}

	if err := ctrl.holdSeed(ctx, r, spec); err != nil {
		return err
	}

	// Claimed before libvirt is handed the definition, so a domain this controller started is never
	// one the spec has no claim on: the claim says a domain may exist, not that one does.
	if !claimed {
		if err := r.AddFinalizer(ctx, spec.Metadata(), ctrl.Name()); err != nil {
			return fmt.Errorf("failed to claim domain spec %q: %w", name, err)
		}
	}

	if err := currentVMStartIntent(ctx, r, spec); err != nil {
		return err
	}

	// Admitted only when libvirt would create or replace the domain: an unchanged running domain
	// is never stopped because the host inventory changed under it.
	admission := libvirtdomain.WithStartAdmission(func() error {
		return checkDomainPlacement(ctx, r, name, spec.TypedSpec().DomainXML)
	})

	if err := client.Start(libvirtdomain.Domain{Name: name, UUID: libvirtdomain.UUID(machineUUID, name)}, spec.TypedSpec().DomainXML, admission); err != nil {
		return fmt.Errorf("failed to start domain %q: %w", name, err)
	}

	if err := ctrl.releaseDisksExcept(ctx, r, name, spec.TypedSpec().Disks); err != nil {
		return err
	}

	return ctrl.releaseSeedsExcept(ctx, r, name, spec.TypedSpec().CloudInit)
}

// currentVMStartIntent checks the latest VM projection at the final start boundary. Directly
// authored domain specs have no VM projection and retain their existing start contract.
func currentVMStartIntent(ctx context.Context, r controller.Reader, spec *hypervisor.VirtualMachineDomainSpec) error {
	name := spec.Metadata().ID()

	vm, err := safe.ReaderGetByID[*hypervisor.VirtualMachineSpec](ctx, r, name)
	if state.IsNotFoundError(err) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("get virtual machine intent %q: %w", name, err)
	}

	pending := func() error {
		return fmt.Errorf("domain %q: definition does not match current VM intent: %w", name, errDiskNotReady)
	}

	if vm.Metadata().Phase() != resource.PhaseRunning || vm.TypedSpec().PowerState != hypervisorhelpers.PowerStateRunning.String() {
		return pending()
	}

	// A disk status can still be Ready between pool observation loss and its own
	// reconciliation. Reject new starts at the final boundary in that window.
	ready, err := blankDiskPoolsReady(ctx, r, vm.TypedSpec().Disks)
	if err != nil {
		return err
	}

	if !ready {
		return pending()
	}

	match, err := currentVMDefinitionMatches(ctx, r, spec, vm)
	if err != nil {
		return err
	}

	if !match {
		return pending()
	}

	return nil
}

func currentVMDefinitionMatches(ctx context.Context, r controller.Reader, spec *hypervisor.VirtualMachineDomainSpec,
	vm *hypervisor.VirtualMachineSpec,
) (bool, error) {
	name := spec.Metadata().ID()

	links, err := safe.ReaderListAll[*network.LinkStatus](ctx, r)
	if err != nil {
		return false, fmt.Errorf("list links for domain %q: %w", name, err)
	}

	disks, err := listResolvedDisks(ctx, r)
	if err != nil {
		return false, err
	}

	text, attachedDisks, seedID, err := renderVirtualMachineDomainWithSeed(ctx, r, name, vm.TypedSpec(), newHostLinks(links), disks)
	if err != nil {
		if isHeldBack(err) {
			return false, nil
		}

		return false, err
	}

	return matchesVMStartIntent(spec.TypedSpec(), text, attachedDisks, seedID, vm.TypedSpec().PowerState), nil
}

func blankDiskPoolsReady(ctx context.Context, r controller.Reader, disks []hypervisor.VirtualMachineDiskSpec) (bool, error) {
	for _, disk := range disks {
		if !disk.Provision.Blank {
			continue
		}

		pool, err := safe.ReaderGetByID[*storage.StoragePoolStatus](ctx, r, disk.Pool)
		if state.IsNotFoundError(err) {
			return false, nil
		}

		if err != nil {
			return false, err
		}

		if pool.Metadata().Phase() != resource.PhaseRunning || pool.TypedSpec().Phase != storage.StoragePoolPhaseReady {
			return false, nil
		}
	}

	return true, nil
}

func matchesVMStartIntent(spec *hypervisor.VirtualMachineDomainSpecSpec, text string, disks []string, seedID, powerState string) bool {
	return text == spec.DomainXML && slices.Equal(disks, spec.Disks) && seedID == spec.CloudInit && powerState == spec.PowerState
}

// holdSeed pins the status before inspecting mutable library state or handing XML to libvirt.
func (ctrl *VirtualMachineController) holdSeed(ctx context.Context, r controller.ReaderWriter, spec *hypervisor.VirtualMachineDomainSpec) error {
	id := spec.TypedSpec().CloudInit
	if id == "" {
		return nil
	}

	name := spec.Metadata().ID()
	pending := func(reason string) error {
		return fmt.Errorf("domain %q: seed %q is %w: %s", name, id, errDiskNotReady, reason)
	}

	seed, err := safe.ReaderGetByID[*hypervisor.CloudInitStatus](ctx, r, id)
	if state.IsNotFoundError(err) {
		return pending("not published")
	}

	if err != nil {
		return err
	}

	if !seed.Metadata().Finalizers().Has(ctrl.Name()) && seed.Metadata().Phase() != resource.PhaseRunning {
		return pending("tearing down")
	}
	// Do not pin an unpublished/failed seed indefinitely while its producer needs to retire it.
	if seed.TypedSpec().Phase != hypervisor.CloudInitPhaseReady || seed.TypedSpec().VirtualMachine != name {
		return pending("status invalid or not ready")
	}

	if !seed.Metadata().Finalizers().Has(ctrl.Name()) {
		if err := r.AddFinalizer(ctx, seed.Metadata(), ctrl.Name()); err != nil {
			return fmt.Errorf("hold seed %q: %w", id, err)
		}
	}

	return ctrl.verifyHeldSeed(ctx, r, spec, seed)
}

// verifyHeldSeed checks that the held status still points to the attached library asset.
func (ctrl *VirtualMachineController) verifyHeldSeed(ctx context.Context, r controller.Reader, spec *hypervisor.VirtualMachineDomainSpec, seed *hypervisor.CloudInitStatus) error {
	name, id := spec.Metadata().ID(), seed.Metadata().ID()
	pending := func(reason string) error {
		return fmt.Errorf("domain %q: seed %q is %w: %s", name, id, errDiskNotReady, reason)
	}

	asset := seed.TypedSpec()
	if !validHeldSeedAsset(asset, name) {
		return pending("status invalid or not ready")
	}

	library, err := safe.ReaderGetByID[*hypervisor.ContentLibraryStatus](ctx, r, asset.Library)
	if state.IsNotFoundError(err) {
		return pending("library missing")
	}

	if err != nil {
		return err
	}

	if library.Metadata().Phase() != resource.PhaseRunning || library.TypedSpec().Phase != hypervisor.ContentLibraryPhaseReady ||
		library.TypedSpec().Path != asset.Path || library.TypedSpec().VolumeID != asset.VolumeID ||
		library.Metadata().Finalizers().Has(hypervisor.ContentLibraryMutationFinalizer(asset.Name)) {
		return pending("library backing changed or file being mutated")
	}

	return ctrl.verifyAttachedSeed(spec.TypedSpec().DomainXML, id, asset, pending)
}

// validHeldSeedAsset checks the status fields required to locate and verify the ISO.
func validHeldSeedAsset(asset *hypervisor.CloudInitStatusSpec, name string) bool {
	return asset.Phase == hypervisor.CloudInitPhaseReady && asset.VirtualMachine == name && asset.Library != "" && asset.Path != "" && asset.Name != "" &&
		filepath.Base(asset.Name) == asset.Name && asset.Digest != "" && asset.VolumeID != "" && asset.InputDigest != ""
}

// verifyAttachedSeed checks the actual definition and bytes before trusting the asset for this run.
func (ctrl *VirtualMachineController) verifyAttachedSeed(domainXML, id string, asset *hypervisor.CloudInitStatusSpec, pending func(string) error) error {
	path := filepath.Join(asset.Path, asset.Name)

	var domain libvirtxml.Domain
	if err := domain.Unmarshal(domainXML); err != nil {
		return pending("invalid domain XML")
	}

	attached := false

	for _, disk := range domain.Devices.Disks {
		if seedDiskIsAttached(disk, path) {
			attached = true
		}
	}

	if !attached {
		return pending("definition does not attach the seed read-only")
	}

	if _, verified := ctrl.verifiedSeeds[id]; !verified {
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() || uint64(info.Size()) != asset.SizeBytes {
			return pending("seed file missing or wrong size")
		}

		if err := verifyDiskDigest(path, asset.Digest); err != nil {
			return fmt.Errorf("%w: seed %q: %w", errDiskNotReady, id, err)
		}

		ctrl.verifiedSeeds[id] = struct{}{}
	}

	return nil
}

// seedDiskIsAttached recognizes only a read-only CD-ROM backed by the held seed.
func seedDiskIsAttached(disk libvirtxml.DomainDisk, path string) bool {
	return disk.Device == "cdrom" && disk.ReadOnly != nil && disk.Source != nil &&
		disk.Source.File != nil && disk.Source.File.File == path
}

// releaseSeedsExcept releases this VM's old seed holds after its new definition starts or stops.
func (ctrl *VirtualMachineController) releaseSeedsExcept(ctx context.Context, r controller.ReaderWriter, name, keep string) error {
	seeds, err := safe.ReaderListAll[*hypervisor.CloudInitStatus](ctx, r)
	if err != nil {
		return err
	}

	for seed := range seeds.All() {
		md := seed.Metadata()
		if seed.TypedSpec().VirtualMachine != name || md.ID() == keep || !md.Finalizers().Has(ctrl.Name()) {
			continue
		}

		if err := r.RemoveFinalizer(ctx, md, ctrl.Name()); err != nil && !state.IsNotFoundError(err) {
			return err
		}

		delete(ctrl.verifiedSeeds, md.ID())
	}

	return nil
}

// releaseOrphanSeeds drops holds when neither a domain nor a pending running spec needs them.
func (ctrl *VirtualMachineController) releaseOrphanSeeds(ctx context.Context, r controller.ReaderWriter, domains map[string]libvirtdomain.Domain) error {
	seeds, err := safe.ReaderListAll[*hypervisor.CloudInitStatus](ctx, r)
	if err != nil {
		return err
	}

	for seed := range seeds.All() {
		if !seed.Metadata().Finalizers().Has(ctrl.Name()) {
			continue
		}

		keep, err := seedHasDomainOrRunningSpec(ctx, r, domains, seed.TypedSpec().VirtualMachine, seed.Metadata().ID())
		if err != nil {
			return err
		}

		if keep {
			continue
		}

		if err := r.RemoveFinalizer(ctx, seed.Metadata(), ctrl.Name()); err != nil && !state.IsNotFoundError(err) {
			return err
		}

		delete(ctrl.verifiedSeeds, seed.Metadata().ID())
	}

	return nil
}

// seedHasDomainOrRunningSpec retains a hold for an existing domain or a current failed attempt.
func seedHasDomainOrRunningSpec(ctx context.Context, r controller.Reader, domains map[string]libvirtdomain.Domain, name, seedID string) (bool, error) {
	if _, exists := domains[name]; exists {
		return true, nil
	}

	spec, err := safe.ReaderGetByID[*hypervisor.VirtualMachineDomainSpec](ctx, r, name)
	if state.IsNotFoundError(err) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	if spec.Metadata().Phase() != resource.PhaseRunning || spec.TypedSpec().PowerState != hypervisorhelpers.PowerStateRunning.String() ||
		spec.TypedSpec().CloudInit != seedID {
		return false, nil
	}

	return seedMatchesCurrentVM(ctx, r, name, seedID)
}

func seedMatchesCurrentVM(ctx context.Context, r controller.Reader, name, seedID string) (bool, error) {
	vm, err := safe.ReaderGetByID[*hypervisor.VirtualMachineSpec](ctx, r, name)
	if state.IsNotFoundError(err) {
		return true, nil // Externally authored domain definition.
	}

	if err != nil {
		return false, err
	}

	if vm.Metadata().Phase() != resource.PhaseRunning || vm.TypedSpec().PowerState != hypervisorhelpers.PowerStateRunning.String() || vm.TypedSpec().CloudInit == nil {
		return false, nil
	}

	desired := vm.TypedSpec().CloudInit

	return hypervisor.CloudInitStatusID(name, hypervisor.CloudInitSpecSpec{
		Library:       desired.Library,
		MetaData:      desired.MetaData,
		UserData:      desired.UserData,
		NetworkConfig: desired.NetworkConfig,
	}) == seedID, nil
}

// holdDisks takes a hold on every disk status the definition attaches, and reports what it holds.
//
// Every disk is looked at before any is held: a hold taken for a domain which then turns out not to
// be startable is one nothing gives back while it waits.
func (ctrl *VirtualMachineController) holdDisks(
	ctx context.Context, r controller.ReaderWriter, name string, disks []string,
) ([]*hypervisor.VirtualMachineDiskStatus, error) {
	statuses := make([]*hypervisor.VirtualMachineDiskStatus, 0, len(disks))

	for _, id := range disks {
		diskStatus, err := safe.ReaderGetByID[*hypervisor.VirtualMachineDiskStatus](ctx, r, id)

		switch {
		case state.IsNotFoundError(err):
			return nil, fmt.Errorf("domain %q: disk status %q is %w: not published yet", name, id, errDiskNotReady)
		case err != nil:
			return nil, fmt.Errorf("failed to get disk status %q of domain %q: %w", id, name, err)
		}

		// One already held is one this controller is already keeping alive, whatever phase it is in.
		if !diskStatus.Metadata().Finalizers().Has(ctrl.Name()) && diskStatus.Metadata().Phase() != resource.PhaseRunning {
			return nil, fmt.Errorf("domain %q: disk status %q is %w: going away", name, id, errDiskNotReady)
		}

		statuses = append(statuses, diskStatus)
	}

	for _, diskStatus := range statuses {
		if err := ctrl.holdDisk(ctx, r, name, diskStatus); err != nil {
			return nil, err
		}
	}

	if err := ctrl.checkNotBeingReplaced(ctx, r, name, statuses); err != nil {
		return nil, err
	}

	return statuses, nil
}

// holdDisk commits both ownership links before the final mutation-marker check.
func (ctrl *VirtualMachineController) holdDisk(ctx context.Context, r controller.ReaderWriter, name string, disk *hypervisor.VirtualMachineDiskStatus) error {
	if !disk.Metadata().Finalizers().Has(ctrl.Name()) {
		if err := r.AddFinalizer(ctx, disk.Metadata(), ctrl.Name()); err != nil {
			return fmt.Errorf("failed to hold disk status %q of domain %q: %w", disk.Metadata().ID(), name, err)
		}
	}

	return ctrl.holdVolume(ctx, r, disk)
}

func (ctrl *VirtualMachineController) holdVolume(ctx context.Context, r controller.ReaderWriter, disk *hypervisor.VirtualMachineDiskStatus) (retErr error) {
	spec := disk.TypedSpec()
	if spec.Pool == "" || spec.Volume == "" {
		return nil
	}

	id := storage.StoragePoolVolumeID(spec.Pool, spec.Volume)

	volume, err := safe.ReaderGetByID[*storage.StoragePoolVolumeStatus](ctx, r, id)
	if state.IsNotFoundError(err) {
		return fmt.Errorf("volume %q is %w: not published", id, errDiskNotReady)
	}

	if err != nil {
		return err
	}

	// Separate holds let attached disks share a backing volume without early release.
	finalizer := ctrl.Name() + "/" + disk.Metadata().ID()
	if !volume.Metadata().Finalizers().Has(finalizer) {
		if volume.Metadata().Phase() != resource.PhaseRunning {
			return fmt.Errorf("volume %q is %w: going away", id, errDiskNotReady)
		}

		if err := r.AddFinalizer(ctx, volume.Metadata(), finalizer); err != nil {
			return err
		}

		metadata := volume.Metadata()

		defer func() {
			if retErr != nil {
				retErr = ctrl.releaseRejectedVolumeHold(ctx, r, metadata, finalizer, retErr)
			}
		}()
	}

	return checkHeldVolume(ctx, r, id, spec)
}

func (ctrl *VirtualMachineController) releaseRejectedVolumeHold(ctx context.Context, r controller.Writer, metadata resource.Pointer, finalizer string, cause error) error {
	if err := r.RemoveFinalizer(ctx, metadata, finalizer); err != nil && !state.IsNotFoundError(err) {
		if ctrl.rejectedVolumeHolds == nil {
			ctrl.rejectedVolumeHolds = map[string]resource.Pointer{}
		}

		ctrl.rejectedVolumeHolds[finalizer] = metadata

		return fmt.Errorf("failed to release rejected volume hold %q: %w", metadata.ID(), err)
	}

	delete(ctrl.rejectedVolumeHolds, finalizer)

	return cause
}

func (ctrl *VirtualMachineController) retryRejectedVolumeHolds(ctx context.Context, r controller.Writer) error {
	for finalizer, metadata := range ctrl.rejectedVolumeHolds {
		if err := ctrl.releaseRejectedVolumeHold(ctx, r, metadata, finalizer, nil); err != nil {
			return err
		}
	}

	return nil
}

func checkHeldVolume(ctx context.Context, r controller.Reader, id resource.ID, spec *hypervisor.VirtualMachineDiskStatusSpec) error {
	// AddFinalizer permits tearing-down resources. Validate a fresh observation
	// after acquisition, and never release a hold belonging to an existing attachment.
	volume, err := safe.ReaderGetByID[*storage.StoragePoolVolumeStatus](ctx, r, id)
	if err != nil {
		return err
	}

	if volume.Metadata().Phase() != resource.PhaseRunning || volume.TypedSpec().Phase != storage.StoragePoolVolumePhaseReady {
		return fmt.Errorf("volume %q is %w: backing unavailable", id, errDiskNotReady)
	}

	if volume.TypedSpec().Pool != spec.Pool || volume.TypedSpec().Name != spec.Volume ||
		volume.TypedSpec().Path != spec.SourcePath || volume.TypedSpec().Format != spec.Format {
		return fmt.Errorf("volume %q is %w: backing observation changed", id, errDiskNotReady)
	}

	return nil
}

// checkNotBeingReplaced refuses disks whose image is being replaced or volume resized.
//
// Read after the holds above are in place, against a marker the mutator publishes before its
// fresh hold census: whichever wrote second sees the other, so exactly one backs off. This side
// waits and keeps its holds. Storage clears its marker immediately when a hold defers growth,
// rather than waiting for the pin and deadlocking a start.
func (ctrl *VirtualMachineController) checkNotBeingReplaced(
	ctx context.Context, r controller.Reader, name string, statuses []*hypervisor.VirtualMachineDiskStatus,
) error {
	for _, diskStatus := range statuses {
		image := diskStatus.TypedSpec().Image

		if err := checkVolumeNotBeingResized(ctx, r, name, diskStatus.TypedSpec()); err != nil {
			return err
		}

		if image.Library == "" {
			continue
		}

		library, err := safe.ReaderGetByID[*hypervisor.ContentLibraryStatus](ctx, r, image.Library)

		switch {
		case state.IsNotFoundError(err):
			// No library is no marker, and the disk status resolved against one which is gone is
			// refused on its own account elsewhere.
			continue
		case err != nil:
			return fmt.Errorf("failed to get content library %q of domain %q: %w", image.Library, name, err)
		}

		if library.Metadata().Finalizers().Has(hypervisor.ContentLibraryMutationFinalizer(image.File)) {
			return fmt.Errorf("domain %q: disk status %q is %w: its image is being replaced",
				name, diskStatus.Metadata().ID(), errDiskNotReady)
		}
	}

	return nil
}

// checkVolumeNotBeingResized checks the pool's exclusion under the caller's disk hold.
func checkVolumeNotBeingResized(ctx context.Context, r controller.Reader, name string, disk *hypervisor.VirtualMachineDiskStatusSpec) error {
	if disk.Pool == "" || disk.Volume == "" {
		return nil
	}

	pool, err := safe.ReaderGetByID[*storage.StoragePoolStatus](ctx, r, disk.Pool)
	if state.IsNotFoundError(err) {
		return fmt.Errorf("domain %q: disk %q is %w: pool status missing", name, disk.Name, errDiskNotReady)
	}

	if err != nil {
		return fmt.Errorf("get pool status for domain %q: %w", name, err)
	}

	if pool.Metadata().Finalizers().Has("storage.StoragePoolController/mutating/backing") {
		return fmt.Errorf("domain %q: disk %q is %w: pool backing is changing", name, disk.Name, errDiskNotReady)
	}

	if pool.Metadata().Finalizers().Has(storage.StoragePoolVolumeMutationFinalizer(disk.Volume)) {
		return fmt.Errorf("domain %q: disk %q is %w: volume is being resized", name, disk.Name, errDiskNotReady)
	}

	if pool.Metadata().Phase() != resource.PhaseRunning || pool.TypedSpec().Phase != storage.StoragePoolPhaseReady {
		return fmt.Errorf("domain %q: disk %q is %w: pool unavailable", name, disk.Name, errDiskNotReady)
	}

	return nil
}

// verifyHeldDisks rehashes the images of the disks which pinned a digest.
//
// Run under the holds, which is the only point at which the library file a status resolved against
// can no longer change. Each is hashed once per hold: libvirt may be handed a definition attaching
// disks the domain it replaces never read, so "the domain is already running" says nothing.
func (ctrl *VirtualMachineController) verifyHeldDisks(name string, held []*hypervisor.VirtualMachineDiskStatus) error {
	for _, diskStatus := range held {
		id := diskStatus.Metadata().ID()

		if _, done := ctrl.verified[id]; done {
			continue
		}

		spec := diskStatus.TypedSpec()

		if spec.Image.Digest != "" && spec.SourcePath != "" {
			if err := verifyDiskDigest(spec.SourcePath, spec.Image.Digest); err != nil {
				return fmt.Errorf("domain %q: disk %q is %w: %w", name, spec.Name, errDiskNotReady, err)
			}
		}

		ctrl.verified[id] = struct{}{}
	}

	return nil
}

func verifyDiskDigest(sourcePath, expected string) error {
	f, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to open %q: %w", sourcePath, err)
	}

	defer f.Close() //nolint:errcheck

	return verifyDigest(f, expected)
}

// releaseHolds gives back every hold this controller has on a disk status keep does not name.
func (ctrl *VirtualMachineController) releaseHolds(
	ctx context.Context, r controller.ReaderWriter, keep func(*hypervisor.VirtualMachineDiskStatus) bool,
) error {
	diskStatuses, err := safe.ReaderListAll[*hypervisor.VirtualMachineDiskStatus](ctx, r)
	if err != nil {
		return fmt.Errorf("failed to list virtual machine disk statuses: %w", err)
	}

	for diskStatus := range diskStatuses.All() {
		md := diskStatus.Metadata()

		if !md.Finalizers().Has(ctrl.Name()) || keep(diskStatus) {
			continue
		}

		if err := r.RemoveFinalizer(ctx, md, ctrl.Name()); err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("failed to release disk status %q: %w", md.ID(), err)
		}

		delete(ctrl.verified, md.ID())
	}

	return ctrl.releaseVolumeHolds(ctx, r, diskStatuses, keep)
}

func (ctrl *VirtualMachineController) releaseVolumeHolds(
	ctx context.Context,
	r controller.ReaderWriter,
	disks safe.List[*hypervisor.VirtualMachineDiskStatus],
	keep func(*hypervisor.VirtualMachineDiskStatus) bool,
) error {
	statuses := map[string]*hypervisor.VirtualMachineDiskStatus{}

	for disk := range disks.All() {
		statuses[disk.Metadata().ID()] = disk
	}

	volumes, err := safe.ReaderListAll[*storage.StoragePoolVolumeStatus](ctx, r)
	if err != nil {
		return err
	}

	for volume := range volumes.All() {
		for _, finalizer := range *volume.Metadata().Finalizers() {
			if !strings.HasPrefix(finalizer, ctrl.Name()+"/") {
				continue
			}

			id := strings.TrimPrefix(finalizer, ctrl.Name()+"/")
			if disk := volumeHoldDisk(id, statuses); disk != nil && keep(disk) {
				continue
			}

			if err := r.RemoveFinalizer(ctx, volume.Metadata(), finalizer); err != nil && !state.IsNotFoundError(err) {
				return err
			}
		}
	}

	return nil
}

func volumeHoldDisk(id string, statuses map[string]*hypervisor.VirtualMachineDiskStatus) *hypervisor.VirtualMachineDiskStatus {
	if disk, present := statuses[id]; present {
		return disk
	}

	// A missing disk status still has a VM identity in its canonical ID.
	name, _, found := strings.CutLast(id, "/")
	if !found {
		return nil
	}

	disk := hypervisor.NewVirtualMachineDiskStatus(hypervisor.NamespaceName, id)
	disk.TypedSpec().VirtualMachine = name

	return disk
}

// releaseDisksExcept gives back the hold on every disk status of a domain which the definition now
// running does not attach.
func (ctrl *VirtualMachineController) releaseDisksExcept(ctx context.Context, r controller.ReaderWriter, name string, keep []string) error {
	return ctrl.releaseHolds(ctx, r, func(diskStatus *hypervisor.VirtualMachineDiskStatus) bool {
		return diskStatus.TypedSpec().VirtualMachine != name || slices.Contains(keep, diskStatus.Metadata().ID())
	})
}

// releaseOrphanHolds gives back the holds left on the disks of a virtual machine which has no domain
// spec any more.
func (ctrl *VirtualMachineController) releaseOrphanHolds(ctx context.Context, r controller.ReaderWriter, known map[string]struct{}) error {
	return ctrl.releaseHolds(ctx, r, func(diskStatus *hypervisor.VirtualMachineDiskStatus) bool {
		_, stillSpecified := known[diskStatus.TypedSpec().VirtualMachine]

		return stillSpecified
	})
}

func (ctrl *VirtualMachineController) stopClaimed(
	ctx context.Context,
	r controller.ReaderWriter,
	logger *zap.Logger,
	client libvirtdomain.Client,
	machineUUID uuid.UUID,
	spec *hypervisor.VirtualMachineDomainSpec,
	exists bool,
	claimed bool,
) error {
	name := spec.Metadata().ID()

	switch {
	case claimed && exists:
		if err := client.Remove(libvirtdomain.Domain{Name: name, UUID: libvirtdomain.UUID(machineUUID, name)}); err != nil {
			return fmt.Errorf("failed to remove domain %q: %w", name, err)
		}
	case exists:
		// Not this controller's domain to remove, and it goes on reading whatever it was given.
		logger.Warn("keeping the disks of an unclaimed domain which is still there",
			zap.String("virtual_machine", name))

		return nil
	}

	// No domain of this virtual machine reads a disk any more. Released even when the spec was never
	// claimed: a start refused before the claim leaves holds behind no claim.
	if err := ctrl.releaseDisksExcept(ctx, r, name, nil); err != nil {
		return err
	}

	if err := ctrl.releaseSeedsExcept(ctx, r, name, ""); err != nil {
		return err
	}

	if !claimed {
		return nil
	}

	if err := r.RemoveFinalizer(ctx, spec.Metadata(), ctrl.Name()); err != nil {
		return fmt.Errorf("failed to release domain spec %q: %w", name, err)
	}

	return nil
}
