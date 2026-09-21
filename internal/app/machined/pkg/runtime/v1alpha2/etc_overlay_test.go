// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build linux

package v1alpha2_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha2"
	"github.com/siderolabs/talos/pkg/xfs"
	"github.com/siderolabs/talos/pkg/xfs/fsopen"
)

// TestEtcOverlay runs the real setupEtcOverlay against a throwaway /etc.
func TestEtcOverlay(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("requires root for mount operations")
	}

	// A static file shipped in the rootfs (the squashfs lower) is visible through the overlay.
	t.Run("static rootfs file is shown as-is", func(t *testing.T) {
		etc, _ := newEtcOverlay(t, map[string]string{"os-release": "STATIC"})

		assert.Equal(t, "STATIC", readEtc(t, etc, "os-release"))
	})

	// A file written through the overlay (into the upper) shadows the static lower file of the
	// same name — how a Talos default (e.g. nfsmount.conf) is overridden at runtime.
	t.Run("file written through the overlay overrides the static lower", func(t *testing.T) {
		etc, etcRoot := newEtcOverlay(t, map[string]string{"nfsmount.conf": "STATIC"})

		require.NoError(t, xfs.WriteFile(etcRoot, "nfsmount.conf", []byte("OVERRIDE"), 0o644))

		assert.Equal(t, "OVERRIDE", readEtc(t, etc, "nfsmount.conf"))
	})

	// A file created in a subdir through the overlay surfaces even though the directory was
	// observed first — writes go through overlayfs, so there is no merged-dir freeze (this is the
	// CRI registry hosts / cri.toml case).
	t.Run("file created in a subdir surfaces after an early dir read", func(t *testing.T) {
		etc, etcRoot := newEtcOverlay(t, map[string]string{"cri/conf.d/00-base.part": "BASE"})

		// observe the directory before the file exists (mirrors containerd's early read).
		_, err := os.Stat(filepath.Join(etc, "cri/conf.d"))
		require.NoError(t, err)

		require.NoError(t, xfs.MkdirAll(etcRoot, "cri/conf.d/hosts/docker.io", 0o755))
		require.NoError(t, xfs.WriteFile(etcRoot, "cri/conf.d/hosts/docker.io/hosts.toml", []byte("HOSTS"), 0o644))

		assert.Equal(t, "HOSTS", readEtc(t, etc, "cri/conf.d/hosts/docker.io/hosts.toml"))
		// the static sibling stays visible alongside the generated tree.
		assert.Equal(t, "BASE", readEtc(t, etc, "cri/conf.d/00-base.part"))
	})

	// A brand-new top-level file written through the overlay surfaces immediately and subsequent
	// in-place content updates are reflected (resolv.conf / hosts churn).
	t.Run("new file surfaces and can be updated in place", func(t *testing.T) {
		etc, etcRoot := newEtcOverlay(t, nil)

		require.NoError(t, xfs.WriteFile(etcRoot, "resolv.conf", []byte("nameserver 1.1.1.1"), 0o644))
		assert.Equal(t, "nameserver 1.1.1.1", readEtc(t, etc, "resolv.conf"))

		require.NoError(t, xfs.WriteFile(etcRoot, "resolv.conf", []byte("nameserver 8.8.8.8"), 0o644))
		assert.Equal(t, "nameserver 8.8.8.8", readEtc(t, etc, "resolv.conf"))
	})

	// Regression: under Docker the runtime bind-mounts /etc/hostname (and /etc/resolv.conf) as
	// file submounts of /etc. OPEN_TREE_CLONE without AT_RECURSIVE drops them from the lower, and
	// overlayfs does not cross submounts inside a lower either, so without the container-mode seed
	// the overlay exposes the empty rootfs file instead of the container name — leaving the
	// container platform, which reads /etc/hostname, to fall back to a generated hostname.
	t.Run("container bind-mounted files survive the overlay compose", func(t *testing.T) {
		etc := newEtcOverlayWithBinds(t, map[string]string{
			"hostname":    "test-docker-controlplane-1",
			"resolv.conf": "nameserver 172.20.0.1",
		})

		assert.Equal(t, "test-docker-controlplane-1", readEtc(t, etc, "hostname"))
		assert.Equal(t, "nameserver 172.20.0.1", readEtc(t, etc, "resolv.conf"))
	})

	// Outside a container those files are not bind mounts and are not seeded: the lower shows
	// through as-is, and hostname stays the empty rootfs default.
	t.Run("non-container mode does not seed the runtime binds", func(t *testing.T) {
		etc, _ := newEtcOverlay(t, nil)

		assert.Equal(t, "", readEtc(t, etc, "hostname"))
	})

	// /etc is read-only at the path level: a write via the path fails even though the overlay is
	// writable through the returned root.
	t.Run("etc is read-only at the path level", func(t *testing.T) {
		etc, _ := newEtcOverlay(t, map[string]string{"os-release": "STATIC"})

		assert.Error(t, os.WriteFile(filepath.Join(etc, "newfile"), []byte("x"), 0o644))
	})
}

