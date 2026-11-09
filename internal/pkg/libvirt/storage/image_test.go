// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	libvirt "github.com/digitalocean/go-libvirt"
	"github.com/google/uuid"
	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
	"libvirt.org/go/libvirtxml"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	storagectrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/storage"
	libvirtstorage "github.com/siderolabs/talos/internal/pkg/libvirt/storage"
	"github.com/siderolabs/talos/internal/pkg/libvirt/storage/copyowner"
	"github.com/siderolabs/talos/internal/pkg/qemuimg"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	storageres "github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

func TestWritableRawNestedQcow2SurvivesImportAndAdoption(t *testing.T) {
	binary, err := exec.LookPath("qemu-img")
	if err != nil {
		t.Skip("qemu-img required")
	}

	rpc, client, pool := newVolumeFixture(t)
	nested := filepath.Join(t.TempDir(), "nested.qcow2")
	output, err := exec.CommandContext(t.Context(), binary, "create", "-f", "qcow2", nested, "1M").CombinedOutput()
	require.NoError(t, err, "%s", output)
	source := filepath.Join(t.TempDir(), "outer.qcow2")
	output, err = exec.CommandContext(t.Context(), binary, "convert", "-f", "raw", "-O", "qcow2", nested, source).CombinedOutput()
	require.NoError(t, err, "%s", output)

	op, err := client.PrepareCopy(pool, "guest.raw", source, "")
	require.NoError(t, err)

	defer op.Close()

	require.NoError(t, op.Import(t.Context(), qemuimg.New(binary), "raw", 2<<20))
	volume, err := client.FinishCopy(t.Context(), pool, op, func() error { return nil })
	require.NoError(t, err)
	require.Equal(t, "raw", volume.Format)
	require.EqualValues(t, 2<<20, volume.Capacity)

	data, err := os.ReadFile(filepath.Join(rpc.target, "guest.raw"))
	require.NoError(t, err)
	nestedBytes, err := os.ReadFile(nested)
	require.NoError(t, err)
	require.Equal(t, nestedBytes, data[:len(nestedBytes)])

	cache := &libvirtstorage.CopyCache{Image: true}
	for range 2 {
		volume, adopted, adoptErr := client.AdoptCopy(pool, "guest.raw", cache)
		require.NoError(t, adoptErr)
		require.True(t, adopted)
		require.Equal(t, "raw", volume.Format)
		require.EqualValues(t, 2<<20, volume.Capacity)
	}
}

func TestWritableRawNestedQcow2GrowsAfterLibvirtRefresh(t *testing.T) {
	connector, pool := liveCopyPool(t)
	binary, err := exec.LookPath("qemu-img")
	require.NoError(t, err)

	source := filepath.Join(t.TempDir(), "source.raw")
	sourceBytes := make([]byte, 1<<20)
	require.NoError(t, os.WriteFile(source, sourceBytes, 0o600))
	client, err := connector.OpenVolumes(t.Context())
	require.NoError(t, err)

	defer client.Close()

	op, err := client.PrepareCopy(pool, "guest.raw", source, digest.SHA256.FromBytes(sourceBytes).String())
	require.NoError(t, err)

	defer op.Close()

	require.NoError(t, op.Import(t.Context(), qemuimg.New(binary), "raw", 3<<20))
	volume, err := client.FinishCopy(t.Context(), pool, op, func() error { return nil })
	require.NoError(t, err)

	nested := filepath.Join(t.TempDir(), "nested.qcow2")
	output, err := exec.CommandContext(t.Context(), binary, "create", "-f", "qcow2", nested, "1M").CombinedOutput()
	require.NoError(t, err, "%s", output)
	guestBytes, err := os.ReadFile(nested)
	require.NoError(t, err)
	file, err := os.OpenFile(volume.Path, os.O_WRONLY, 0)
	require.NoError(t, err)
	_, err = file.WriteAt(guestBytes, 0)
	require.NoError(t, err)
	require.NoError(t, file.Sync())
	require.NoError(t, file.Close())

	prefix, err := os.ReadFile(volume.Path)
	require.NoError(t, err)
	require.Equal(t, 3<<20, len(prefix))

	uri := "qemu+unix:///session?socket=" + os.Getenv("TALOS_TEST_STORAGE_SOCKET")
	output, err = exec.CommandContext(t.Context(), "virsh", "--connect", uri, "pool-refresh", pool.Name).CombinedOutput()
	require.NoError(t, err, "%s", output)
	observed, exists, err := client.Volume(pool, "guest.raw")
	require.NoError(t, err)
	require.True(t, exists)
	require.Equal(t, "qcow2", observed.Format, "real libvirt must have reclassified guest sectors")
	require.EqualValues(t, 1<<20, observed.Capacity)

	outcome, err := client.ResizeVolumeOperation(pool, "guest.raw", 6<<20)
	require.NoError(t, err)
	require.Equal(t, libvirtstorage.Finished, outcome)

	info, err := os.Stat(volume.Path)
	require.NoError(t, err)
	require.EqualValues(t, 6<<20, info.Size(), "grow the owned raw file, not its guest qcow2 header")

	contents, err := os.ReadFile(volume.Path)
	require.NoError(t, err)
	require.Equal(t, prefix, contents[:len(prefix)], "growth must preserve every existing guest byte")

	outcome, err = client.ResizeVolumeOperation(pool, "guest.raw", 2<<20)
	require.Error(t, err, "owned raw images must never shrink")
	require.Equal(t, libvirtstorage.NoMutationSubmitted, outcome)

	info, err = os.Stat(volume.Path)
	require.NoError(t, err)
	require.EqualValues(t, 6<<20, info.Size())
}

