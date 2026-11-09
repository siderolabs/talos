// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/google/uuid"
	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	storagectrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/storage"
	libvirtstorage "github.com/siderolabs/talos/internal/pkg/libvirt/storage"
	"github.com/siderolabs/talos/internal/pkg/libvirt/storage/copyowner"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	storageres "github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

// Serialize the fixture RPC's sessions, while keeping the real filesystem
// transfer outside the session lock, as it is in production.
type retrySourceSession struct {
	libvirtstorage.VolumeClient
	unlock   func()
	prepared chan struct{}
}

func TestCopySourceIdentityChangesAreRetryable(t *testing.T) {
	for _, replace := range []bool{false, true} {
		t.Run(fmt.Sprint("replace=", replace), func(t *testing.T) {
			rpc, client, pool := newVolumeFixture(t)
			source := filepath.Join(t.TempDir(), "source")
			require.NoError(t, os.WriteFile(source, []byte("original"), 0o600))
			op, err := client.PrepareCopy(pool, "guest.raw", source, "")
			require.NoError(t, err)

			defer op.Close()

			var (
				once      sync.Once
				changeErr error
			)

			op.SetProgressObserver(func(libvirtstorage.CopyProgress) {
				once.Do(func() {
					if replace {
						changeErr = os.WriteFile(source+".new", []byte("original"), 0o600)
						if changeErr == nil {
							changeErr = os.Rename(source+".new", source)
						}
					} else {
						changeErr = os.Truncate(source, 1)
					}
				})
			})
			require.ErrorIs(t, op.Transfer(t.Context()), libvirtstorage.ErrCopySourceChanged)
			require.NoError(t, changeErr)
			require.NoFileExists(t, filepath.Join(rpc.target, "guest.raw"))
		})
	}
}

func (s *retrySourceSession) Close() { s.unlock() }

func (s *retrySourceSession) PrepareCopy(pool libvirtstorage.Pool, name, source, expected string) (*libvirtstorage.CopyOperation, error) {
	s.prepared <- struct{}{}

	return s.VolumeClient.PrepareCopy(pool, name, source, expected)
}

func TestCopyUnsupportedFormatWaitsForChangedIntent(t *testing.T) {
	rpc, _, pool := newVolumeFixture(t)
	source := filepath.Join(t.TempDir(), "source.qcow2")
	require.NoError(t, os.WriteFile(source, qcow2Magic, 0o600))

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

	spec := storageres.NewStoragePoolVolumeSpec(storageres.NamespaceName, storageres.StoragePoolVolumeID(pool.Name, "guest.raw"))
	*spec.TypedSpec() = storageres.StoragePoolVolumeSpecSpec{
		Pool:   pool.Name,
		Name:   "guest.raw",
		Format: "raw",
		Copy: &storageres.StoragePoolVolumeCopySpec{
			Source: source,
			Digest: digest.SHA256.FromBytes(qcow2Magic).String(),
		},
	}
	s.Create(spec, state.WithCreateOwner("hypervisor.VirtualMachineDiskController"))

	var sessions sync.Mutex

	prepared := make(chan struct{}, 10)

	require.NoError(t, s.Runtime().RegisterController(&storagectrl.StoragePoolVolumeController{
		Open: func(context.Context) (libvirtstorage.VolumeClient, error) {
			sessions.Lock()

			return &retrySourceSession{VolumeClient: libvirtstorage.NewTestClient(rpc), unlock: sessions.Unlock, prepared: prepared}, nil
		},
	}))

	select {
	case <-prepared:
	case <-time.After(3 * time.Second):
		t.Fatal("copy did not start")
	}

	require.Eventually(t, func() bool {
		status, err := safe.StateGetByID[*storageres.StoragePoolVolumeStatus](s.Ctx(), s.State(), spec.Metadata().ID())

		return err == nil && status.TypedSpec().Copy != nil && status.TypedSpec().Copy.Phase == storageres.StoragePoolVolumeCopyPhaseFailed
	}, 3*time.Second, 10*time.Millisecond)

	select {
	case <-prepared:
		t.Fatal("unsupported format must not repeat the transfer for unchanged intent")
	case <-time.After(1500 * time.Millisecond):
	}

	require.NoFileExists(t, filepath.Join(rpc.target, "guest.raw"))

	data := []byte("supported raw image")
	replacement := filepath.Join(t.TempDir(), "source.raw")
	require.NoError(t, os.WriteFile(replacement, data, 0o600))

	current, err := safe.StateGetByID[*storageres.StoragePoolVolumeSpec](s.Ctx(), s.State(), spec.Metadata().ID())
	require.NoError(t, err)

	current.TypedSpec().Copy.Source = replacement
	current.TypedSpec().Copy.Digest = digest.SHA256.FromBytes(data).String()
	require.NoError(t, s.State().Update(s.Ctx(), current, state.WithUpdateOwner("hypervisor.VirtualMachineDiskController")))

	require.Eventually(t, func() bool {
		status, getErr := safe.StateGetByID[*storageres.StoragePoolVolumeStatus](s.Ctx(), s.State(), spec.Metadata().ID())

		return getErr == nil && status.TypedSpec().Phase == storageres.StoragePoolVolumePhaseReady &&
			status.TypedSpec().Copy != nil && status.TypedSpec().Copy.Phase == storageres.StoragePoolVolumeCopyPhaseSucceeded
	}, 3*time.Second, 10*time.Millisecond)
	require.Equal(t, data, mustReadVolume(t, filepath.Join(rpc.target, "guest.raw")))
}

