// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hostns_test

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/containerd/containerd/v2/core/mount"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/pkg/hostns"
)

// TestSetupTeardownNoLeak exercises the full mount setup and, crucially, verifies the
// teardown removes every scratch mount without disturbing the host's own mounts.
//
// It runs inside a fresh private mount namespace so the test's mounts — and any
// teardown bug — cannot leak into the CI host. LockOSThread is intentionally never
// unlocked: the Go runtime terminates the (namespace-tainted) thread when the test
// goroutine exits, so other tests keep the original namespace.
func TestSetupTeardownNoLeak(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("requires root for mount operations")
	}

	runtime.LockOSThread()

	require.NoError(t, unix.Unshare(unix.CLONE_NEWNS))
	require.NoError(t, unix.Mount("", "/", "", unix.MS_REC|unix.MS_PRIVATE, ""))

	// Setup recursively bind-mounts /sys into the debug root and makes that bind
	// shared. Keep the source shared too, matching Talos, so an unsafe teardown of
	// the bind propagates back and unmounts the host source.
	require.NoError(t, unix.Mount("", "/sys", "", unix.MS_REC|unix.MS_SHARED, ""))

	tmp := t.TempDir()

	// Fake image snapshot: overlayfs needs at least two lower layers (a real Nix image
	// has ~70). The top layer carries /nix/bin/tool.
	imgUpperLayer := filepath.Join(tmp, "img1")
	imgLowerLayer := filepath.Join(tmp, "img0")
	require.NoError(t, os.MkdirAll(filepath.Join(imgUpperLayer, "nix", "bin"), 0o755))
	require.NoError(t, os.MkdirAll(imgLowerLayer, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(imgUpperLayer, "nix", "bin", "tool"), []byte("x"), 0o755))

	snap := []mount.Mount{{Type: "overlay", Source: "overlay", Options: []string{"lowerdir=" + imgUpperLayer + ":" + imgLowerLayer}}}

	baseDir := filepath.Join(tmp, "base")
	varBase := filepath.Join(tmp, "var")

	baseline := len(mountTargets(t))

	merged, teardown, err := hostns.Setup(snap, baseDir, varBase)
	require.NoError(t, err)

	torndown := false

	t.Cleanup(func() {
		if !torndown {
			_ = teardown() //nolint:errcheck
		}
	})

	// The image /nix is overlaid into the merged root and reachable.
	_, err = os.Stat(filepath.Join(merged, "nix", "bin", "tool"))
	assert.NoError(t, err, "image /nix overlaid into merged")

	// The overlay root and every host bind are mounted.
	assert.True(t, isMounted(t, merged), "overlay root mounted")

	// merged must be unbindable so the recursive /var bind can't re-include it (which
	// would nest merged into itself — an exploding mount recursion).
	assert.True(t, isUnbindable(t, merged), "overlay root is unbindable")

	for _, dir := range hostns.HostBinds {
		if _, statErr := os.Stat("/" + dir); statErr != nil {
			continue // host lacks this dir (e.g. /system off-Talos); Setup skips it
		}

		assert.True(t, isMounted(t, filepath.Join(merged, dir)), "%s bound into merged", dir)
	}

	// A session creates a scratch mount inside the chroot root.
	require.NoError(t, os.MkdirAll(filepath.Join(merged, "mnt", "x"), 0o755))
	require.NoError(t, unix.Mount("sess", filepath.Join(merged, "mnt", "x"), "tmpfs", 0, ""))

	// Teardown must remove ALL our mounts and leave the host's mounts intact.
	require.NoError(t, teardown())

	torndown = true

	assert.Zero(t, countUnder(t, baseDir), "no scratch mounts remain under baseDir")
	assert.True(t, isMounted(t, "/proc"), "host /proc survived teardown")
	assert.True(t, isMounted(t, "/sys"), "host /sys survived teardown")
	assert.Equal(t, baseline, len(mountTargets(t)), "mount table returned to baseline — nothing leaked")
}

// TestSetupRollbackOnError verifies a failed Setup leaves no mounts behind.
func TestSetupRollbackOnError(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("requires root for mount operations")
	}

	runtime.LockOSThread()

	require.NoError(t, unix.Unshare(unix.CLONE_NEWNS))
	require.NoError(t, unix.Mount("", "/", "", unix.MS_REC|unix.MS_PRIVATE, ""))

	tmp := t.TempDir()
	baseDir := filepath.Join(tmp, "base")
	varBase := filepath.Join(tmp, "var")

	baseline := len(mountTargets(t))

	// A snapshot whose lower dir does not exist makes the image overlay mount fail.
	snap := []mount.Mount{{Type: "overlay", Source: "overlay", Options: []string{"lowerdir=" + filepath.Join(tmp, "does-not-exist")}}}

	_, _, err := hostns.Setup(snap, baseDir, varBase)
	require.Error(t, err)

	assert.Equal(t, baseline, len(mountTargets(t)), "failed Setup left no mounts behind")
}

func mountTargets(t *testing.T) []string {
	t.Helper()

	data, err := os.ReadFile("/proc/self/mountinfo")
	require.NoError(t, err)

	var targets []string

	for line := range strings.SplitSeq(strings.TrimSpace(string(data)), "\n") {
		if fields := strings.Fields(line); len(fields) >= 5 {
			targets = append(targets, fields[4]) // field 5: mount point
		}
	}

	return targets
}

func isMounted(t *testing.T, target string) bool {
	t.Helper()

	return slices.Contains(mountTargets(t), target)
}

// isUnbindable reports whether the mount at target carries the "unbindable"
// propagation type (per /proc/self/mountinfo optional fields).
func isUnbindable(t *testing.T, target string) bool {
	t.Helper()

	data, err := os.ReadFile("/proc/self/mountinfo")
	require.NoError(t, err)

	for line := range strings.SplitSeq(strings.TrimSpace(string(data)), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 7 || fields[4] != target {
			continue
		}

		// Optional propagation fields run from index 6 up to the "-" separator.
		for _, f := range fields[6:] {
			if f == "-" {
				break
			}

			if f == "unbindable" {
				return true
			}
		}
	}

	return false
}

func countUnder(t *testing.T, prefix string) int {
	t.Helper()

	n := 0

	for _, m := range mountTargets(t) {
		if strings.HasPrefix(m, prefix) {
			n++
		}
	}

	return n
}