func TestWritableQcow2AdoptionRejectsChangedFormat(t *testing.T) {
	binary, err := exec.LookPath("qemu-img")
	if err != nil {
		t.Skip("qemu-img required")
	}

	rpc, client, pool := newVolumeFixture(t)
	source := filepath.Join(t.TempDir(), "source.raw")
	require.NoError(t, os.WriteFile(source, make([]byte, 1<<20), 0o600))
	op, err := client.PrepareCopy(pool, "guest.qcow2", source, "")
	require.NoError(t, err)

	defer op.Close()

	require.NoError(t, op.Import(t.Context(), qemuimg.New(binary), "qcow2", 2<<20))
	_, err = client.FinishCopy(t.Context(), pool, op, func() error { return nil })
	require.NoError(t, err)
	file, err := os.OpenFile(filepath.Join(rpc.target, "guest.qcow2"), os.O_WRONLY, 0)
	require.NoError(t, err)
	_, err = file.WriteAt([]byte("RAW!"), 0)
	require.NoError(t, err)
	require.NoError(t, file.Close())

	_, _, err = client.AdoptCopy(pool, "guest.qcow2", &libvirtstorage.CopyCache{Image: true})
	require.Error(t, err)
}

func TestWritableRawNestedQcow2Live(t *testing.T) {
	binary, err := exec.LookPath("qemu-img")
	if err != nil {
		t.Skip("qemu-img required")
	}

	connector, pool := liveCopyPool(t)
	nested := filepath.Join(t.TempDir(), "nested.qcow2")
	output, err := exec.CommandContext(t.Context(), binary, "create", "-f", "qcow2", nested, "1M").CombinedOutput()
	require.NoError(t, err, "%s", output)
	source := filepath.Join(t.TempDir(), "outer.qcow2")
	output, err = exec.CommandContext(t.Context(), binary, "convert", "-f", "raw", "-O", "qcow2", nested, source).CombinedOutput()
	require.NoError(t, err, "%s", output)

	client, err := connector.OpenVolumes(t.Context())
	require.NoError(t, err)
	op, err := client.PrepareCopy(pool, "guest.raw", source, "")
	require.NoError(t, err)

	client.Close()
	defer op.Close()

	require.NoError(t, op.Import(t.Context(), qemuimg.New(binary), "raw", 2<<20))

	client, err = connector.OpenVolumes(t.Context())
	require.NoError(t, err)
	volume, err := client.FinishCopy(t.Context(), pool, op, func() error { return nil })
	require.NoError(t, err)
	require.Equal(t, "raw", volume.Format)
	require.EqualValues(t, 2<<20, volume.Capacity)
	client.Close()

	client, err = connector.OpenVolumes(t.Context())
	require.NoError(t, err)

	defer client.Close() //nolint:errcheck

	cache := &libvirtstorage.CopyCache{Image: true}
	for range 2 {
		volume, adopted, adoptErr := client.AdoptCopy(pool, "guest.raw", cache)
		require.NoError(t, adoptErr)
		require.True(t, adopted)
		require.Equal(t, "raw", volume.Format)
		require.EqualValues(t, 2<<20, volume.Capacity)
	}
}

func TestImageImportIndependent(t *testing.T) {
	binary, err := exec.LookPath("qemu-img")
	if err != nil {
		t.Skip("qemu-img required")
	}

	for _, pair := range [][2]string{{"raw", "raw"}, {"raw", "qcow2"}, {"qcow2", "raw"}, {"qcow2", "qcow2"}} {
		t.Run(pair[0]+"-to-"+pair[1], func(t *testing.T) {
			format := pair[1]
			rpc, client, pool := newVolumeFixture(t)
			source := filepath.Join(t.TempDir(), "source")
			contents := make([]byte, 1<<20)
			copy(contents[512:], "guest sentinel")
			require.NoError(t, os.WriteFile(source, contents, 0o600))

			if pair[0] == "qcow2" {
				output, err := exec.CommandContext(t.Context(), binary, "convert", "-f", "raw", "-O", "qcow2", source, source+".qcow2").CombinedOutput()
				require.NoError(t, err, "%s", output)

				source += ".qcow2"
			}

			sourceBytes, err := os.ReadFile(source)
			require.NoError(t, err)
			op, err := client.PrepareCopy(pool, "guest."+format, source, digest.SHA256.FromBytes(sourceBytes).String())
			require.NoError(t, err)

			defer op.Close()

			require.NoError(t, op.Import(t.Context(), qemuimg.New(binary), format, 2<<20))
			_, err = client.FinishCopy(t.Context(), pool, op, func() error { return nil })
			require.NoError(t, err)
			require.NoError(t, os.Remove(source))

			_, adopted, err := client.AdoptCopy(pool, "guest."+format, &libvirtstorage.CopyCache{Image: true})
			require.NoError(t, err)
			require.True(t, adopted)

			file, err := os.Open(filepath.Join(rpc.target, "guest."+format))
			require.NoError(t, err)

			defer file.Close() //nolint:errcheck

			info, err := qemuimg.New(binary).Info(t.Context(), file)
			require.NoError(t, err)
			require.Equal(t, format, info.Format)
			require.EqualValues(t, 2<<20, info.VirtualSize)
			outputPath := filepath.Join(t.TempDir(), "restored.raw")
			output, err := exec.CommandContext(t.Context(), binary, "convert", "-f", format, "-O", "raw", file.Name(), outputPath).CombinedOutput()
			require.NoError(t, err, "%s", output)
			actual, err := os.ReadFile(outputPath)
			require.NoError(t, err)
			require.Equal(t, contents, actual[:len(contents)])
			require.Equal(t, make([]byte, 1<<20), actual[len(contents):])
		})
	}
}

