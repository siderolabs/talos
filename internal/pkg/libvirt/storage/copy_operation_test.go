// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	libvirtstorage "github.com/siderolabs/talos/internal/pkg/libvirt/storage"
	"github.com/siderolabs/talos/internal/pkg/libvirt/storage/copyowner"
)

// runLimitedChild reruns test in a child process with writes capped at limit
// bytes, so the file size limit never touches the parent or other tests.
func runLimitedChild(t *testing.T, test string, limit uint64) bool {
	t.Helper()

	if os.Getenv("TALOS_FSIZE_TEST_CHILD") != "1" {
		child := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^"+test+"$")

		child.Env = append(os.Environ(), "TALOS_FSIZE_TEST_CHILD=1", fmt.Sprintf("TALOS_FSIZE_LIMIT=%d", limit))
		output, err := child.CombinedOutput()
		require.NoError(t, err, "%s", output)

		return false
	}

	signal.Ignore(syscall.SIGXFSZ)

	return true
}

func limitFileSize(t *testing.T) {
	t.Helper()

	var limit unix.Rlimit

	require.NoError(t, unix.Getrlimit(unix.RLIMIT_FSIZE, &limit))

	var size uint64

	_, err := fmt.Sscan(os.Getenv("TALOS_FSIZE_LIMIT"), &size)
	require.NoError(t, err)
	require.NoError(t, unix.Setrlimit(unix.RLIMIT_FSIZE, &unix.Rlimit{Cur: size, Max: limit.Max}))

	t.Cleanup(func() {
		require.NoError(t, unix.Setrlimit(unix.RLIMIT_FSIZE, &limit))
	})
}

func progressOperation(t *testing.T, data string) *libvirtstorage.CopyOperation {
	t.Helper()

	return progressOperationWithDigest(t, data, digest.SHA256.FromString(data).String())
}

func progressOperationWithDigest(t *testing.T, data, expected string) *libvirtstorage.CopyOperation {
	t.Helper()

	_, client, pool := newVolumeFixture(t)
	source := filepath.Join(t.TempDir(), "source")

	require.NoError(t, os.WriteFile(source, []byte(data), 0o600))

	op, err := client.PrepareCopy(pool, "guest.iso", source, expected)
	require.NoError(t, err)
	t.Cleanup(op.Close)

	return op
}

func TestCopyProgressTransferAndCleanup(t *testing.T) {
	for _, data := range []string{"", strings.Repeat("x", 300000)} {
		t.Run(fmt.Sprintf("bytes=%d", len(data)), func(t *testing.T) {
			op := progressOperation(t, data)

			var events []libvirtstorage.CopyProgress

			op.SetProgressObserver(func(p libvirtstorage.CopyProgress) { events = append(events, p) })
			require.NoError(t, op.Transfer(t.Context()))
			require.NotEmpty(t, events, "transfer must emit progress")
			require.Equal(t, libvirtstorage.CopyPhaseTransfer, events[0].Phase)
			require.True(t, events[0].TotalKnown)
			require.Equal(t, uint64(len(data)), events[0].TotalBytes)

			var boundaries []libvirtstorage.CopyPhase
			for _, p := range events {
				if len(boundaries) == 0 || boundaries[len(boundaries)-1] != p.Phase {
					boundaries = append(boundaries, p.Phase)
				}

				require.False(t, p.StartedAt.IsZero())
				require.GreaterOrEqual(t, p.Elapsed.Nanoseconds(), int64(0))
			}

			require.Equal(t, []libvirtstorage.CopyPhase{libvirtstorage.CopyPhaseTransfer, libvirtstorage.CopyPhaseVerify, libvirtstorage.CopyPhaseSync}, boundaries)
			require.Equal(t, uint64(len(data)), events[len(events)-1].BytesCopied)
			op.Close()
			require.Equal(t, libvirtstorage.CopyPhaseCleanup, events[len(events)-1].Phase)
			count := len(events)

			op.Close()
			require.Len(t, events, count)
		})
	}
}

