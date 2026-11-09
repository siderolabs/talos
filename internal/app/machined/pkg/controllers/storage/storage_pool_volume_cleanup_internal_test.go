// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	libvirtstorage "github.com/siderolabs/talos/internal/pkg/libvirt/storage"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	hypervisorcfg "github.com/siderolabs/talos/pkg/machinery/config/types/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
	storageres "github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

type privateCleanupPoolRace struct {
	controller.Writer
	state.StateReader
	current   *heldPoolRace
	poolReads int
}

func (r *privateCleanupPoolRace) Get(ctx context.Context, ptr resource.Pointer, opts ...state.GetOption) (resource.Resource, error) {
	if ptr.Type() == storageres.StoragePoolStatusType {
		r.poolReads++
	}

	if ptr.Type() == storageres.StoragePoolStatusType && r.current.held {
		return r.current.pool, nil
	}

	return r.StateReader.Get(ctx, ptr, opts...)
}

// countingCleanupWriter observes actual pool-hold writes through the controller path.
type countingCleanupWriter struct {
	controller.Writer
	acquisitions int
}

func (r *countingCleanupWriter) AddFinalizer(ctx context.Context, ptr resource.Pointer, finalizers ...resource.Finalizer) error {
	r.acquisitions++

	return r.Writer.AddFinalizer(ctx, ptr, finalizers...)
}

// leaveStaleStage gives the pool something to collect, so cleanup cannot skip it as idle.
func leaveStaleStage(t *testing.T, target string) {
	t.Helper()

	require.NoError(t, os.WriteFile(filepath.Join(target, ".interrupted.raw.0123456789abcdef.staged"), nil, 0o600))
}

func TestPrivateCopyCleanupSkipsIdlePool(t *testing.T) {
	ctx := t.Context()
	st := state.WrapCore(inmem.NewState())
	ctr, err := container.New()
	require.NoError(t, err)
	require.NoError(t, st.Create(ctx, config.NewMachineConfig(ctr)))

	pool := storageres.NewStoragePoolStatus(storageres.NamespaceName, "images")
	pool.TypedSpec().Phase = storageres.StoragePoolPhaseReady
	pool.TypedSpec().TargetPath = t.TempDir()
	pool.TypedSpec().VolumeID = "backing"
	require.NoError(t, st.Create(ctx, pool))
	current := &heldPoolRace{pool: pool, change: func(*storageres.StoragePoolStatus) {}}
	writer := &countingCleanupWriter{Writer: current}
	r := &privateCleanupPoolRace{Writer: writer, StateReader: st, current: current}
	ctrl := &StoragePoolVolumeController{}

	held := map[string]struct{}{}
	collected, err := ctrl.cleanupPrivateCopies(ctx, r, zap.NewNop(), map[string]*storageres.StoragePoolStatus{"images": pool}, held)
	require.NoError(t, err)
	require.False(t, collected)
	require.Empty(t, held)
	require.Zero(t, writer.acquisitions, "an idle pool must not be held for maintenance")
}

func TestPrivateCopyCleanupExcludedPoolDoesNotToggleHold(t *testing.T) {
	ctx := t.Context()
	st := state.WrapCore(inmem.NewState())
	ctr, err := container.New()
	require.NoError(t, err)
	require.NoError(t, st.Create(ctx, config.NewMachineConfig(ctr)))

	pool := storageres.NewStoragePoolStatus(storageres.NamespaceName, "images")
	pool.TypedSpec().Phase = storageres.StoragePoolPhaseReady
	pool.TypedSpec().TargetPath = t.TempDir()
	pool.TypedSpec().VolumeID = "backing"
	pool.Metadata().Finalizers().Add(storageres.StoragePoolVolumeMutationFinalizer("uncertain.raw"))
	leaveStaleStage(t, pool.TypedSpec().TargetPath)
	require.NoError(t, st.Create(ctx, pool))
	current := &heldPoolRace{pool: pool, change: func(*storageres.StoragePoolStatus) {}}
	writer := &countingCleanupWriter{Writer: current}
	r := &privateCleanupPoolRace{Writer: writer, StateReader: st, current: current}
	ctrl := &StoragePoolVolumeController{}

	for pass := range 2 {
		held := map[string]struct{}{}

		_, err = ctrl.cleanupPrivateCopies(ctx, r, zap.NewNop(), map[string]*storageres.StoragePoolStatus{"images": pool}, held)
		if pass == 0 {
			require.ErrorIs(t, err, errVolumePending)
		} else {
			require.NoError(t, err)
		}

		require.Empty(t, held)
	}

	require.Zero(t, writer.acquisitions, "an existing mutation fence must prevent maintenance hold writes")
}