type imageSession struct {
	libvirtstorage.VolumeClient
	unlock func()
}

func (session *imageSession) Close() { session.unlock() }

func TestWritableImageControllerPreservesGuestWritesOnReAdd(t *testing.T) {
	binary, err := exec.LookPath("qemu-img")
	if err != nil {
		t.Skip("qemu-img required")
	}

	rpc, _, pool := newVolumeFixture(t)
	s := &ctest.DefaultSuite{}
	s.SetT(t)
	s.SetupTest()
	t.Cleanup(s.TearDownTest)

	system := hardware.NewSystemInformation(hardware.SystemInformationID)
	system.TypedSpec().UUID = machine
	s.Create(system)

	poolStatus := storageres.NewStoragePoolStatus(storageres.NamespaceName, pool.Name)
	*poolStatus.TypedSpec() = storageres.StoragePoolStatusSpec{
		Phase:      storageres.StoragePoolPhaseReady,
		VolumeID:   "backing",
		TargetPath: rpc.target,
	}
	s.Create(poolStatus, state.WithCreateOwner("storage.StoragePoolController"))

	data := make([]byte, 1<<20)
	source := filepath.Join(t.TempDir(), "source.raw")
	require.NoError(t, os.WriteFile(source, data, 0o600))

	newRequest := func() *storageres.StoragePoolVolumeSpec {
		request := storageres.NewStoragePoolVolumeSpec(storageres.NamespaceName, storageres.StoragePoolVolumeID(pool.Name, "guest.img"))
		*request.TypedSpec() = storageres.StoragePoolVolumeSpecSpec{
			Pool:     pool.Name,
			Name:     "guest.img",
			Format:   "raw",
			Capacity: 2 << 20,
			Copy:     &storageres.StoragePoolVolumeCopySpec{Image: true},
		}

		return request
	}
	request := newRequest()
	request.TypedSpec().Copy.Source = source
	request.TypedSpec().Copy.Digest = digest.SHA256.FromBytes(data).String()
	s.Create(request, state.WithCreateOwner("hypervisor.VirtualMachineDiskController"))

	var sessions sync.Mutex

	require.NoError(t, s.Runtime().RegisterController(&storagectrl.StoragePoolVolumeController{
		ImageTool: qemuimg.New(binary),
		Open: func(context.Context) (libvirtstorage.VolumeClient, error) {
			sessions.Lock()

			return &imageSession{
				VolumeClient: libvirtstorage.NewTestClient(rpc),
				unlock:       sessions.Unlock,
			}, nil
		},
	}))

	waitReady := func() {
		t.Helper()
		require.Eventually(t, func() bool {
			status, err := safe.StateGetByID[*storageres.StoragePoolVolumeStatus](s.Ctx(), s.State(), request.Metadata().ID())

			return err == nil && status.TypedSpec().Phase == storageres.StoragePoolVolumePhaseReady &&
				status.TypedSpec().Copy != nil && status.TypedSpec().Copy.Phase == storageres.StoragePoolVolumeCopyPhaseSucceeded
		}, 5*time.Second, time.Millisecond)
	}
	waitReady()
	require.NoError(t, os.Remove(source))

	guest, err := os.OpenFile(filepath.Join(rpc.target, "guest.img"), os.O_WRONLY, 0)
	require.NoError(t, err)
	_, err = guest.WriteAt([]byte("guest-written sentinel"), 512)
	require.NoError(t, err)
	require.NoError(t, guest.Sync())
	require.NoError(t, guest.Close())

	_, err = s.State().Teardown(s.Ctx(), request.Metadata(), state.WithTeardownOwner("hypervisor.VirtualMachineDiskController"))
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		current, err := safe.StateGetByID[*storageres.StoragePoolVolumeSpec](s.Ctx(), s.State(), request.Metadata().ID())

		return err == nil && current.Metadata().Finalizers().Empty()
	}, 5*time.Second, time.Millisecond)
	s.Destroy(request, state.WithDestroyOwner("hypervisor.VirtualMachineDiskController"))
	require.Eventually(t, func() bool {
		_, err := safe.StateGetByID[*storageres.StoragePoolVolumeStatus](s.Ctx(), s.State(), request.Metadata().ID())

		return state.IsNotFoundError(err)
	}, 5*time.Second, time.Millisecond)

	request = newRequest()
	s.Create(request, state.WithCreateOwner("hypervisor.VirtualMachineDiskController"))
	waitReady()

	contents, err := os.ReadFile(filepath.Join(rpc.target, "guest.img"))
	require.NoError(t, err)
	require.Equal(t, "guest-written sentinel", string(contents[512:512+len("guest-written sentinel")]))
}