func TestCopyProgressPartialWrite(t *testing.T) {
	if !runLimitedChild(t, "TestCopyProgressPartialWrite", 200000) {
		return
	}

	// The source is written before the limit applies; only the stage is capped.
	op := progressOperation(t, strings.Repeat("x", 300000))

	var events []libvirtstorage.CopyProgress

	op.SetProgressObserver(func(p libvirtstorage.CopyProgress) { events = append(events, p) })
	limitFileSize(t)

	require.ErrorIs(t, op.Transfer(t.Context()), syscall.EFBIG)
	require.Equal(t, uint64(200000), events[len(events)-1].BytesCopied, "partial successful bytes must be observed even on error")
}

func TestCopyProgressCancellation(t *testing.T) {
	op := progressOperation(t, strings.Repeat("x", 300000))
	ctx, cancel := context.WithCancel(t.Context())

	var events []libvirtstorage.CopyProgress

	op.SetProgressObserver(func(p libvirtstorage.CopyProgress) {
		events = append(events, p)
		if p.BytesCopied > 0 {
			cancel()
		}
	})
	require.ErrorIs(t, op.Transfer(ctx), context.Canceled)
	require.NotEmpty(t, events)

	for _, p := range events {
		require.Equal(t, libvirtstorage.CopyPhaseTransfer, p.Phase)
	}

	op.Close()
	require.Equal(t, libvirtstorage.CopyPhaseCleanup, events[len(events)-1].Phase)

	require.NoError(t, progressOperation(t, "nil observer").Transfer(t.Context()))
}

func TestCopyProgressDigestFailure(t *testing.T) {
	for _, mismatch := range []bool{false, true} {
		t.Run(fmt.Sprint(mismatch), func(t *testing.T) {
			data := strings.Repeat("payload", 50000)

			expected := digest.SHA256.FromString(data).String()
			if mismatch {
				expected = digest.SHA256.FromString("different").String()
			}

			op := progressOperationWithDigest(t, data, expected)

			var events []libvirtstorage.CopyProgress

			op.SetProgressObserver(func(p libvirtstorage.CopyProgress) { events = append(events, p) })
			err := op.Transfer(t.Context())
			last := events[len(events)-1]

			require.Equal(t, uint64(len(data)), last.BytesCopied, "the digest is computed inline in one pass")

			if mismatch {
				require.ErrorContains(t, err, "digest mismatch")
				require.Equal(t, libvirtstorage.CopyPhaseVerify, last.Phase)
			} else {
				require.NoError(t, err)
				require.Equal(t, libvirtstorage.CopyPhaseSync, last.Phase)
			}
		})
	}
}

func TestCopyBindingKeyRejectsInvalidAndLateAssignment(t *testing.T) {
	op := progressOperation(t, "payload")

	require.ErrorContains(t, op.SetBindingKey("bad"), "invalid copy binding key")
	require.NoError(t, op.SetBindingKey(strings.Repeat("a", 64)))
	require.NoError(t, op.Transfer(t.Context()))
	require.ErrorContains(t, op.SetBindingKey(strings.Repeat("b", 64)), "before transfer")
}

// sharedTargetClient returns another client for the pool behind rpc. Separate
// fixture state keeps concurrent sessions free of shared fake bookkeeping.
func sharedTargetClient(t *testing.T, r *rpc) libvirtstorage.VolumeClient {
	t.Helper()

	other, client, _ := newVolumeFixture(t)
	other.target = r.target

	return client
}

func preparedCopy(t *testing.T, client libvirtstorage.VolumeClient, pool libvirtstorage.Pool, name, data, key string) *libvirtstorage.CopyOperation {
	t.Helper()

	source := filepath.Join(t.TempDir(), "source")
	require.NoError(t, os.WriteFile(source, []byte(data), 0o600))

	op, err := client.PrepareCopy(pool, name, source, "")
	require.NoError(t, err)
	t.Cleanup(op.Close)
	require.NoError(t, op.SetBindingKey(key))
	require.NoError(t, op.Transfer(t.Context()))

	return op
}