func TestPrivateCopyMaintenanceDeadlineSurvivesProgressEvents(t *testing.T) {
	ctx := t.Context()
	st := state.WrapCore(inmem.NewState())
	ctr, err := container.New()
	require.NoError(t, err)
	require.NoError(t, st.Create(ctx, config.NewMachineConfig(ctr)))

	pool := storageres.NewStoragePoolStatus(storageres.NamespaceName, "images")
	pool.TypedSpec().Phase = storageres.StoragePoolPhaseReady
	pool.TypedSpec().TargetPath = t.TempDir()
	pool.TypedSpec().VolumeID = "backing"
	require.NoError(t, st.Create(ctx, pool))
	current := &heldPoolRace{pool: pool, change: func(*storageres.StoragePoolStatus) {}}
	r := &privateCleanupPoolRace{Writer: current, StateReader: st, current: current}
	now := time.Unix(100, 0)
	ctrl := &StoragePoolVolumeController{privateCopyNow: func() time.Time { return now }}

	for _, tick := range []time.Duration{0, time.Second, 2 * time.Second, 59 * time.Second} {
		now = time.Unix(100, 0).Add(tick)

		leaveStaleStage(t, pool.TypedSpec().TargetPath)
		_, err = ctrl.cleanupPrivateCopies(ctx, r, zap.NewNop(), map[string]*storageres.StoragePoolStatus{"images": pool}, map[string]struct{}{})
		require.NoError(t, err)
	}

	require.Equal(t, 1, r.poolReads, "progress reconciliations must not repeat a filesystem scan")
	require.Equal(t, time.Second, privateCopyRetryDelay(0, ctrl.nextPrivateCopyCleanup, now))
	now = now.Add(time.Second)

	leaveStaleStage(t, pool.TypedSpec().TargetPath)
	_, err = ctrl.cleanupPrivateCopies(ctx, r, zap.NewNop(), map[string]*storageres.StoragePoolStatus{"images": pool}, map[string]struct{}{})
	require.NoError(t, err)
	require.Equal(t, 2, r.poolReads, "deadline must allow the next maintenance pass")
}

func TestPrivateCopyPendingDoesNotScheduleOrdinaryRetry(t *testing.T) {
	now := time.Unix(100, 0)
	require.Equal(t, time.Minute, privateCopyRetryDelay(volumeRetryDelay(nil, now), now.Add(time.Minute), now))
	require.Equal(t, time.Second, privateCopyRetryDelay(volumeRetryDelay([]error{errors.New("volume failure")}, now), now.Add(time.Minute), now))
}

func TestPrivateCopyCleanupRechecksBackingAfterHold(t *testing.T) {
	ctx := t.Context()
	st := state.WrapCore(inmem.NewState())
	ctr, err := container.New()
	require.NoError(t, err)
	require.NoError(t, st.Create(ctx, config.NewMachineConfig(ctr)))
	target := t.TempDir()
	private := filepath.Join(target, ".copy-on-use")
	require.NoError(t, os.Mkdir(private, 0o700))
	marker := filepath.Join(private, "image-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	require.NoError(t, os.WriteFile(marker, []byte("image-owner:raw:"+uuid.NewString()+"\n"), 0o600))

	pool := storageres.NewStoragePoolStatus(storageres.NamespaceName, "images")
	pool.TypedSpec().Phase = storageres.StoragePoolPhaseReady
	pool.TypedSpec().VolumeID = "u-vms"
	pool.TypedSpec().TargetPath = target
	require.NoError(t, st.Create(ctx, pool))
	change := &heldPoolRace{pool: pool.DeepCopy().(*storageres.StoragePoolStatus), change: func(current *storageres.StoragePoolStatus) { current.TypedSpec().TargetPath = "/replacement" }}
	r := &privateCleanupPoolRace{Writer: change, StateReader: st, current: change}
	ctrl := &StoragePoolVolumeController{}
	held := map[string]struct{}{}
	maintenance, err := ctrl.cleanupPrivateCopies(ctx, r, zap.NewNop(), map[string]*storageres.StoragePoolStatus{"images": pool}, held)
	require.ErrorIs(t, err, errVolumePending)
	require.True(t, change.held, "the backing replacement must occur after acquiring the hold")
	require.False(t, maintenance)
	require.Empty(t, held)
	require.FileExists(t, marker, "a stale backing observation cannot authorize cleanup")
}

func TestPrivateMaintenanceExpiredDeadlineWithoutEligibleWork(t *testing.T) {
	for _, test := range []struct {
		name       string
		configured bool
	}{
		{
			name: "missing-config",
		},
		{
			name:       "no-ready-pool",
			configured: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := t.Context()
			st := state.WrapCore(inmem.NewState())

			if test.configured {
				ctr, err := container.New()
				require.NoError(t, err)
				require.NoError(t, st.Create(ctx, config.NewMachineConfig(ctr)))
			}

			now := time.Now()
			ctrl := &StoragePoolVolumeController{
				privateCopyNow:         func() time.Time { return now },
				nextPrivateCopyCleanup: now.Add(-time.Minute),
			}
			r := &privateCleanupPoolRace{StateReader: st}
			_, err := ctrl.cleanupPrivateCopies(ctx, r, zap.NewNop(), map[string]*storageres.StoragePoolStatus{}, map[string]struct{}{})
			require.NoError(t, err)
			require.Zero(t, privateCopyRetryDelay(0, ctrl.nextPrivateCopyCleanup, now), "no eligible work must wait for input events, not expired-timer churn")
		})
	}
}