func TestFailedWritablePublicationMetadataAloneAllowsReplacement(t *testing.T) {
	binary, err := exec.LookPath("qemu-img")
	if err != nil {
		t.Skip("qemu-img required")
	}

	rpc, client, pool := newVolumeFixture(t)
	name := "guest.img"
	source := filepath.Join(t.TempDir(), "source.raw")
	require.NoError(t, os.WriteFile(source, make([]byte, 1<<20), 0o600))
	op, err := client.PrepareCopy(pool, name, source, "")
	require.NoError(t, err)

	defer op.Close()

	require.NoError(t, op.Import(t.Context(), qemuimg.New(binary), "raw", 2<<20))

	// Model a failure immediately after the ownership marker was synced, but
	// before the public payload link. FinishCopy closes and removes its stage.
	failure := fmt.Errorf("publication interrupted")
	_, err = client.FinishCopy(t.Context(), pool, op, func() error {
		entries, readErr := os.ReadDir(rpc.target)
		require.NoError(t, readErr)

		var stage string

		for _, entry := range entries {
			if strings.HasSuffix(entry.Name(), ".staged") {
				stage = entry.Name()
			}
		}

		require.NotEmpty(t, stage)

		marker := filepath.Join(rpc.target, fmt.Sprintf(".copy-on-use/image-%x", sha256.Sum256([]byte(name))))
		owner := imageOwnerFixtureToken(t, filepath.Join(rpc.target, stage))
		require.NoError(t, os.WriteFile(marker, []byte("image-owner:raw:"+owner+"\n"), 0o600))

		return failure
	})
	require.ErrorIs(t, err, failure)
	absent, err := copyowner.PublicationAbsent(rpc.target, name)
	require.NoError(t, err)
	require.True(t, absent, "metadata alone cannot resurrect a removed stage")
	require.NoFileExists(t, filepath.Join(rpc.target, name))

	// The original source can disappear; the surviving metadata must not
	// prevent a fresh import from a replacement source.
	require.NoError(t, os.Remove(source))
	replacement := filepath.Join(t.TempDir(), "replacement.raw")
	require.NoError(t, os.WriteFile(replacement, make([]byte, 1<<20), 0o600))
	fresh, err := client.PrepareCopy(pool, name, replacement, "")
	require.NoError(t, err)

	defer fresh.Close()

	require.NoError(t, fresh.Import(t.Context(), qemuimg.New(binary), "raw", 2<<20))
	_, err = client.FinishCopy(t.Context(), pool, fresh, func() error { return nil })
	require.NoError(t, err)
	require.FileExists(t, filepath.Join(rpc.target, name))
}

func TestImageImportReportsHashAndConversionProgress(t *testing.T) {
	binary, err := exec.LookPath("qemu-img")
	if err != nil {
		t.Skip("qemu-img required")
	}

	_, client, pool := newVolumeFixture(t)
	source := filepath.Join(t.TempDir(), "source.raw")
	require.NoError(t, os.WriteFile(source, make([]byte, 1<<20), 0o600))
	op, err := client.PrepareCopy(pool, "guest.img", source, "")
	require.NoError(t, err)

	defer op.Close()

	var seen []libvirtstorage.CopyProgress

	op.SetProgressObserver(func(progress libvirtstorage.CopyProgress) { seen = append(seen, progress) })
	require.NoError(t, op.Import(t.Context(), qemuimg.New(binary), "qcow2", 2<<20))

	var verified, converted bool

	for _, item := range seen {
		if item.Phase == libvirtstorage.CopyPhaseVerify && item.BytesCopied > 0 {
			verified = true
		}

		if item.Phase == libvirtstorage.CopyPhaseConvert && item.BytesCopied > 0 {
			converted = true
		}
	}

	require.True(t, verified, "hash pass must report advancing progress")
	require.True(t, converted, "convert must report advancing progress")
}

func TestWritableImageDeletionDoesNotResurrectGuestData(t *testing.T) {
	binary, err := exec.LookPath("qemu-img")
	if err != nil {
		t.Skip("qemu-img required")
	}

	rpc, client, pool := newVolumeFixture(t)
	source := filepath.Join(t.TempDir(), "source.raw")
	require.NoError(t, os.WriteFile(source, make([]byte, 1<<20), 0o600))
	op, err := client.PrepareCopy(pool, "guest.img", source, "")
	require.NoError(t, err)

	defer op.Close()

	require.NoError(t, op.Import(t.Context(), qemuimg.New(binary), "raw", 2<<20))
	_, err = client.FinishCopy(t.Context(), pool, op, func() error { return nil })
	require.NoError(t, err)

	name := filepath.Join(rpc.target, "guest.img")
	witness := filepath.Join(rpc.target, fmt.Sprintf(".copy-on-use/image-%x", sha256.Sum256([]byte("guest.img"))))
	guest, err := os.OpenFile(name, os.O_WRONLY, 0)
	require.NoError(t, err)
	_, err = guest.WriteAt([]byte("secret guest data"), 512)
	require.NoError(t, err)
	require.NoError(t, guest.Close())

	published, err := os.Stat(name)
	require.NoError(t, err)
	marker, err := os.Stat(witness)
	require.NoError(t, err)
	require.False(t, os.SameFile(published, marker), "witness must not retain guest inode")
	require.NoError(t, os.Remove(name))

	_, adopted, err := client.AdoptCopy(pool, "guest.img", &libvirtstorage.CopyCache{Image: true})
	require.NoError(t, err)
	require.False(t, adopted)
	require.NoFileExists(t, name)

	_, adopted, err = client.AdoptCopy(pool, "guest.img", &libvirtstorage.CopyCache{Image: true})
	require.NoError(t, err)
	require.False(t, adopted)
	require.NoFileExists(t, name)
}

