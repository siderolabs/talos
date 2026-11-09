// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package disks_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/owned"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	diskhelpers "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/hypervisor/internal/disks"
	libvirtstorage "github.com/siderolabs/talos/internal/pkg/libvirt/storage"
	"github.com/siderolabs/talos/internal/pkg/libvirt/storage/copyowner"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

// Two names joined by one hyphen must not make two machines share a volume.
func TestBlankVolumeNamesAreInjective(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	st := state.WrapCore(inmem.NewState())
	writer := owned.New(st, diskhelpers.ControllerName)
	pool := storage.NewStoragePoolStatus(storage.NamespaceName, "pool1")
	pool.TypedSpec().Phase = storage.StoragePoolPhaseReady
	pool.TypedSpec().TargetPath = t.TempDir()
	require.NoError(t, st.Create(ctx, pool))

	for _, test := range []struct{ vm, disk string }{
		{vm: "vm-a", disk: "b"},
		{vm: "vm", disk: "a-b"},
	} {
		vm := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, test.vm)
		vm.TypedSpec().Disks = []hypervisor.VirtualMachineDiskSpec{
			{
				Name:      test.disk,
				Pool:      "pool1",
				Size:      1 << 30,
				Format:    "qcow2",
				Bus:       "virtio",
				Type:      "disk",
				Provision: hypervisor.VirtualMachineDiskProvisionSpec{Blank: true},
			},
		}
		require.NoError(t, st.Create(ctx, vm))
	}

	reconciler := diskhelpers.NewReconciler(&diskhelpers.ImageVerifier{})
	require.NoError(t, reconciler.Reconcile(ctx, writer, zap.NewNop()))
	first, err := safe.ReaderGetByID[*storage.StoragePoolVolumeSpec](ctx, writer, "pool1/vm-a__b.qcow2")
	require.NoError(t, err)
	second, err := safe.ReaderGetByID[*storage.StoragePoolVolumeSpec](ctx, writer, "pool1/vm__a-b.qcow2")
	require.NoError(t, err)
	assert.NotEqual(t, first.TypedSpec().Name, second.TypedSpec().Name)
	assert.Equal(t, "vm-a__b.qcow2", first.TypedSpec().Name)
	assert.Equal(t, "vm__a-b.qcow2", second.TypedSpec().Name)
}

type countedImageVerifier struct{ calls int }

func (verifier *countedImageVerifier) Observe(_ hypervisor.VirtualMachineDiskSpec, _ hypervisor.ContentLibraryStatusSpec) (hypervisor.VirtualMachineDiskStatusSpec, error) {
	verifier.calls++

	return hypervisor.VirtualMachineDiskStatusSpec{SourcePath: "/library/object"}, nil
}

type conflictingDiskWriter struct {
	controller.ReaderWriter
	conflicts int
}

func (writer *conflictingDiskWriter) Modify(ctx context.Context, empty resource.Resource, modify func(resource.Resource) error, options ...owned.ModifyOption) error {
	return writer.ReaderWriter.Modify(ctx, empty, func(current resource.Resource) error {
		if err := modify(current); err != nil {
			return err
		}

		if empty.Metadata().Type() == storage.StoragePoolVolumeSpecType && writer.conflicts == 0 {
			writer.conflicts++

			return writer.ReaderWriter.AddFinalizer(ctx, current.Metadata(), "test.concurrent")
		}

		return nil
	}, options...)
}

func TestWritableFailedCopyObservesSourceOncePerPass(t *testing.T) {
	for _, conflict := range []bool{false, true} {
		t.Run(map[bool]string{false: "normal", true: "conflict"}[conflict], func(t *testing.T) {
			testWritableFailedCopyObservation(t, conflict)
		})
	}
}

