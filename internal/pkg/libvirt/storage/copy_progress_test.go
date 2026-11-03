// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:testpackage // Exercise the production checked-writer seam, including partial writes.
package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/require"
)

func progressOperation(t *testing.T, data string) *CopyOperation {
	t.Helper()
	dir := t.TempDir()
	source := filepath.Join(t.TempDir(), "source")
	require.NoError(t, os.WriteFile(source, []byte(data), 0o600))

	root, err := os.OpenRoot(dir)
	require.NoError(t, err)

	op := &CopyOperation{Source: source, Expected: digest.SHA256.FromString(data).String(), root: root, staged: "stage", release: func() {}}
	t.Cleanup(op.Close)

	return op
}

func TestCopyProgressTransferAndCleanup(t *testing.T) {
	for _, data := range []string{"", strings.Repeat("x", 300000)} {
		t.Run(fmt.Sprintf("bytes=%d", len(data)), func(t *testing.T) {
			op := progressOperation(t, data)

			var events []CopyProgress

			op.SetProgressObserver(func(p CopyProgress) { events = append(events, p) })
			require.NoError(t, op.Transfer(t.Context()))
			require.NotEmpty(t, events, "transfer must emit progress")
			require.Equal(t, CopyPhaseTransfer, events[0].Phase)
			require.True(t, events[0].TotalKnown)
			require.Equal(t, uint64(len(data)), events[0].TotalBytes)

			var boundaries []CopyPhase
			for _, p := range events {
				if len(boundaries) == 0 || boundaries[len(boundaries)-1] != p.Phase {
					boundaries = append(boundaries, p.Phase)
				}

				require.False(t, p.StartedAt.IsZero())
				require.GreaterOrEqual(t, p.Elapsed.Nanoseconds(), int64(0))
			}

			require.Equal(t, []CopyPhase{CopyPhaseTransfer, CopyPhaseVerify, CopyPhaseSync}, boundaries)
			require.Equal(t, uint64(len(data)), events[len(events)-1].BytesCopied)
			op.Close()
			require.Equal(t, CopyPhaseCleanup, events[len(events)-1].Phase)
			count := len(events)

			op.Close()
			require.Len(t, events, count)
		})
	}
}

func TestCopyProgressPartialWrite(t *testing.T) {
	op := &CopyOperation{}

	var events []CopyProgress

	op.SetProgressObserver(func(p CopyProgress) { events = append(events, p) })

	failure := errors.New("disk full")
	n, err := op.write(t.Context(), copyWriteFunc(func([]byte) (int, error) { return 3, failure }), []byte("abcdef"))
	require.Equal(t, 3, n)
	require.ErrorIs(t, err, failure)
	require.NotEmpty(t, events, "partial successful bytes must be observed even on error")
	require.Equal(t, uint64(3), events[0].BytesCopied)
	require.False(t, events[0].TotalKnown)
}

func TestCopyProgressCancellationAndNil(t *testing.T) {
	op := progressOperation(t, strings.Repeat("x", 300000))
	ctx, cancel := context.WithCancel(t.Context())

	var events []CopyProgress

	op.SetProgressObserver(func(p CopyProgress) {
		events = append(events, p)
		if p.BytesCopied > 0 {
			cancel()
		}
	})
	require.ErrorIs(t, op.Transfer(ctx), context.Canceled)
	require.NotEmpty(t, events)

	for _, p := range events {
		require.Equal(t, CopyPhaseTransfer, p.Phase)
	}

	op.Close()
	require.Equal(t, CopyPhaseCleanup, events[len(events)-1].Phase)
	nilOp := progressOperation(t, "nil observer")
	require.NoError(t, nilOp.Transfer(t.Context()))
	require.True(t, nilOp.progress.StartedAt.IsZero())
}

func BenchmarkCopyWriteNilObserver(b *testing.B) {
	op := &CopyOperation{}
	data := make([]byte, 128<<10)
	ctx := context.Background()

	b.ReportAllocs()
	b.SetBytes(int64(len(data)))

	for b.Loop() {
		if _, err := op.write(ctx, io.Discard, data); err != nil {
			b.Fatal(err)
		}
	}
}

func TestCopyProgressSinglePassAndDigestFailure(t *testing.T) {
	for _, mismatch := range []bool{false, true} {
		t.Run(fmt.Sprint(mismatch), func(t *testing.T) {
			data := strings.Repeat("payload", 50000)

			op := progressOperation(t, data)
			if mismatch {
				op.Expected = digest.SHA256.FromString("different").String()
			}

			input := strings.NewReader(data)

			var readBytes int

			reader := copyReadFunc(func(p []byte) (int, error) {
				n, err := input.Read(p)
				readBytes += n

				return n, err
			})

			var phases []CopyPhase

			op.SetProgressObserver(func(p CopyProgress) { phases = append(phases, p.Phase) })
			err := op.writeCopyContext(t.Context(), reader)
			require.Equal(t, len(data), readBytes)
			require.Zero(t, input.Len())
			require.Equal(t, uint64(len(data)), op.progress.BytesCopied)
			require.False(t, op.progress.TotalKnown)

			if mismatch {
				require.ErrorContains(t, err, "digest mismatch")
				require.Equal(t, CopyPhaseVerify, phases[len(phases)-1])
				require.NotContains(t, phases, CopyPhaseSync)
			} else {
				require.NoError(t, err)
				require.Equal(t, CopyPhaseSync, phases[len(phases)-1])
			}
		})
	}
}

func TestCopyProgressNilObserverAllocations(t *testing.T) {
	op := &CopyOperation{}
	data := make([]byte, 128<<10)
	ctx := context.Background()

	require.Zero(t, testing.AllocsPerRun(100, func() {
		_, err := op.write(ctx, io.Discard, data)
		if err != nil {
			panic(err)
		}
	}))
	require.True(t, op.progress.StartedAt.IsZero())
}