func TestImageImportFailuresDoNotPublish(t *testing.T) {
	binary, err := exec.LookPath("qemu-img")
	if err != nil {
		t.Skip("qemu-img required")
	}

	for _, kind := range []string{"undersized", "digest", "malformed", "collision"} {
		t.Run(kind, func(t *testing.T) {
			rpc, client, pool := newVolumeFixture(t)

			data := make([]byte, 1<<20)
			if kind == "malformed" {
				copy(data, "QFI\xfb")
			}

			source := filepath.Join(t.TempDir(), "source")
			require.NoError(t, os.WriteFile(source, data, 0o600))

			expected := digest.SHA256.FromBytes(data).String()
			if kind == "digest" {
				expected = digest.SHA256.FromString("wrong").String()
			}

			capacity := uint64(2 << 20)
			if kind == "undersized" {
				capacity = 512
			}

			if kind == "collision" {
				require.NoError(t, os.WriteFile(filepath.Join(rpc.target, "guest.img"), []byte("foreign disk"), 0o600))
			}

			op, err := client.PrepareCopy(pool, "guest.img", source, expected)
			require.NoError(t, err)

			defer op.Close()

			err = op.Import(t.Context(), qemuimg.New(binary), "qcow2", capacity)
			if kind == "collision" {
				require.NoError(t, err)
				_, err = client.FinishCopy(t.Context(), pool, op, func() error { return nil })
				require.Error(t, err)
				contents, err := os.ReadFile(filepath.Join(rpc.target, "guest.img"))
				require.NoError(t, err)
				require.Equal(t, "foreign disk", string(contents))
			} else {
				require.Error(t, err)
				require.True(t, qemuimg.IsTerminal(err), "permanent %s rejection must be terminal: %v", kind, err)
				require.NoFileExists(t, filepath.Join(rpc.target, "guest.img"))
			}
		})
	}
}

func TestImageImportToolFailureNeverCreatesBlankDisk(t *testing.T) {
	binary, err := exec.LookPath("qemu-img")
	if err != nil {
		t.Skip("qemu-img required")
	}

	rpc, client, pool := newVolumeFixture(t)
	source := filepath.Join(t.TempDir(), "source")
	require.NoError(t, os.WriteFile(source, make([]byte, 1<<20), 0o600))
	tool := filepath.Join(t.TempDir(), "no-space")
	require.NoError(t, os.WriteFile(tool, []byte(fmt.Sprintf(
		"#!/bin/sh\nif [ \"$1\" = convert ]; then echo 'No space left on device' >&2; exit 1; fi\nexec '%s' \"$@\"\n", binary)), 0o700))

	op, err := client.PrepareCopy(pool, "guest.img", source, "")
	require.NoError(t, err)

	defer op.Close()

	err = op.Import(t.Context(), qemuimg.New(tool), "qcow2", 2<<20)

	var failure *qemuimg.ExecError
	require.ErrorAs(t, err, &failure)
	require.Contains(t, string(failure.Stderr), "No space left on device")
	op.Close()
	require.NoFileExists(t, filepath.Join(rpc.target, "guest.img"))
	entries, err := os.ReadDir(rpc.target)
	require.NoError(t, err)
	require.Len(t, entries, 1)
}

func TestWritableImageRejectsImmutableCopyOwnership(t *testing.T) {
	_, client, pool := newVolumeFixture(t)
	source := filepath.Join(t.TempDir(), "source")
	require.NoError(t, os.WriteFile(source, []byte("immutable media"), 0o600))
	op, err := client.PrepareCopy(pool, "guest.img", source, "")
	require.NoError(t, err)

	defer op.Close()

	require.NoError(t, op.Transfer(t.Context()))
	_, err = client.FinishCopy(t.Context(), pool, op, func() error { return nil })
	require.NoError(t, err)
	_, adopted, err := client.AdoptCopy(pool, "guest.img", &libvirtstorage.CopyCache{Image: true})
	require.Error(t, err)
	require.False(t, adopted)
}