func TestConfiguredCopyOwnershipRequiresRequestAndDestinationIdentity(t *testing.T) {
	for _, test := range []struct {
		name        string
		owner       resource.Owner
		destination string
		digest      string
		jobSource   string
		omitStatus  bool
		omitConfig  bool
		wantPending bool
		wantOwner   bool
	}{
		{name: "foreign producer", owner: "external.CopyController", destination: "vm__install__generation.raw"},
		{name: "authentic request waits for active config", owner: configuredDiskCopyProducer, destination: "vm__install__generation.raw", omitConfig: true, wantPending: true},
		{name: "wrong destination", owner: configuredDiskCopyProducer, destination: "unrelated.raw", wantPending: true},
		{name: "destination status lags request creation", owner: configuredDiskCopyProducer, destination: "vm__install__generation.raw", omitStatus: true, wantPending: true},
		{name: "changed request intent", owner: configuredDiskCopyProducer, destination: "vm__install__generation.raw", jobSource: "/superseded/installer.iso"},
		{name: "configured unpinned CD-ROM", owner: configuredDiskCopyProducer, destination: "vm__install__generation.raw", wantOwner: true},
		{name: "configured pinned CD-ROM", owner: configuredDiskCopyProducer, destination: "vm__install__generation.raw", digest: "sha256:" + strings.Repeat("a", 64), wantOwner: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := t.Context()
			st := state.WrapCore(inmem.NewState())
			vm := hypervisorcfg.NewVirtualMachineConfigV1Alpha1()
			vm.MetaName = "vm"
			vm.DisksConfig = hypervisorcfg.VirtualMachineDiskList{
				{
					DiskName: "install",
					DiskPool: "images",
					DiskType: hypervisorhelpers.VirtualMachineDiskTypeCDROM,
					ProvisionConfig: hypervisorcfg.VirtualMachineDiskProvision{
						FromImageConfig: &hypervisorcfg.VirtualMachineDiskFromImage{
							ImageLibrary: "retired-library",
							ImageFile:    "installer.iso",
							ImageDigest:  test.digest,
						},
					},
				},
			}
			ctr, err := container.New(vm)
			require.NoError(t, err)

			if !test.omitConfig {
				require.NoError(t, st.Create(ctx, config.NewMachineConfig(ctr)))
			}

			disk := configuredCopyDisk(vm.Disks()[0])
			key := disk.ImageBindingKey(vm.Name())
			request := storageres.NewStoragePoolVolumeSpec(storageres.NamespaceName, storageres.StoragePoolVolumeID("images", test.destination))
			*request.TypedSpec() = storageres.StoragePoolVolumeSpecSpec{
				Pool: "images", Name: test.destination, Format: disk.Format,
				Copy: &storageres.StoragePoolVolumeCopySpec{Source: "/unavailable/installer.iso"},
			}
			request.Metadata().Labels().Set(libvirtstorage.CopyBindingLabel, key)
			require.NoError(t, st.Create(ctx, request, state.WithCreateOwner(test.owner)))

			status := hypervisor.NewVirtualMachineDiskStatus(hypervisor.NamespaceName, hypervisor.VirtualMachineDiskStatusID(vm.Name(), disk))
			status.TypedSpec().VirtualMachine = vm.Name()
			status.TypedSpec().Name = disk.Name
			status.TypedSpec().Pool = disk.Pool
			status.TypedSpec().Volume = "vm__install__generation.raw"
			status.Metadata().Labels().Set(libvirtstorage.CopyBindingLabel, key)

			if !test.omitStatus {
				require.NoError(t, st.Create(ctx, status, state.WithCreateOwner(configuredDiskCopyProducer)))
			}

			job := &copyJob{id: request.Metadata().ID(), spec: *request.TypedSpec(), bindingKey: key, reader: st, ctx: ctx}
			if test.jobSource != "" {
				job.spec.Copy = &storageres.StoragePoolVolumeCopySpec{Source: test.jobSource}
			}

			if !test.wantOwner {
				// A nil operation deliberately makes an unauthorized SetBindingKey observable.
				err := bindConfiguredCopyOwner(job, nil)
				if test.wantPending {
					require.ErrorContains(t, err, "configured copy destination is not current")
				} else {
					require.NoError(t, err)
				}

				return
			}

			owned, err := configuredCopyOwner(job)
			require.NoError(t, err)
			require.True(t, owned, "configured source-free CD-ROM copy must retain recovery authority")
		})
	}
}