//nolint:cyclop,gocyclo // Compare retry, permanent rejection and obsolete intent through the same real transfer path.
func TestCopySourceIdentityRetryWithoutResourceEvent(t *testing.T) {
	for _, tc := range []struct {
		name          string
		permanent     bool
		directory     bool
		changedIntent bool
		withdrawn     bool
	}{
		{name: "same digest path replacement"},
		{name: "digest mismatch", permanent: true},
		{name: "nonregular source", permanent: true, directory: true},
		{name: "changed intent", changedIntent: true},
		{name: "withdrawn intent", withdrawn: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rpc, _, pool := newVolumeFixture(t)
			source := filepath.Join(t.TempDir(), "object")
			data := []byte("same digest immutable image bytes")

			if tc.directory {
				require.NoError(t, os.Mkdir(source, 0o700))
			} else {
				require.NoError(t, os.WriteFile(source, data, 0o600))
			}

			var (
				once     sync.Once
				sessions sync.Mutex
			)

			prepared := make(chan struct{}, 10)
			changed := make(chan error, 1)
			core, _ := observer.New(zap.DebugLevel)
			suite := &ctest.DefaultSuite{}
			spec := storageres.NewStoragePoolVolumeSpec(storageres.NamespaceName, storageres.StoragePoolVolumeID(pool.Name, "guest.raw"))
			suite.Logger = zap.New(core, zap.Hooks(func(entry zapcore.Entry) error {
				if entry.Message == "copy provisioning progress" {
					once.Do(func() {
						// Transfer has opened the old inode. Replace its path with
						// identical bytes before it checks the source identity.
						err := os.WriteFile(source+".replacement", data, 0o600)
						if err == nil {
							err = os.Rename(source+".replacement", source)
						}

						if err == nil && tc.withdrawn {
							_, err = suite.State().Teardown(suite.Ctx(), spec.Metadata(), state.WithTeardownOwner("hypervisor.VirtualMachineDiskController"))
						}

						if err == nil && tc.changedIntent {
							current, getErr := safe.StateGetByID[*storageres.StoragePoolVolumeSpec](suite.Ctx(), suite.State(), spec.Metadata().ID())

							err = getErr
							if err == nil {
								current.TypedSpec().Copy.Source = ""
								err = suite.State().Update(suite.Ctx(), current, state.WithUpdateOwner("hypervisor.VirtualMachineDiskController"))
							}
						}

						changed <- err
					})
				}

				return nil
			}))
			suite.SetT(t)
			suite.SetupTest()
			t.Cleanup(suite.TearDownTest)

			system := hardware.NewSystemInformation(hardware.SystemInformationID)
			system.TypedSpec().UUID = machine
			suite.Create(system)

			poolStatus := storageres.NewStoragePoolStatus(storageres.NamespaceName, pool.Name)
			*poolStatus.TypedSpec() = storageres.StoragePoolStatusSpec{Phase: storageres.StoragePoolPhaseReady, VolumeID: "backing", TargetPath: rpc.target}
			suite.Create(poolStatus, state.WithCreateOwner("storage.StoragePoolController"))

			*spec.TypedSpec() = storageres.StoragePoolVolumeSpecSpec{
				Pool: pool.Name, Name: "guest.raw", Format: "raw",
				Copy: &storageres.StoragePoolVolumeCopySpec{Source: source, Digest: digest.SHA256.FromBytes(data).String()},
			}
			if tc.permanent && !tc.directory {
				spec.TypedSpec().Copy.Digest = digest.SHA256.FromString("wrong digest").String()
			}

			suite.Create(spec, state.WithCreateOwner("hypervisor.VirtualMachineDiskController"))
			require.NoError(t, suite.Runtime().RegisterController(&storagectrl.StoragePoolVolumeController{
				Open: func(context.Context) (libvirtstorage.VolumeClient, error) {
					sessions.Lock()

					return &retrySourceSession{VolumeClient: libvirtstorage.NewTestClient(rpc), unlock: sessions.Unlock, prepared: prepared}, nil
				},
			}))

			select {
			case <-prepared:
			case <-time.After(3 * time.Second):
				t.Fatal("copy did not start")
			}

			if !tc.directory {
				select {
				case err := <-changed:
					require.NoError(t, err)
				case <-time.After(3 * time.Second):
					t.Fatal("transfer boundary was not observed")
				}
			}

			if tc.permanent || tc.changedIntent || tc.withdrawn {
				if tc.permanent {
					require.Eventually(t, func() bool {
						status, err := safe.StateGetByID[*storageres.StoragePoolVolumeStatus](suite.Ctx(), suite.State(), spec.Metadata().ID())

						return err == nil && status.TypedSpec().Copy != nil && status.TypedSpec().Copy.Phase == storageres.StoragePoolVolumeCopyPhaseFailed
					}, 3*time.Second, 10*time.Millisecond)
				}

				select {
				case <-prepared:
					t.Fatal("permanent or obsolete copy must not retry")
				case <-time.After(1500 * time.Millisecond):
				}

				require.NoFileExists(t, filepath.Join(rpc.target, "guest.raw"))

				return
			}

			select {
			case <-prepared:
			case <-time.After(4 * time.Second):
				t.Fatal("source identity race must retry automatically without a resource event")
			}

			require.Eventually(t, func() bool {
				status, err := safe.StateGetByID[*storageres.StoragePoolVolumeStatus](suite.Ctx(), suite.State(), spec.Metadata().ID())

				return err == nil && status.TypedSpec().Phase == storageres.StoragePoolVolumePhaseReady &&
					status.TypedSpec().Copy != nil && status.TypedSpec().Copy.Phase == storageres.StoragePoolVolumeCopyPhaseSucceeded
			}, 3*time.Second, 10*time.Millisecond)
			require.Equal(t, data, mustReadVolume(t, filepath.Join(rpc.target, "guest.raw")))
		})
	}
}