func TestImageImportCancellationDrainsRealQEMU(t *testing.T) {
	binary, err := exec.LookPath("qemu-img")
	if err != nil {
		t.Skip("qemu-img required")
	}

	rpc, client, pool := newVolumeFixture(t)
	source := filepath.Join(t.TempDir(), "source")

	data := make([]byte, 4<<20)
	for i := range data {
		data[i] = byte(i%251 + 1)
	}

	require.NoError(t, os.WriteFile(source, data, 0o600))
	directory := t.TempDir()
	pidfile := filepath.Join(directory, "pid")
	tool := filepath.Join(directory, "slow-qemu")
	require.NoError(t, os.WriteFile(tool, []byte(fmt.Sprintf(
		"#!/bin/sh\nif [ \"$1\" = convert ]; then\n echo $$ > '%s'\n shift\n exec '%s' convert -r 65536 \"$@\"\nfi\nexec '%s' \"$@\"\n", pidfile, binary, binary)), 0o700))

	toolClient := qemuimg.New(tool)

	op, err := client.PrepareCopy(pool, "guest.img", source, digest.SHA256.FromBytes(data).String())
	require.NoError(t, err)

	defer op.Close()

	ctx, cancel := context.WithCancel(t.Context())

	done := make(chan error, 1)
	go func() { done <- op.Import(ctx, toolClient, "qcow2", 8<<20) }()

	defer func() {
		cancel()
		<-done
	}()

	var pid int

	loggedProbe := false

	require.Eventually(t, func() bool {
		select {
		case err := <-done:
			done <- err

			require.NoError(t, err, "import exited before a converter was observed")

			return false
		default:
		}

		content, err := os.ReadFile(pidfile)
		if err != nil {
			return false
		}

		if _, err := fmt.Sscan(string(content), &pid); err != nil {
			return false
		}

		if !loggedProbe {
			command, commandErr := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
			_, mapsErr := os.ReadFile(fmt.Sprintf("/proc/%d/maps", pid))
			t.Logf("converter observation: pid=%d argv=%q argvErr=%v mapsErr=%v", pid, command, commandErr, mapsErr)

			loggedProbe = true
		}

		return ownedQEMUConvert(pid)
	}, 5*time.Second, time.Millisecond)

	argv, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	require.NoError(t, err)
	executable, err := os.Readlink(fmt.Sprintf("/proc/%d/exe", pid))
	require.NoError(t, err)
	t.Logf("active owned QEMU child: pid=%d executable=%s argv=%q", pid, executable, strings.Split(strings.TrimSuffix(string(argv), "\x00"), "\x00"))

	cancel()

	err = <-done
	done <- err

	require.ErrorIs(t, err, context.Canceled)
	_, err = os.Stat(fmt.Sprintf("/proc/%d", pid))
	require.ErrorIs(t, err, os.ErrNotExist)
	t.Logf("canceled QEMU child %d exited; /proc identity is absent", pid)
	op.Close()
	require.NoFileExists(t, filepath.Join(rpc.target, "guest.img"))
	entries, err := os.ReadDir(rpc.target)
	require.NoError(t, err)
	require.Len(t, entries, 1, "only the empty ownership directory remains")
}

// The packaged executable can be entered through an explicit musl loader. Check
// the owned exec PID's arguments and mapped ELF identity, not its kernel comm.
func ownedQEMUConvert(pid int) bool {
	command, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil {
		return false
	}

	args := strings.Split(strings.TrimSuffix(string(command), "\x00"), "\x00")

	convert := slices.Index(args, "convert")
	if convert < 1 || len(args) <= convert+2 || args[convert+1] != "-r" || args[convert+2] != "65536" {
		return false
	}

	maps, err := os.ReadFile(fmt.Sprintf("/proc/%d/maps", pid))
	if err != nil {
		return false
	}

	return qemuExecutableMapped(args[:convert], string(maps))
}

func qemuExecutableMapped(args []string, maps string) bool {
	for _, argument := range args {
		if filepath.Base(argument) != "qemu-img" {
			continue
		}

		identity, err := os.Stat(argument)
		if err != nil {
			continue
		}

		for line := range strings.SplitSeq(maps, "\n") {
			fields := strings.Fields(line)
			if len(fields) < 6 || !strings.Contains(fields[1], "x") {
				continue
			}

			// The parser has a private root, so compare device/inode identity.
			if mappedFileMatches(identity.Sys().(*syscall.Stat_t), fields[3], fields[4]) {
				return true
			}
		}
	}

	return false
}

func mappedFileMatches(stat *syscall.Stat_t, deviceID, inodeID string) bool {
	device := strings.Split(deviceID, ":")
	if len(device) != 2 {
		return false
	}

	major, majorErr := strconv.ParseUint(device[0], 16, 32)
	minor, minorErr := strconv.ParseUint(device[1], 16, 32)
	inode, inodeErr := strconv.ParseUint(inodeID, 10, 64)

	return majorErr == nil && minorErr == nil && inodeErr == nil && inode == stat.Ino &&
		uint32(major) == unix.Major(stat.Dev) && uint32(minor) == unix.Minor(stat.Dev)
}

// Libvirt's cached raw driver remains authoritative after the guest writes a
// nested image header. Host adoption must never reinterpret guest sectors.
type rawObservationRPC struct {
	*rpc
	targetReads int
}

func (r *rawObservationRPC) StorageVolGetXMLDesc(vol libvirt.StorageVol, flags uint32) (string, error) {
	return (&libvirtxml.StorageVolume{Name: vol.Name, Target: &libvirtxml.StorageVolumeTarget{
		Format: &libvirtxml.StorageVolumeTargetFormat{Type: "raw"},
	}}).Marshal()
}

func (r *rawObservationRPC) StoragePoolGetXMLDesc(pool libvirt.StoragePool, flags libvirt.StorageXMLFlags) (string, error) {
	r.targetReads++

	return r.rpc.StoragePoolGetXMLDesc(pool, flags)
}