// newEtcOverlay populates a throwaway rootfs /etc with the given static files, then composes the
// real setupEtcOverlay over it in non-container mode. It returns the overlay mountpoint (read
// through it) and the writable overlay root (write through it, as controllers do).
// Use newEtcOverlayWithBinds for the container case.
func newEtcOverlay(t *testing.T, staticFiles map[string]string) (string, xfs.Root) {
	t.Helper()

	etcPath := filepath.Join(t.TempDir(), "etc")
	require.NoError(t, os.MkdirAll(etcPath, 0o755))

	// setupEtcOverlay seeds the files the runtime bind-mounts under /etc: extensions.yaml
	// (initramfs bind) always, plus resolv.conf and hostname (runtime binds) in container mode.
	// The seed reads each one, so they have to exist in the lower whatever the mode.
	for _, f := range []string{"extensions.yaml", "resolv.conf", "hostname"} {
		require.NoError(t, os.WriteFile(filepath.Join(etcPath, f), nil, 0o644))
	}

	// cni/kubernetes mountpoints are provided by the squashfs lower, create them so the
	// writable-tmpfs move_mounts have a target.
	for _, d := range []string{"cni/net.d", "kubernetes/manifests"} {
		require.NoError(t, os.MkdirAll(filepath.Join(etcPath, d), 0o755))
	}

	for path, contents := range staticFiles {
		require.NoError(t, os.MkdirAll(filepath.Join(etcPath, filepath.Dir(path)), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(etcPath, path), []byte(contents), 0o644))
	}

	etcRoot, unmount, err := v1alpha2.SetupEtcOverlay(etcPath, false, []fsopen.Option{
		fsopen.WithStringParameter("mode", "0755"),
		fsopen.WithStringParameter("size", "8M"),
	}, zaptest.NewLogger(t))
	require.NoError(t, err)
	t.Cleanup(func() { unmount() }) //nolint:errcheck

	return etcPath, etcRoot
}

func readEtc(t *testing.T, etc, name string) string {
	t.Helper()

	contents, err := os.ReadFile(filepath.Join(etc, name))
	require.NoError(t, err)

	return string(contents)
}

// newEtcOverlayWithBinds reproduces the container layout: each named file is a bind mount over an
// otherwise-empty file in the throwaway rootfs /etc, the way Docker mounts
// /var/lib/docker/containers/<id>/hostname onto /etc/hostname. The overlay is composed in
// container mode, so the runtime binds get seeded. It returns the overlay mountpoint.
func newEtcOverlayWithBinds(t *testing.T, binds map[string]string) string {
	t.Helper()

	// The bind sources live outside the /etc that gets cloned as the overlay lower.
	srcDir := t.TempDir()

	etcPath := filepath.Join(t.TempDir(), "etc")
	require.NoError(t, os.MkdirAll(etcPath, 0o755))

	for _, f := range []string{"extensions.yaml", "resolv.conf", "hostname"} {
		require.NoError(t, os.WriteFile(filepath.Join(etcPath, f), nil, 0o644))
	}

	for _, d := range []string{"cni/net.d", "kubernetes/manifests"} {
		require.NoError(t, os.MkdirAll(filepath.Join(etcPath, d), 0o755))
	}

	for name, contents := range binds {
		src := filepath.Join(srcDir, name)
		require.NoError(t, os.WriteFile(src, []byte(contents), 0o644))

		target := filepath.Join(etcPath, name)
		require.NoError(t, unix.Mount(src, target, "", unix.MS_BIND, ""))

		t.Cleanup(func() { unix.Unmount(target, unix.MNT_DETACH) }) //nolint:errcheck
	}

	// Sanity: the binds are in place before the overlay is composed, so a failure in the assertions
	// is the overlay dropping them rather than a broken fixture.
	for name, contents := range binds {
		require.Equal(t, contents, readEtc(t, etcPath, name))
	}

	etcRoot, unmount, err := v1alpha2.SetupEtcOverlay(etcPath, true, []fsopen.Option{
		fsopen.WithStringParameter("mode", "0755"),
		fsopen.WithStringParameter("size", "8M"),
	}, zaptest.NewLogger(t))
	require.NoError(t, err)

	t.Cleanup(func() { unmount() })       //nolint:errcheck
	t.Cleanup(func() { etcRoot.Close() }) //nolint:errcheck

	return etcPath
}