// liveCopyPool uses an explicitly selected disposable libvirt daemon, never the host default.
// TALOS_TEST_STORAGE_SOCKET must name its Unix socket; the daemon must accept qemu:///session.
func liveCopyPool(t *testing.T) (*libvirtstorage.Connector, libvirtstorage.Pool) {
	t.Helper()

	socket := os.Getenv("TALOS_TEST_STORAGE_SOCKET")
	if socket == "" {
		t.Skip("set TALOS_TEST_STORAGE_SOCKET to an isolated libvirt daemon socket")
	}

	connector := libvirtstorage.New(socket, "qemu:///session")
	pool := libvirtstorage.Pool{
		Name:   "talos-copy-test-" + uuid.NewString(),
		UUID:   uuid.New(),
		Target: filepath.Join(t.TempDir(), "pool"),
	}

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	client, err := connector.Open(ctx)
	require.NoError(t, err)

	defer client.Close()

	_, err = client.EnsureOperation(pool, pool.Target, func() error {
		return os.Mkdir(pool.Target, 0o700)
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()

		cleanupClient, cleanupErr := connector.Open(cleanupCtx)
		require.NoError(t, cleanupErr)

		defer cleanupClient.Close()

		_, cleanupErr = cleanupClient.RemoveOperation(pool)
		require.NoError(t, cleanupErr)
	})

	return connector, pool
}