func TestWritableRawAdoptionNeverSniffsGuestSectors(t *testing.T) {
	rpc, _, pool := newVolumeFixture(t)
	observed := &rawObservationRPC{rpc: rpc}
	client := libvirtstorage.NewTestClient(observed)
	name := "guest.raw"
	data := make([]byte, 4096)
	copy(data, "QFI\xfb") // Deliberately invalid qcow2, entirely valid raw guest data.
	require.NoError(t, os.WriteFile(filepath.Join(rpc.target, name), data, 0o600))
	root, err := os.OpenRoot(rpc.target)
	require.NoError(t, err)

	defer root.Close() //nolint:errcheck

	require.NoError(t, root.Mkdir(".copy-on-use", 0o700))

	witness := fmt.Sprintf(".copy-on-use/image-%x", sha256.Sum256([]byte(name)))
	owner := imageOwnerFixtureToken(t, filepath.Join(rpc.target, name))
	marker := "image-owner:raw:" + owner + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(rpc.target, witness), []byte(marker), 0o600))

	cache := &libvirtstorage.CopyCache{Image: true}

	for range 2 { // Cold recovery and warm proof both preserve raw guest bytes.
		before := observed.targetReads
		volume, adopted, err := client.AdoptCopy(pool, name, cache)
		require.NoError(t, err)
		require.True(t, adopted)
		require.Equal(t, "raw", volume.Format)
		require.Equal(t, before+1, observed.targetReads, "one pool-target observation per adoption")
	}

	require.Equal(t, data, mustReadVolume(t, filepath.Join(rpc.target, name)))
}

func TestImageImportPreflightBeforeSourceRead(t *testing.T) {
	for _, kind := range []string{"missing tool", "unexecutable tool", "malformed source", "undersized"} {
		t.Run(kind, func(t *testing.T) {
			rpc, client, pool := newVolumeFixture(t)
			source := filepath.Join(t.TempDir(), "source")

			data := make([]byte, 1<<20)
			if kind == "malformed source" {
				copy(data, "QFI\xfb")
			}

			require.NoError(t, os.WriteFile(source, data, 0o600))

			tool := "/bin/true"
			if kind == "unexecutable tool" {
				tool = filepath.Join(t.TempDir(), "bad-interpreter")
				require.NoError(t, os.WriteFile(tool, []byte("#!/missing-dynamic-loader\n"), 0o700))
			}

			if kind == "missing tool" {
				tool = "/absent-qemu-img"
			}

			capacity := uint64(2 << 20)
			if kind == "undersized" {
				capacity = 512
			}

			op, err := client.PrepareCopy(pool, "guest.img", source, "")
			require.NoError(t, err)

			defer op.Close()

			var processed uint64

			op.SetProgressObserver(func(progress libvirtstorage.CopyProgress) { processed = progress.BytesCopied })
			err = op.Import(t.Context(), qemuimg.New(tool), "qcow2", capacity)
			require.Error(t, err)
			require.Zero(t, processed, "preflight must precede full source transfer/verification")
			require.True(t, qemuimg.IsTerminal(err), "%v", err)
			entries, err := os.ReadDir(rpc.target)
			require.NoError(t, err)
			require.Len(t, entries, 1, "no source or output stage before preflight succeeds")
		})
	}
}

type staleImageRPC struct {
	*rpc
	stale bool
}

func (r *staleImageRPC) StoragePoolRefresh(pool libvirt.StoragePool, flags uint32) error {
	r.stale = false

	return r.rpc.StoragePoolRefresh(pool, flags)
}

func (r *staleImageRPC) StorageVolGetXMLDesc(vol libvirt.StorageVol, flags uint32) (string, error) {
	if r.stale {
		return (&libvirtxml.StorageVolume{Name: vol.Name, Target: &libvirtxml.StorageVolumeTarget{Format: &libvirtxml.StorageVolumeTargetFormat{Type: "raw"}}}).Marshal()
	}

	return r.rpc.StorageVolGetXMLDesc(vol, flags)
}

func TestImageFinishRefreshesPreviouslyObservedStage(t *testing.T) {
	binary, err := exec.LookPath("qemu-img")
	if err != nil {
		t.Skip("qemu-img required")
	}

	rpc, _, pool := newVolumeFixture(t)
	observed := &staleImageRPC{rpc: rpc}
	client := libvirtstorage.NewTestClient(observed)
	source := filepath.Join(t.TempDir(), "source")
	require.NoError(t, os.WriteFile(source, make([]byte, 1<<20), 0o600))
	op, err := client.PrepareCopy(pool, "guest.qcow2", source, "")
	require.NoError(t, err)

	defer op.Close()

	require.NoError(t, op.Import(t.Context(), qemuimg.New(binary), "qcow2", 2<<20))
	// Another operation refreshed while this name still held a raw/empty inode.
	// Lookup now hits; only an explicit refresh can invalidate its cached format.
	require.NoError(t, rpc.StoragePoolRefresh(libvirt.StoragePool{}, 0))

	observed.stale = true
	volume, err := client.FinishCopy(t.Context(), pool, op, func() error { return nil })
	require.NoError(t, err)
	require.Equal(t, "qcow2", volume.Format)
}

