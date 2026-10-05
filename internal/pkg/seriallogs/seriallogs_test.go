// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package seriallogs_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/talos/internal/pkg/seriallogs"
)

func TestFollowRotation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "serial.log")
	require.NoError(t, os.WriteFile(path, []byte("initial\n"), 0o600))

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	chunks := make(chan string, 8)

	done := make(chan error, 1)
	go func() {
		done <- seriallogs.Stream(ctx, path, -1, true, func(data []byte) error {
			select {
			case chunks <- string(data):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
	}()

	receive := func(want string) {
		t.Helper()

		select {
		case got := <-chunks:
			require.Equal(t, want, got)
		case err := <-done:
			t.Fatalf("stream stopped before %q: %v", want, err)
		case <-ctx.Done():
			t.Fatalf("missing %q: %v", want, ctx.Err())
		}
	}

	receive("initial\n")

	writer, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, writer.Close()) })
	require.NoError(t, os.Rename(path, path+".0"))

	_, err = writer.WriteString("old-inode\n")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, []byte("replacement"), 0o600))
	receive("old-inode\n")
	receive("replacement")
	cancel()

	select {
	case err := <-done:
		require.Error(t, err)
	case <-time.After(time.Second):
		t.Fatal("cancellation did not join reader")
	}

	select {
	case extra := <-chunks:
		t.Fatalf("duplicate output %q", extra)
	default:
	}
}

func TestConcurrentHistoryReaders(t *testing.T) {
	path := filepath.Join(t.TempDir(), "serial.log")
	require.NoError(t, os.WriteFile(path, []byte("prompt>"), 0o600))
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	outputs := make([]chan string, 2)
	results := make([]chan error, 2)

	for index := range outputs {
		outputs[index] = make(chan string, 2)
		results[index] = make(chan error, 1)

		go func() {
			results[index] <- seriallogs.Stream(ctx, path, 1, true, func(data []byte) error {
				select {
				case outputs[index] <- string(data):
					return nil
				case <-ctx.Done():
					return status.FromContextError(ctx.Err()).Err()
				}
			})
		}()
	}

	t.Cleanup(func() {
		cancel()

		for _, result := range results {
			require.Equal(t, codes.Canceled, status.Code(<-result))
		}
	})

	for _, output := range outputs {
		requireHistoryOutput(t, ctx, output, "prompt>")
	}

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	require.NoError(t, err)
	_, err = file.WriteString("\nnext")
	require.NoError(t, err)
	require.NoError(t, file.Close())

	for _, output := range outputs {
		requireHistoryOutput(t, ctx, output, "\nnext")
	}
}