func liveCopyStage(t *testing.T, connector *libvirtstorage.Connector, pool libvirtstorage.Pool) *libvirtstorage.CopyOperation {
	t.Helper()

	source := filepath.Join(t.TempDir(), "source.raw")
	require.NoError(t, os.WriteFile(source, []byte("independent raw media"), 0o400))

	client, err := connector.OpenVolumes(t.Context())
	require.NoError(t, err)

	op, err := client.PrepareCopy(pool, "guest.raw", source, "")
	client.Close()
	require.NoError(t, err)
	t.Cleanup(op.Close)

	return op
}

func TestCopyOperationLiveSeparateSessions(t *testing.T) {
	connector, pool := liveCopyPool(t)
	op := liveCopyStage(t, connector, pool)

	var phases []libvirtstorage.CopyPhase

	op.SetProgressObserver(func(p libvirtstorage.CopyProgress) {
		if len(phases) == 0 || phases[len(phases)-1] != p.Phase {
			phases = append(phases, p.Phase)
		}
	})

	// Preparation's transport has already closed. No libvirt session exists during transfer.
	require.NoError(t, op.Transfer(t.Context()))

	client, err := connector.OpenVolumes(t.Context())
	require.NoError(t, err)

	defer client.Close()

	volume, err := client.FinishCopy(t.Context(), pool, op, func() error { return nil })
	require.NoError(t, err)
	require.Equal(t, filepath.Join(pool.Target, "guest.raw"), volume.Path)

	contents, err := os.ReadFile(volume.Path)
	require.NoError(t, err)
	require.Equal(t, "independent raw media", string(contents))
	require.Equal(t, []libvirtstorage.CopyPhase{
		libvirtstorage.CopyPhaseTransfer, libvirtstorage.CopyPhaseVerify, libvirtstorage.CopyPhaseSync,
		libvirtstorage.CopyPhaseValidate, libvirtstorage.CopyPhasePublish, libvirtstorage.CopyPhaseCleanup,
	}, phases)
}

func TestCopyOperationLiveRejectsStaleCompletion(t *testing.T) {
	connector, pool := liveCopyPool(t)
	op := liveCopyStage(t, connector, pool)
	require.NoError(t, op.Transfer(t.Context()))

	client, err := connector.OpenVolumes(t.Context())
	require.NoError(t, err)

	defer client.Close()

	stale := errors.New("request changed")
	_, err = client.FinishCopy(t.Context(), pool, op, func() error { return stale })
	require.ErrorIs(t, err, stale)
	require.NoFileExists(t, filepath.Join(pool.Target, "guest.raw"))
}

