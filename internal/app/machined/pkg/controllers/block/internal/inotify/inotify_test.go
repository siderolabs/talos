// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package inotify_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block/internal/inotify"
)

func assertEvent(t *testing.T, watchCh <-chan string, errCh <-chan error, expected string) {
	t.Helper()

	select {
	case path := <-watchCh:
		require.Equal(t, expected, path)
	case err := <-errCh:
		require.FailNow(t, "unexpected error", "%s", err)
	case <-time.After(time.Second):
		require.FailNow(t, "timeout")
	}
}

func assertNoEvent(t *testing.T, watchCh <-chan string, errCh <-chan error) {
	t.Helper()

	select {
	case path := <-watchCh:
		require.FailNow(t, "unexpected path", "%s", path)
	case err := <-errCh:
		require.FailNow(t, "unexpected error", "%s", err)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestWatcherCloseWrite(t *testing.T) {
	watcher, err := inotify.NewWatcher()
	require.NoError(t, err)

	d := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(d, "file1"), []byte("test1"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(d, "file2"), []byte("test2"), 0o644))

	require.NoError(t, watcher.Add(filepath.Join(d, "file1"), unix.IN_CLOSE_WRITE))

	watchCh, errCh := watcher.Run()

	require.NoError(t, watcher.Add(filepath.Join(d, "file2"), unix.IN_CLOSE_WRITE))

	assertNoEvent(t, watchCh, errCh)

	// open file1 for writing, should get inotify event
	f1, err := os.OpenFile(filepath.Join(d, "file1"), os.O_WRONLY, 0)
	require.NoError(t, err)

	require.NoError(t, f1.Close())

	assertEvent(t, watchCh, errCh, filepath.Join(d, "file1"))

	// open file2 for reading, should not get inotify event
	f2, err := os.OpenFile(filepath.Join(d, "file2"), os.O_RDONLY, 0)
	require.NoError(t, err)

	require.NoError(t, f2.Close())

	assertNoEvent(t, watchCh, errCh)

	// remove file2
	require.NoError(t, os.Remove(filepath.Join(d, "file2")))

	assertNoEvent(t, watchCh, errCh)

	require.NoError(t, watcher.Remove(filepath.Join(d, "file2")))

	require.NoError(t, watcher.Close())
}

func TestWatcherDirectory(t *testing.T) {
	watcher, err := inotify.NewWatcher()
	require.NoError(t, err)

	d := t.TempDir()

	require.NoError(t, os.Mkdir(filepath.Join(d, "dir1"), 0o755))

	require.NoError(t, os.Symlink("a1", filepath.Join(d, "dir1", "link1")))
	require.NoError(t, os.Symlink("a2", filepath.Join(d, "dir1", "link2")))

	require.NoError(t, watcher.Add(d, unix.IN_CREATE|unix.IN_DELETE|unix.IN_MOVE))
	require.NoError(t, watcher.Add(filepath.Join(d, "dir1"), unix.IN_CREATE|unix.IN_DELETE|unix.IN_MOVE))

	watchCh, errCh := watcher.Run()

	assertNoEvent(t, watchCh, errCh)

	require.NoError(t, os.Remove(filepath.Join(d, "dir1", "link1")))

	assertEvent(t, watchCh, errCh, filepath.Join(d, "dir1", "link1"))

	require.NoError(t, os.Mkdir(filepath.Join(d, "dir2"), 0o755))

	assertEvent(t, watchCh, errCh, filepath.Join(d, "dir2"))

	require.NoError(t, os.Symlink("a3", filepath.Join(d, "dir1", "#.link3")))

	assertEvent(t, watchCh, errCh, filepath.Join(d, "dir1", "#.link3"))

	require.NoError(t, os.Rename(filepath.Join(d, "dir1", "#.link3"), filepath.Join(d, "dir1", "link3")))

	assertEvent(t, watchCh, errCh, filepath.Join(d, "dir1", "#.link3"))
	assertEvent(t, watchCh, errCh, filepath.Join(d, "dir1", "link3"))

	// no more events
	assertNoEvent(t, watchCh, errCh)

	require.NoError(t, watcher.Remove(d))

	require.NoError(t, watcher.Close())
}
