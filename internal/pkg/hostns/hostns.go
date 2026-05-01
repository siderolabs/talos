// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package hostns builds the PROFILE_HOST_NS debug root in the host mount namespace.
//
// It does NOT fork the mount namespace: the debug session runs in the host's mount
// namespace so tools that manage host mounts (zpool, mount, umount) actually affect
// the host. The Nix toolset is layered in via an overlay whose scratch mounts live
// under a per-session directory and are removed on teardown, so the only lasting
// footprint is what the session deliberately creates on the host.
package hostns

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/containerd/containerd/v2/core/mount"
	"golang.org/x/sys/unix"

	mountv3 "github.com/siderolabs/talos/internal/pkg/mount/v3"
)

// HostBinds are the live host mounts bind-mounted into the debug root so tools can
// reach the running node: devices, kernel filesystems, runtime sockets (/run,
// /system) and data (/var). They are recursively bind-mounted and made shared, so a
// mount a session creates under them (e.g. `zpool create -m /var/tank`) propagates to
// the host, and unmounts propagate back — the session manages host mounts as if it
// were in the host namespace (which it is). Flags (nodev, nosuid, noexec) are inherited
// from the already-compliant host mounts.
//
// /var is bound even though the session's own scratch (merged, image, overlay uppers)
// lives under it: Setup marks those scratch mounts MS_UNBINDABLE, so the recursive /var
// bind skips them and cannot nest merged into itself.
var HostBinds = []string{"dev", "proc", "sys", "run", "system", "var"}