func TestConcurrentCopyPublicationKeepsWinningOwner(t *testing.T) {
	r, _, pool := newVolumeFixture(t)
	name, key := "concurrent.iso", strings.Repeat("a", 64)
	start := make(chan struct{})
	results := make(chan error, 2)

	for _, data := range []string{"first", "second"} {
		client := sharedTargetClient(t, r)
		op := preparedCopy(t, client, pool, name, data, key)

		go func() {
			<-start

			_, err := client.FinishCopy(t.Context(), pool, op, func() error { return nil })
			results <- err
		}()
	}

	close(start)

	successes := 0

	for range 2 {
		if <-results == nil {
			successes++
		}
	}

	require.Equal(t, 1, successes)

	root, err := os.OpenRoot(r.target)
	require.NoError(t, err)

	defer root.Close() //nolint:errcheck

	info, err := root.Lstat(name)
	require.NoError(t, err)
	require.True(t, copyowner.OwnerMatches(root, key, name, info), "the owner record names the published inode")
}

func TestCopyPublicationSerializesFinalValidation(t *testing.T) {
	r, _, pool := newVolumeFixture(t)
	first, second := sharedTargetClient(t, r), sharedTargetClient(t, r)
	firstOp := preparedCopy(t, first, pool, "first.iso", "first", strings.Repeat("a", 64))
	secondOp := preparedCopy(t, second, pool, "second.iso", "second", strings.Repeat("b", 64))

	var secondValidated atomic.Bool

	secondDone := make(chan error, 1)

	_, err := first.FinishCopy(t.Context(), pool, firstOp, func() error {
		go func() {
			_, finishErr := second.FinishCopy(t.Context(), pool, secondOp, func() error {
				secondValidated.Store(true)

				return nil
			})
			secondDone <- finishErr
		}()

		time.Sleep(100 * time.Millisecond)
		require.False(t, secondValidated.Load(), "final validation must run inside publication serialization")

		return nil
	})
	require.NoError(t, err)
	require.NoError(t, <-secondDone)
	require.True(t, secondValidated.Load())
}

func TestOwnedRawGrowthRejectsFileSizeLimit(t *testing.T) {
	if !runLimitedChild(t, "TestOwnedRawGrowthRejectsFileSizeLimit", 8) {
		return
	}

	r, client, pool := newVolumeFixture(t)
	name := "guest.raw"

	require.NoError(t, os.WriteFile(filepath.Join(r.target, name), []byte("data"), 0o600))
	require.NoError(t, os.Mkdir(filepath.Join(r.target, copyowner.Dir), 0o700))

	root, err := os.OpenRoot(r.target)
	require.NoError(t, err)

	defer root.Close() //nolint:errcheck

	owner, err := copyowner.ClaimImageOwner(root, name)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(r.target, copyowner.Witness(name, true)), []byte(copyowner.ImageMarker("raw", owner)), 0o600))

	r.known[name] = struct{}{}

	limitFileSize(t)

	outcome, err := client.ResizeVolumeOperation(pool, name, 16)
	require.ErrorIs(t, err, syscall.EFBIG)
	require.Equal(t, libvirtstorage.Finished, outcome, "a definitive local rejection must not leave an unknown-mutation fence")

	info, err := root.Stat(name)
	require.NoError(t, err)
	require.EqualValues(t, 4, info.Size())

	outcome, err = client.ResizeVolumeOperation(pool, name, 8)
	require.NoError(t, err)
	require.Equal(t, libvirtstorage.Finished, outcome)

	data, err := os.ReadFile(filepath.Join(r.target, name))
	require.NoError(t, err)
	require.Len(t, data, 8)
	require.Equal(t, "data", string(data[:4]))
}