// canonicalHoldUpdate delegates to real COSI state; only the hold boundary is intercepted.
type canonicalHoldUpdate struct {
	controller.Writer
	state.StateReader
	st      state.State
	next    *config.MachineConfig
	updated bool
}

func (r *canonicalHoldUpdate) AddFinalizer(ctx context.Context, ptr resource.Pointer, finalizers ...resource.Finalizer) error {
	if err := r.st.AddFinalizer(ctx, ptr, finalizers...); err != nil {
		return err
	}

	current, err := r.st.Get(ctx, r.next.Metadata())
	if err != nil {
		return err
	}

	r.next.Metadata().SetVersion(current.Metadata().Version())

	if err = r.st.Update(ctx, r.next); err != nil {
		return err
	}

	r.updated = true

	return nil
}

func TestPrivateCopyCanonicalRevisionAfterRetentionSnapshotPreservesRecovery(t *testing.T) {
	ctx := t.Context()
	st := state.WrapCore(inmem.NewState())
	empty, err := container.New()
	require.NoError(t, err)
	require.NoError(t, st.Create(ctx, config.NewMachineConfig(empty)))

	vm := hypervisorcfg.NewVirtualMachineConfigV1Alpha1()
	vm.MetaName = "guest"
	vm.DisksConfig = hypervisorcfg.VirtualMachineDiskList{
		{
			DiskName: "disk",
			DiskPool: "images",
			DiskType: hypervisorhelpers.VirtualMachineDiskTypeCDROM,
			ProvisionConfig: hypervisorcfg.VirtualMachineDiskProvision{
				FromImageConfig: &hypervisorcfg.VirtualMachineDiskFromImage{
					ImageLibrary: "missing-source",
					ImageFile:    "missing.img",
				},
			},
		},
	}
	configured, err := container.New(vm)
	require.NoError(t, err)

	key := configuredCopyDisk(vm.Disks()[0]).ImageBindingKey(vm.Name())
	require.NotEmpty(t, key)
	target := t.TempDir()
	private := filepath.Join(target, ".copy-on-use")
	require.NoError(t, os.Mkdir(private, 0o700))

	name := "retired.img"
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(name)))
	witness := filepath.Join(private, digest)
	owner := filepath.Join(private, "owner-"+digest)
	binding := filepath.Join(private, "binding-"+key)

	require.NoError(t, os.WriteFile(witness, []byte("immutable recovery payload"), 0o600))
	info, err := os.Lstat(witness)
	require.NoError(t, err)

	identity := info.Sys().(*syscall.Stat_t)
	require.NoError(t, os.WriteFile(owner, []byte(fmt.Sprintf("%s\n%s\n%d\n", key, name, identity.Ino)), 0o600))
	require.NoError(t, os.WriteFile(binding, []byte(name), 0o600))

	pool := storageres.NewStoragePoolStatus(storageres.NamespaceName, "images")
	pool.TypedSpec().Phase = storageres.StoragePoolPhaseReady
	pool.TypedSpec().TargetPath = target
	pool.TypedSpec().VolumeID = "backing"
	require.NoError(t, st.Create(ctx, pool))
	rw := &canonicalHoldUpdate{
		StateReader: st,
		st:          st,
		next:        config.NewMachineConfig(configured),
	}
	ctrl := &StoragePoolVolumeController{}

	maintenance, err := ctrl.cleanupPrivateCopies(ctx, rw, zap.NewNop(), map[string]*storageres.StoragePoolStatus{"images": pool}, map[string]struct{}{})
	if err != nil {
		require.ErrorIs(t, err, errVolumePending)
	}

	require.True(t, rw.updated, "canonical update must occur at the actual hold boundary")

	current, err := st.Get(ctx, config.NewMachineConfig(empty).Metadata())
	require.NoError(t, err)
	require.Equal(t, key, configuredCopyDisk(current.(*config.MachineConfig).Config().VirtualMachineConfigs()[0].Disks()[0]).ImageBindingKey("guest"))

	for _, path := range []string{witness, owner, binding} {
		require.FileExists(t, path, "newly canonical recovery metadata must survive: %s", path)
	}

	require.False(t, maintenance, "stale retention must defer collection")
}