func TestCopyOperationLiveRejectsChangedBacking(t *testing.T) {
	connector, pool := liveCopyPool(t)
	op := liveCopyStage(t, connector, pool)
	require.NoError(t, op.Transfer(t.Context()))
	require.NoError(t, os.Rename(pool.Target, pool.Target+"-old"))
	require.NoError(t, os.Mkdir(pool.Target, 0o700))

	client, err := connector.OpenVolumes(t.Context())
	require.NoError(t, err)

	defer client.Close()

	_, err = client.FinishCopy(t.Context(), pool, op, func() error { return nil })
	require.ErrorContains(t, err, "backing changed")
	require.NoFileExists(t, filepath.Join(pool.Target, "guest.raw"))
	require.NoFileExists(t, filepath.Join(pool.Target+"-old", "guest.raw"))
}

func TestCopyOperationLiveActiveStageSurvivesSweep(t *testing.T) {
	connector, pool := liveCopyPool(t)
	op := liveCopyStage(t, connector, pool)
	require.NoError(t, op.Transfer(t.Context()))

	client, err := connector.OpenVolumes(t.Context())
	require.NoError(t, err)

	defer client.Close()

	_, err = client.CreateVolume(pool, "unrelated.raw", "raw", 4096, "", "")
	require.NoError(t, err)

	_, err = client.FinishCopy(t.Context(), pool, op, func() error { return nil })
	require.NoError(t, err)
}

// cancelCopyContext cancels at a deterministic cooperative transfer checkpoint.
// Its real Context cancellation and Done channel remain consistent.
type cancelCopyContext struct {
	context.Context //nolint:containedctx // The test decorates a real Context to cancel at a deterministic transfer checkpoint.
	cancel          context.CancelFunc
	checks          atomic.Int32
}

func (ctx *cancelCopyContext) Err() error {
	if ctx.checks.Add(1) == 8 {
		ctx.cancel()
	}

	return ctx.Context.Err()
}

func TestCopyOperationLiveCancelsDuringTransfer(t *testing.T) {
	connector, pool := liveCopyPool(t)
	op := liveCopyStage(t, connector, pool)
	require.NoError(t, os.Chmod(op.Source, 0o600))
	require.NoError(t, os.Truncate(op.Source, 1<<20))

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	transferCtx := &cancelCopyContext{Context: ctx, cancel: cancel}
	require.ErrorIs(t, op.Transfer(transferCtx), context.Canceled)

	entries, err := os.ReadDir(pool.Target)
	require.NoError(t, err)

	var partial int64

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, statErr := entry.Info()
		require.NoError(t, statErr)

		partial += info.Size()
	}

	require.Positive(t, partial, "cancellation must occur after copying started")
	require.Less(t, partial, int64(1<<20), "cancellation must stop before copying the entire source")
	op.Close()

	entries, err = os.ReadDir(pool.Target)
	require.NoError(t, err)

	for _, entry := range entries {
		require.True(t, entry.IsDir(), "canceled stage must be removed")
	}

	require.NoFileExists(t, filepath.Join(pool.Target, "guest.raw"))
}

