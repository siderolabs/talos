// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package filehash_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime/internal/filehash"
)

func assertEvent(t *testing.T, eventCh <-chan string, errCh <-chan error, expected string) {
	t.Helper()

	select {
	case path := <-eventCh:
		require.Equal(t, expected, path)
	case err := <-errCh:
		require.FailNow(t, "unexpected error: %v", err)
	case <-time.After(2 * time.Second):
		require.FailNow(t, "timeout waiting for event")
	}
}

func assertNoEvent(t *testing.T, eventCh <-chan string, errCh <-chan error) {
	t.Helper()

	select {
	case path := <-eventCh:
		require.FailNow(t, "unexpected event: %v", path)
	case err := <-errCh:
		require.FailNow(t, "unexpected error: %v", err)
	case <-time.After(500 * time.Millisecond):
	}
}

func TestWatcherDetectsChange(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	file := filepath.Join(dir, "testfile")
	require.NoError(t, os.WriteFile(file, []byte("foo"), 0o644))

	watcher, err := filehash.NewWatcher(file)
	require.NoError(t, err)

	defer watcher.Close() //nolint:errcheck

	eventCh, errCh := watcher.Run()

	// Initial change should be detected
	assertEvent(t, eventCh, errCh, file)

	// No change, so no event
	assertNoEvent(t, eventCh, errCh)

	// Modify file
	require.NoError(t, os.WriteFile(file, []byte("bar"), 0o644))
	assertEvent(t, eventCh, errCh, file)

	// No change, so no event
	assertNoEvent(t, eventCh, errCh)
}

func TestWatcherHandlesMissingFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	file := filepath.Join(dir, "missingfile")

	watcher, err := filehash.NewWatcher(file)
	require.NoError(t, err)

	defer watcher.Close() //nolint:errcheck

	eventCh, errCh := watcher.Run()

	// Should get an error because file does not exist
	select {
	case <-eventCh:
		require.FailNow(t, "unexpected event for missing file")
	case err := <-errCh:
		require.Error(t, err)
	case <-time.After(2 * time.Second):
		require.FailNow(t, "timeout waiting for error")
	}
}

func TestWatcherClose(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	file := filepath.Join(dir, "testfile")
	require.NoError(t, os.WriteFile(file, []byte("foo"), 0o644))

	watcher, err := filehash.NewWatcher(file)
	require.NoError(t, err)

	eventCh, errCh := watcher.Run()
	watcher.Close() //nolint:errcheck

	// Channels should be closed
	_, ok1 := <-eventCh
	_, ok2 := <-errCh

	require.False(t, ok1)
	require.False(t, ok2)
}