func TestImageConversionReadsVerifiedOriginalInode(t *testing.T) {
	binary, err := exec.LookPath("qemu-img")
	if err != nil {
		t.Skip("qemu-img required")
	}

	rpc, client, pool := newVolumeFixture(t)
	source := filepath.Join(t.TempDir(), "source")
	data := make([]byte, 1<<20)
	require.NoError(t, os.WriteFile(source, data, 0o600))
	tool := filepath.Join(t.TempDir(), "tool")
	script := fmt.Sprintf(`#!/bin/sh
if [ "$1" = convert ]; then
 previous=; last=
 for argument do previous=$last; last=$argument; done
 [ "$previous" -ef '%s' ] || { echo 'conversion did not use verified source inode' >&2; exit 29; }
fi
exec '%s' "$@"
`, source, binary)
	require.NoError(t, os.WriteFile(tool, []byte(script), 0o700))

	op, err := client.PrepareCopy(pool, "guest.qcow2", source, digest.SHA256.FromBytes(data).String())
	require.NoError(t, err)

	defer op.Close()

	require.NoError(t, op.Import(t.Context(), qemuimg.New(tool), "qcow2", 2<<20))

	entries, err := os.ReadDir(rpc.target)
	require.NoError(t, err)
	require.Len(t, entries, 2, "one output stage and ownership directory; no full source copy")
}

func TestImageImportRejectsRestoredSourceMutation(t *testing.T) {
	binary, err := exec.LookPath("qemu-img")
	if err != nil {
		t.Skip("qemu-img required")
	}

	rpc, client, pool := newVolumeFixture(t)
	source := filepath.Join(t.TempDir(), "source")
	data := make([]byte, 1<<20)
	require.NoError(t, os.WriteFile(source, data, 0o600))
	times := source + ".times"
	stat, err := os.Stat(source)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(times, nil, 0o600))
	require.NoError(t, os.Chtimes(times, stat.ModTime(), stat.ModTime()))
	tool := filepath.Join(t.TempDir(), "tool")
	script := fmt.Sprintf(`#!/bin/sh
if [ "$1" = convert ]; then
 printf x | dd of='%s' bs=1 count=1 conv=notrunc 2>/dev/null || exit 32
 printf '\000' | dd of='%s' bs=1 count=1 conv=notrunc 2>/dev/null || exit 32
 touch -r '%s' '%s' || exit 32
fi
exec '%s' "$@"
`, source, source, times, source, binary)
	require.NoError(t, os.WriteFile(tool, []byte(script), 0o700))

	op, err := client.PrepareCopy(pool, "guest.img", source, digest.SHA256.FromBytes(data).String())
	require.NoError(t, err)

	defer op.Close()

	err = op.Import(t.Context(), qemuimg.New(tool), "qcow2", 2<<20)
	require.ErrorIs(t, err, libvirtstorage.ErrCopySourceChanged)
	require.False(t, qemuimg.IsTerminal(err))
	require.NoFileExists(t, filepath.Join(rpc.target, "guest.img"))
}

func TestWritableImageOwnershipRejectsRecycledInode(t *testing.T) {
	binary, err := exec.LookPath("qemu-img")
	if err != nil {
		t.Skip("qemu-img required")
	}

	rpc, client, pool := newVolumeFixture(t)
	source := filepath.Join(t.TempDir(), "source.raw")
	require.NoError(t, os.WriteFile(source, make([]byte, 1<<20), 0o600))
	op, err := client.PrepareCopy(pool, "guest.img", source, "")
	require.NoError(t, err)

	t.Cleanup(op.Close)

	require.NoError(t, op.Import(t.Context(), qemuimg.New(binary), "raw", 2<<20))
	_, err = client.FinishCopy(t.Context(), pool, op, func() error { return nil })
	require.NoError(t, err)
	op.Close() // Release the unpublished stage's link before deleting the public disk.

	warm := &libvirtstorage.CopyCache{Image: true}
	_, adopted, err := client.AdoptCopy(pool, "guest.img", warm)
	require.NoError(t, err)
	require.True(t, adopted)

	name := filepath.Join(rpc.target, "guest.img")
	witness := filepath.Join(rpc.target, fmt.Sprintf(".copy-on-use/image-%x", sha256.Sum256([]byte("guest.img"))))
	marker, err := os.ReadFile(witness)
	require.NoError(t, err)
	require.NoError(t, os.Remove(name))

	foreign := make([]byte, 2<<20)
	copy(foreign, "foreign replacement; not the original guest")
	require.NoError(t, os.WriteFile(name, foreign, 0o600))
	// The marker still names the old owner; the replacement carries no owner
	// attribute, so even a recycled inode number cannot claim it.
	require.Contains(t, string(marker), "image-owner:")

	for _, cache := range []*libvirtstorage.CopyCache{warm, {Image: true}} {
		volume, _, err := client.AdoptCopy(pool, "guest.img", cache)
		require.Error(t, err)
		require.Zero(t, volume, "recycled inode numbers do not establish writable ownership")
	}

	outcome, err := client.ResizeVolumeOperation(pool, "guest.img", 4*1024*1024)
	require.Error(t, err)
	require.Equal(t, libvirtstorage.NoMutationSubmitted, outcome)
	require.NoError(t, os.WriteFile(witness, []byte("invalid ownership marker"), 0o600))

	outcome, err = client.ResizeVolumeOperation(pool, "guest.img", 4*1024*1024)
	require.Error(t, err)
	require.Equal(t, libvirtstorage.NoMutationSubmitted, outcome, "invalid image ownership must not fall back to libvirt probing")

	got, err := os.ReadFile(name)
	require.NoError(t, err)
	require.Equal(t, foreign, got)
}

func imageOwnerFixtureToken(t *testing.T, path string) string {
	t.Helper()

	owner := uuid.NewString()
	require.NoError(t, unix.Setxattr(path, "user.talos.image-owner", []byte(owner), unix.XATTR_CREATE))

	return owner
}