func testWritableFailedCopyObservation(t *testing.T, conflict bool) {
	ctx := t.Context()
	st := state.WrapCore(inmem.NewState())
	writer := owned.New(st, diskhelpers.ControllerName)
	pool := storage.NewStoragePoolStatus(storage.NamespaceName, "pool")
	pool.TypedSpec().Phase = storage.StoragePoolPhaseReady
	pool.TypedSpec().TargetPath = t.TempDir()
	require.NoError(t, st.Create(ctx, pool))

	disk := hypervisor.VirtualMachineDiskSpec{
		Name:   "system",
		Pool:   "pool",
		Format: "raw",
		Type:   "disk",
		Bus:    "virtio",
		Size:   1024,
		Provision: hypervisor.VirtualMachineDiskProvisionSpec{
			FromImage: &hypervisor.VirtualMachineDiskFromImageSpec{
				Library: "library",
				File:    "image.raw",
			},
		},
	}
	request := storage.NewStoragePoolVolumeSpec(storage.NamespaceName, "pool/vm__system.img")
	*request.TypedSpec() = storage.StoragePoolVolumeSpecSpec{
		Pool:     "pool",
		Name:     "vm__system.img",
		Format:   "raw",
		Capacity: disk.Size,
		Copy: &storage.StoragePoolVolumeCopySpec{
			Image:         true,
			Source:        "/library/object",
			SourceLibrary: "library",
		},
	}
	require.NoError(t, writer.Create(ctx, request))

	request, err := safe.ReaderGetByID[*storage.StoragePoolVolumeSpec](ctx, writer, request.Metadata().ID())
	require.NoError(t, err)

	pool, err = safe.ReaderGetByID[*storage.StoragePoolStatus](ctx, writer, "pool")
	require.NoError(t, err)

	status := storage.NewStoragePoolVolumeStatus(storage.NamespaceName, request.Metadata().ID())
	*status.TypedSpec() = storage.StoragePoolVolumeStatusSpec{
		Pool:   "pool",
		Name:   "vm__system.img",
		Format: "raw",
		Phase:  storage.StoragePoolVolumePhaseNotReady,
		Copy: &storage.StoragePoolVolumeCopyStatusSpec{
			Phase:  storage.StoragePoolVolumeCopyPhaseFailed,
			Intent: storage.StoragePoolVolumeCopyInitializationIntent(request, pool),
		},
	}
	require.NoError(t, st.Create(ctx, status))

	library := hypervisor.NewContentLibraryStatus(hypervisor.NamespaceName, "library")
	require.NoError(t, st.Create(ctx, library))

	vm := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "vm")
	vm.TypedSpec().Disks = []hypervisor.VirtualMachineDiskSpec{disk}
	require.NoError(t, st.Create(ctx, vm))

	verifier := &countedImageVerifier{}
	ctrl := diskhelpers.NewReconciler(verifier)

	var r controller.ReaderWriter = writer

	if conflict {
		r = &conflictingDiskWriter{ReaderWriter: writer}
	}

	err = ctrl.Reconcile(t.Context(), r, zap.NewNop())
	require.NoError(t, err)
	currentDisk, err := safe.ReaderGetByID[*hypervisor.VirtualMachineDiskStatus](ctx, r, hypervisor.VirtualMachineDiskStatusID("vm", disk))
	require.NoError(t, err)
	require.Equal(t, hypervisor.VirtualMachineDiskPhaseNotReady, currentDisk.TypedSpec().Phase)
	require.Contains(t, currentDisk.TypedSpec().Error, "not ready")
	require.Equal(t, 1, verifier.calls, "a failed-copy pass must not re-observe inside the write or afterward")

	if conflict {
		require.Equal(t, 1, r.(*conflictingDiskWriter).conflicts)
	}
}

func malformedCopyBindingFixture(t *testing.T) (controller.ReaderWriter, *storage.StoragePoolVolumeSpec, *hypervisor.ContentLibraryStatus) {
	t.Helper()

	ctx := t.Context()
	st := state.WrapCore(inmem.NewState())
	writer := owned.New(st, diskhelpers.ControllerName)
	target := t.TempDir()
	key := strings.Repeat("a", 64)
	recordDir := filepath.Join(target, ".copy-on-use")
	require.NoError(t, os.Mkdir(recordDir, 0o700))
	require.NoError(t, os.Mkdir(filepath.Join(recordDir, "binding-"+key), 0o700))

	pool := storage.NewStoragePoolStatus(storage.NamespaceName, "pool")
	pool.TypedSpec().TargetPath = target
	pool.TypedSpec().Phase = storage.StoragePoolPhaseReady
	require.NoError(t, st.Create(ctx, pool))

	request := storage.NewStoragePoolVolumeSpec(storage.NamespaceName, "pool/image.raw")
	request.TypedSpec().Pool = "pool"
	request.TypedSpec().Name = "image.raw"
	request.TypedSpec().Format = "raw"
	request.TypedSpec().Copy = &storage.StoragePoolVolumeCopySpec{
		Image:         true,
		Source:        "/library/object",
		SourceLibrary: "library",
	}
	request.Metadata().Labels().Set(libvirtstorage.CopyBindingLabel, key)
	require.NoError(t, writer.Create(ctx, request))
	require.NoError(t, st.AddFinalizer(ctx, request.Metadata(), "test.worker"))

	library := hypervisor.NewContentLibraryStatus(hypervisor.NamespaceName, "library")
	require.NoError(t, st.Create(ctx, library))

	_, err := copyowner.ReadBinding(target, key)
	require.ErrorContains(t, err, "invalid copy binding record")

	return writer, request, library
}

func TestMalformedBindingRetainsSourceThroughReconciliationTeardown(t *testing.T) {
	r, request, library := malformedCopyBindingFixture(t)
	ctrl := diskhelpers.NewReconciler(&diskhelpers.ImageVerifier{})

	err := ctrl.Reconcile(t.Context(), r, zap.NewNop())
	require.ErrorContains(t, err, "invalid copy binding record")

	current, err := safe.ReaderGetByID[*hypervisor.ContentLibraryStatus](t.Context(), r, library.Metadata().ID())
	require.NoError(t, err)
	require.True(t, current.Metadata().Finalizers().Has(diskhelpers.ControllerName), "unknown copy completion must acquire the source hold")

	currentRequest, err := safe.ReaderGetByID[*storage.StoragePoolVolumeSpec](t.Context(), r, request.Metadata().ID())
	require.NoError(t, err)
	require.Equal(t, resource.PhaseTearingDown, currentRequest.Metadata().Phase(), "the reconciliation must have run teardown")
}
