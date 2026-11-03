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

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	libvirtstorage "github.com/siderolabs/talos/internal/pkg/libvirt/storage"
)

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

func TestCopyBindingRequiresMatchingOwnershipAndSurvivesRestart(t *testing.T) {
	target := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(target, ".copy-on-use"), 0o700))

	key := fmt.Sprintf("%x", sha256.Sum256([]byte("intent")))
	name := "vm__install__digest.raw"
	require.NoError(t, os.WriteFile(filepath.Join(target, name), []byte("image"), 0o600))
	require.Error(t, new(libvirtstorage.CopyBindings).Record(target, key, name))
	witness := fmt.Sprintf("%x", sha256.Sum256([]byte(name)))
	require.NoError(t, os.Link(filepath.Join(target, name), filepath.Join(target, ".copy-on-use", witness)))
	require.NoError(t, new(libvirtstorage.CopyBindings).Record(target, key, name))
	got, err := libvirtstorage.ReadCopyBinding(target, key)
	require.NoError(t, err)
	require.Equal(t, name, got)
	require.NoError(t, os.Remove(filepath.Join(target, name)))
	got, err = libvirtstorage.ReadCopyBinding(target, key)
	require.Error(t, err)
	require.Empty(t, got)
}

func TestCopyBindingDoesNotGuessBetweenCompletedCopies(t *testing.T) {
	target := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(target, ".copy-on-use"), 0o700))

	key := fmt.Sprintf("%x", sha256.Sum256([]byte("intent")))

	for _, name := range []string{"a.raw", "b.raw"} {
		require.NoError(t, os.WriteFile(filepath.Join(target, name), []byte(name), 0o600))
		witness := fmt.Sprintf("%x", sha256.Sum256([]byte(name)))
		require.NoError(t, os.Link(filepath.Join(target, name), filepath.Join(target, ".copy-on-use", witness)))
	}

	got, err := libvirtstorage.ReadCopyBinding(target, key)
	require.NoError(t, err)
	require.Empty(t, got)
	require.NoError(t, new(libvirtstorage.CopyBindings).Record(target, key, "a.raw"))
	got, err = libvirtstorage.ReadCopyBinding(target, key)
	require.NoError(t, err)
	require.Equal(t, "a.raw", got)
	require.NoError(t, new(libvirtstorage.CopyBindings).Record(target, key, "b.raw"))
	got, err = libvirtstorage.ReadCopyBinding(target, key)
	require.NoError(t, err)
	require.Equal(t, "b.raw", got)
}

func TestCopyBindingWarmRecordStable(t *testing.T) {
	target := t.TempDir()
	key := strings.Repeat("a", 64)
	name := "a.raw"

	require.NoError(t, os.Mkdir(filepath.Join(target, ".copy-on-use"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(target, name), []byte("image"), 0o600))
	witness := fmt.Sprintf("%x", sha256.Sum256([]byte(name)))
	require.NoError(t, os.Link(filepath.Join(target, name), filepath.Join(target, ".copy-on-use", witness)))

	bindings := libvirtstorage.CopyBindings{}
	require.NoError(t, bindings.Record(target, key, name))
	path := filepath.Join(target, ".copy-on-use", "binding-"+key)
	before, err := os.Stat(path)
	require.NoError(t, err)

	for range 3 {
		require.NoError(t, bindings.Record(target, key, name))
	}

	after, err := os.Stat(path)
	require.NoError(t, err)
	require.True(t, os.SameFile(before, after), "warm observations must not rewrite bindings")

	cold := libvirtstorage.CopyBindings{}
	require.NoError(t, cold.Record(target, key, name))
}

func TestCopyBindingRejectsMalformedRecord(t *testing.T) {
	for _, test := range []struct {
		name     string
		contents string
		symlink  bool
	}{
		{name: "empty"},
		{name: "oversize", contents: strings.Repeat("a", 256)},
		{name: "traversal", contents: "../a.raw"},
		{name: "symlink", contents: "a.raw", symlink: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			target := t.TempDir()
			key := strings.Repeat("b", 64)

			require.NoError(t, os.Mkdir(filepath.Join(target, ".copy-on-use"), 0o700))

			record := filepath.Join(target, ".copy-on-use", "binding-"+key)
			if test.symlink {
				require.NoError(t, os.WriteFile(filepath.Join(target, "record"), []byte(test.contents), 0o600))
				require.NoError(t, os.Symlink("../record", record))
			} else {
				require.NoError(t, os.WriteFile(record, []byte(test.contents), 0o600))
			}

			_, err := libvirtstorage.ReadCopyBinding(target, key)
			require.Error(t, err)
		})
	}
}

func TestCopyPublicationAbsentRejectsWitnessOnly(t *testing.T) {
	target := t.TempDir()
	name := "a.raw"

	require.NoError(t, os.Mkdir(filepath.Join(target, ".copy-on-use"), 0o700))

	witness := fmt.Sprintf("%x", sha256.Sum256([]byte(name)))
	require.NoError(t, os.WriteFile(filepath.Join(target, ".copy-on-use", witness), []byte("image"), 0o600))
	absent, err := libvirtstorage.CopyPublicationAbsent(target, name)
	require.NoError(t, err)
	require.False(t, absent, "a partial publication is not proof that rebinding is safe")
}
