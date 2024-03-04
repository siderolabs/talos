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

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block/internal/inotify"
)

func TestWatcher(t *testing.T) {
	watcher, err := inotify.NewWatcher()
	require.NoError(t, err)

	d := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(d, "file1"), []byte("test1"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(d, "file2"), []byte("test2"), 0o644))

	require.NoError(t, watcher.Add(filepath.Join(d, "file1")))

	watchCh, errCh := watcher.Run()

	require.NoError(t, watcher.Add(filepath.Join(d, "file2")))

	select {
	case path := <-watchCh:
		require.FailNow(t, "unexpected path", "%s", path)
	case err = <-errCh:
		require.FailNow(t, "unexpected error", "%s", err)
	case <-time.After(100 * time.Millisecond):
	}

	// open file1 for writing, should get inotify event
	f1, err := os.OpenFile(filepath.Join(d, "file1"), os.O_WRONLY, 0)
	require.NoError(t, err)

	require.NoError(t, f1.Close())

	select {
	case path := <-watchCh:
		require.Equal(t, filepath.Join(d, "file1"), path)
	case err = <-errCh:
		require.FailNow(t, "unexpected error", "%s", err)
	case <-time.After(time.Second):
		require.FailNow(t, "timeout")
	}

	// open file2 for reading, should not get inotify event
	f2, err := os.OpenFile(filepath.Join(d, "file2"), os.O_RDONLY, 0)
	require.NoError(t, err)

	require.NoError(t, f2.Close())

	select {
	case path := <-watchCh:
		require.FailNow(t, "unexpected path", "%s", path)
	case err = <-errCh:
		require.FailNow(t, "unexpected error", "%s", err)
	case <-time.After(100 * time.Millisecond):
	}

	require.NoError(t, watcher.Remove(filepath.Join(d, "file2")))

	require.NoError(t, watcher.Close())
}