func requireHistoryOutput(t *testing.T, ctx context.Context, output <-chan string, expected string) {
	t.Helper()

	select {
	case data := <-output:
		require.Equal(t, expected, data)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
}

func TestEmptyAndMultiBlockTail(t *testing.T) {
	path := filepath.Join(t.TempDir(), "serial.log")
	require.NoError(t, os.WriteFile(path, nil, 0o600))
	require.NoError(t, seriallogs.Stream(t.Context(), path, 1, false, func([]byte) error {
		t.Error("empty source must not emit a chunk")

		return nil
	}))
	require.NoError(t, os.WriteFile(path, []byte("first\n"+strings.Repeat("x", 65536)+"\nlast\n"), 0o600))

	var output strings.Builder

	require.NoError(t, seriallogs.Stream(t.Context(), path, 2, false, func(data []byte) error {
		_, err := output.Write(data)

		return err
	}))
	require.Equal(t, strings.Repeat("x", 65536)+"\nlast\n", output.String())
}

func TestReadOnlyErrorsAndBounds(t *testing.T) {
	path := filepath.Join(t.TempDir(), "serial.log")
	send := func([]byte) error { return nil }

	err := seriallogs.Stream(t.Context(), path, -1, false, send)
	require.Equal(t, codes.NotFound, status.Code(err))
	_, err = os.Stat(path)
	require.ErrorIs(t, err, os.ErrNotExist)

	require.NoError(t, os.Symlink("/etc/passwd", path))
	err = seriallogs.Stream(t.Context(), path, -1, false, send)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	require.NoError(t, os.Remove(path))
	require.NoError(t, unix.Mkfifo(path, 0o600))
	err = seriallogs.Stream(t.Context(), path, -1, false, send)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	require.NoError(t, os.Remove(path))
	require.NoError(t, os.WriteFile(path, bytes.Repeat([]byte("x"), 9*1024*1024), 0o600))
	err = seriallogs.Stream(t.Context(), path, 1, false, send)
	require.Equal(t, codes.ResourceExhausted, status.Code(err))

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err = seriallogs.Stream(ctx, path, -1, true, send)
	require.Equal(t, codes.Canceled, status.Code(err))

	if os.Geteuid() != 0 {
		require.NoError(t, os.Chmod(path, 0))
		err = seriallogs.Stream(t.Context(), path, -1, false, send)
		require.Equal(t, codes.PermissionDenied, status.Code(err))

		info, statErr := os.Stat(path)
		require.NoError(t, statErr)
		require.Zero(t, info.Mode().Perm())
	}
}

func TestFiniteSnapshotBoundedChunks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "serial.log")
	original := bytes.Repeat([]byte("x"), 128*1024)
	require.NoError(t, os.WriteFile(path, original, 0o600))
	writer, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)

	require.NoError(t, err)
	defer func() { require.NoError(t, writer.Close()) }()

	var out bytes.Buffer

	err = seriallogs.Stream(t.Context(), path, -1, false, func(data []byte) error {
		require.LessOrEqual(t, len(data), 32*1024)

		_, writeErr := writer.WriteString("new bytes")
		require.NoError(t, writeErr)
		_, writeErr = out.Write(data)

		return writeErr
	})
	require.NoError(t, err)
	require.Equal(t, original, out.Bytes())
}

func TestFollowDeletionAndTruncation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "serial.log")
	require.NoError(t, os.WriteFile(path, []byte("long-initial-value"), 0o600))

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	chunks := make(chan string, 4)
	done := make(chan error, 1)

	go func() {
		done <- seriallogs.Stream(ctx, path, -1, true, func(data []byte) error {
			select {
			case chunks <- string(data):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
	}()

	receive := func(want string) {
		t.Helper()

		select {
		case got := <-chunks:
			require.Equal(t, want, got)
		case err := <-done:
			t.Fatalf("stopped before %q: %v", want, err)
		case <-ctx.Done():
			t.Fatalf("missing %q", want)
		}
	}

	receive("long-initial-value")
	require.NoError(t, os.WriteFile(path, []byte("short"), 0o600))
	receive("short")
	require.NoError(t, os.Remove(path))
	// Leave a genuine missing-path interval; the reader must neither exit nor
	// busy-loop, and cancellation remains available while waiting for creation.
	time.Sleep(250 * time.Millisecond)
	require.NoError(t, os.WriteFile(path, []byte("recreated"), 0o600))
	receive("recreated")
	cancel()

	select {
	case err := <-done:
		require.Equal(t, codes.Canceled, status.Code(err))
	case <-time.After(time.Second):
		t.Fatal("reader did not terminate")
	}
}

func TestSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "serial.log")
	require.NoError(t, os.WriteFile(path, []byte("first\nsecond\nlast"), 0o600))

	for _, test := range []struct {
		name string
		tail int32
		want string
	}{
		{
			name: "all",
			tail: -1,
			want: "first\nsecond\nlast",
		},
		{
			name: "tail",
			tail: 2,
			want: "second\nlast",
		},
		{
			name: "none",
			tail: 0,
			want: "",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			var out bytes.Buffer

			err := seriallogs.Stream(t.Context(), path, test.tail, false, func(data []byte) error {
				_, err := out.Write(data)

				return err
			})
			require.NoError(t, err)
			require.Equal(t, test.want, out.String())
		})
	}
}