func TestCopyAdoptionLiveRecoveryAndCacheInvalidation(t *testing.T) {
	connector, pool := liveCopyPool(t)
	op := liveCopyStage(t, connector, pool)
	require.NoError(t, op.Transfer(t.Context()))

	client, err := connector.OpenVolumes(t.Context())
	require.NoError(t, err)

	defer client.Close()

	volume, err := client.FinishCopy(t.Context(), pool, op, func() error { return nil })
	require.NoError(t, err)
	require.NoError(t, os.Remove(op.Source))

	cache := &libvirtstorage.CopyCache{}
	for range 3 {
		adopted, found, adoptErr := client.AdoptCopy(pool, "guest.raw", cache)
		require.NoError(t, adoptErr)
		require.True(t, found)
		require.Equal(t, volume.Path, adopted.Path)
	}

	// A lost visible link invalidates the cache and must recover through the durable witness.
	require.NoError(t, os.Remove(volume.Path))

	_, found, err := client.AdoptCopy(pool, "guest.raw", cache)
	require.NoError(t, err)
	require.True(t, found)
	require.FileExists(t, volume.Path)

	// A fresh run has no in-memory proof, yet still adopts without the original source.
	_, found, err = client.AdoptCopy(pool, "guest.raw", &libvirtstorage.CopyCache{})
	require.NoError(t, err)
	require.True(t, found)

	// Neither a cached proof nor a cold restart can authorize a foreign same-name replacement.
	require.NoError(t, os.Remove(volume.Path))
	require.NoError(t, os.WriteFile(volume.Path, []byte("foreign"), 0o600))

	_, _, err = client.AdoptCopy(pool, "guest.raw", cache)
	require.ErrorContains(t, err, "ownership witness")

	contents, err := os.ReadFile(volume.Path)
	require.NoError(t, err)
	require.Equal(t, "foreign", string(contents))
}

func TestCopyAdoptionLiveRejectsMissingOwnershipWitness(t *testing.T) {
	connector, pool := liveCopyPool(t)
	op := liveCopyStage(t, connector, pool)
	require.NoError(t, op.Transfer(t.Context()))

	client, err := connector.OpenVolumes(t.Context())
	require.NoError(t, err)

	defer client.Close()

	_, err = client.FinishCopy(t.Context(), pool, op, func() error { return nil })
	require.NoError(t, err)

	cache := &libvirtstorage.CopyCache{}
	_, found, err := client.AdoptCopy(pool, "guest.raw", cache)
	require.NoError(t, err)
	require.True(t, found)

	witness := filepath.Join(pool.Target, ".copy-on-use", fmt.Sprintf("%x", sha256.Sum256([]byte("guest.raw"))))
	require.NoError(t, os.Remove(witness))

	_, _, err = client.AdoptCopy(pool, "guest.raw", cache)
	require.ErrorContains(t, err, "ownership witness")
}

func TestCopyBindingLiveRecoversImmutableWithoutSourceOrPublicLink(t *testing.T) {
	for _, recorded := range []bool{false, true} {
		t.Run(fmt.Sprintf("completed-binding-%t", recorded), func(t *testing.T) {
			testCopyBindingImmutableRecovery(t, recorded)
		})
	}
}

func testCopyBindingImmutableRecovery(t *testing.T, recorded bool) {
	t.Helper()
	connector, pool := liveCopyPool(t)
	source := filepath.Join(t.TempDir(), "source.iso")
	require.NoError(t, os.WriteFile(source, []byte("immutable install media"), 0o400))
	client, err := connector.OpenVolumes(t.Context())
	require.NoError(t, err)

	defer client.Close()

	key := strings.Repeat("c", 64)
	op, err := client.PrepareCopy(pool, "install.iso", source, "")
	require.NoError(t, err)
	t.Cleanup(op.Close)
	require.NoError(t, op.SetBindingKey(key))
	require.NoError(t, op.Transfer(t.Context()))
	volume, err := client.FinishCopy(t.Context(), pool, op, func() error { return nil })
	require.NoError(t, err)
	op.Close()

	if recorded {
		require.NoError(t, new(copyowner.Bindings).Record(pool.Target, key, op.Name))
	}

	require.NoError(t, os.Remove(source))
	require.NoError(t, os.Remove(volume.Path))

	name, err := copyowner.ReadBinding(pool.Target, key)
	require.NoError(t, err)
	require.Equal(t, op.Name, name)
	require.NoFileExists(t, volume.Path, "selection must not mutate the pool")

	recovered, found, err := client.AdoptCopy(pool, name, &libvirtstorage.CopyCache{})
	require.NoError(t, err)
	require.True(t, found)

	contents, err := os.ReadFile(recovered.Path)
	require.NoError(t, err)
	require.Equal(t, "immutable install media", string(contents))
}