// Setup builds the debug chroot root in the CURRENT (host) mount namespace:
//   - the image snapshot mounted read-only at baseDir/image (via fsopen, so the many-
//     layer overlay does not go through containerd's mount.All, whose CLONE_FS worker
//     would mount it in a way that escapes cleanup);
//   - an overlay root (lower=/ plus a disk-backed upper under varBase) at baseDir/merged;
//   - the image's /nix overlaid at merged/nix (disk-backed upper);
//   - HostBinds recursively bind-mounted and shared into merged.
//
// It returns the merged root (for the child to chroot into) and a teardown func.
// Teardown makes merged's subtree private and then recursively unmounts merged, so it
// removes ONLY our scratch mounts: the host's own mounts, and anything the session
// propagated to the host (e.g. a created zfs pool), are left intact.
func Setup(snapshotMounts []mount.Mount, baseDir, varBase string) (merged string, teardown func() error, err error) { //nolint:gocyclo
	imageDir := filepath.Join(baseDir, "image")
	merged = filepath.Join(baseDir, "merged")
	mergedMounted := false

	// cleanup removes only our scratch mounts: it severs propagation first so tearing
	// down our overlay does not ripple onto the host's mounts, or onto anything a
	// session propagated up (e.g. /var/tank of a freshly created pool). merged (and its
	// /nix overlay + host binds) is unmounted first, then the sibling image mount it
	// layered on. Held in a local so the error-path rollback below never sees the nil
	// teardown that an error `return` leaves in the named result.
	cleanup := func() error {
		if mergedMounted {
			if privateErr := unix.Mount("", merged, "", unix.MS_PRIVATE|unix.MS_REC, ""); privateErr != nil {
				return fmt.Errorf("make merged root private: %w", privateErr)
			}
		}

		return errors.Join(
			mount.UnmountRecursive(merged, unix.MNT_DETACH),
			mount.UnmountRecursive(imageDir, unix.MNT_DETACH),
		)
	}

	// Roll back any partial setup on error.
	defer func() {
		if err != nil {
			cleanup() //nolint:errcheck
		}
	}()

	if err = os.MkdirAll(imageDir, 0o755); err != nil {
		return "", nil, fmt.Errorf("mkdir image dir: %w", err)
	}

	if err = mountImageSnapshot(snapshotMounts, imageDir); err != nil {
		return "", nil, fmt.Errorf("mount image snapshot: %w", err)
	}

	// Overlay root: host / as lower, disk-backed upper/work under varBase (mountv3
	// creates the target and the upper/work dirs). The upper lets the session create
	// /nix, /etc/nix, etc. without touching Talos's immutable squashfs, on disk.
	if _, err = mountv3.NewOverlayWithBasePath([]string{"/"}, merged, varBase, nil).Mount(); err != nil {
		return "", nil, fmt.Errorf("mount overlay root: %w", err)
	}

	mergedMounted = true

	// Overlay the image /nix at merged/nix, disk-backed upper, so new nix store paths
	// from `nix profile install` land on disk (the inmem containerd root is tmpfs).
	nixSrc := filepath.Join(imageDir, "nix")
	nixDst := filepath.Join(merged, "nix")

	if _, err = mountv3.NewOverlayWithBasePath([]string{nixSrc}, nixDst, varBase, nil).Mount(); err != nil {
		return "", nil, fmt.Errorf("mount /nix overlay: %w", err)
	}

	// merged and image live under /var (baseDir), which is recursively bound into
	// merged/var below. Mark them MS_UNBINDABLE *now* — after the /nix overlay, which
	// clones imageDir/nix as its lower (unbindable would block that) — so the recursive
	// /var bind skips them instead of re-including merged and nesting it into itself (an
	// exploding mount recursion). Unbindable blocks being a bind *source* only; the host
	// binds below still mount *under* merged fine.
	for _, m := range []string{imageDir, merged} {
		if err = unix.Mount("", m, "", unix.MS_UNBINDABLE, ""); err != nil {
			return "", nil, fmt.Errorf("make %s unbindable: %w", m, err)
		}
	}

	// Bind the live host mounts in, shared. The overlay lowerdir=/ only exposes the
	// squashfs mount-point directories, not the live devtmpfs/procfs/tmpfs/EPHEMERAL
	// mounts, so bind them in explicitly. Shared propagation makes host mount
	// management work both ways (see HostBinds).
	for _, dir := range HostBinds {
		src := "/" + dir

		// Skip a bind whose source the host doesn't have. On Talos all of HostBinds
		// exist; this keeps the setup robust (and testable off-Talos) rather than
		// hard-failing the whole session over one missing path.
		if _, statErr := os.Stat(src); statErr != nil {
			continue
		}

		dst := filepath.Join(merged, dir)

		if err = os.MkdirAll(dst, 0o755); err != nil {
			return "", nil, fmt.Errorf("mkdir %s: %w", dst, err)
		}

		if err = unix.Mount(src, dst, "", unix.MS_BIND|unix.MS_REC, ""); err != nil {
			return "", nil, fmt.Errorf("bind-mount %s: %w", src, err)
		}

		if err = unix.Mount("", dst, "", unix.MS_SHARED|unix.MS_REC, ""); err != nil {
			return "", nil, fmt.Errorf("make %s shared: %w", dst, err)
		}
	}

	return merged, cleanup, nil
}

// mountImageSnapshot mounts the containerd image snapshot read-only at target using
// fsopen (mountv3) for the overlay case, rather than containerd's mount.All whose
// many-layer path mounts from a CLONE_FS-only worker. We only read the image (the
// /nix overlay layered on top supplies the writable layer), so read-only suffices.
func mountImageSnapshot(mounts []mount.Mount, target string) error {
	if len(mounts) == 1 && mounts[0].Type == "overlay" {
		var (
			lowers []string
			upper  string
		)

		for _, o := range mounts[0].Options {
			switch {
			case strings.HasPrefix(o, "lowerdir="):
				lowers = strings.Split(strings.TrimPrefix(o, "lowerdir="), ":")
			case strings.HasPrefix(o, "upperdir="):
				upper = strings.TrimPrefix(o, "upperdir=")
			}
		}

		// overlay lowerdir is ordered top-to-bottom; the writable upper (if any) sits
		// above all lowers, so it becomes the highest read-only layer.
		dirs := lowers
		if upper != "" {
			dirs = append([]string{upper}, lowers...)
		}

		if _, err := mountv3.NewReadOnlyOverlay(dirs, target, nil).Mount(); err != nil {
			return err
		}

		return nil
	}

	// Single-layer snapshots come back as a bind mount; mount.All handles those
	// inline (no lowerdir compaction, no worker goroutine).
	return mount.All(mounts, target)
}
