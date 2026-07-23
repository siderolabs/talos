// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package lookpath resolves executables by PATH within a root directory, following
// symlinks confined to that root via openat2(RESOLVE_IN_ROOT).
package lookpath

import (
	"fmt"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

// InRoot resolves name to a path that can be exec'd after chrooting into root.
//
// A name containing a slash is returned unchanged (treated as an explicit path).
// Otherwise the PATH entry from env is searched, and each candidate is resolved under
// root with RESOLVE_IN_ROOT: absolute symlink targets (e.g. Nix profile links pointing
// into /nix/store, which may live on a sub-mount of root) are interpreted relative to
// root, and resolution can never escape it. The returned path is the PATH-relative
// candidate, valid once the caller chroots into root. Returns an error if name is not
// found as an executable file.
func InRoot(root, name string, env []string) (string, error) {
	if strings.Contains(name, "/") {
		return name, nil
	}

	var pathEnv string

	for _, e := range env {
		if v, ok := strings.CutPrefix(e, "PATH="); ok {
			pathEnv = v
		}
	}

	rootFD, err := unix.Open(root, unix.O_PATH|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return "", fmt.Errorf("open root %q: %w", root, err)
	}

	defer unix.Close(rootFD) //nolint:errcheck

	for _, dir := range filepath.SplitList(pathEnv) {
		if dir == "" {
			continue
		}

		candidate := filepath.Join(dir, name)

		if executableInRoot(rootFD, candidate) {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("executable %q not found in $PATH", name)
}

// executableInRoot reports whether candidate resolves, confined to rootFD, to a
// regular executable file.
func executableInRoot(rootFD int, candidate string) bool {
	fd, err := unix.Openat2(rootFD, strings.TrimPrefix(candidate, "/"), &unix.OpenHow{
		Flags:   unix.O_PATH | unix.O_CLOEXEC,
		Resolve: unix.RESOLVE_IN_ROOT | unix.RESOLVE_NO_MAGICLINKS,
	})
	if err != nil {
		return false
	}

	defer unix.Close(fd) //nolint:errcheck

	var st unix.Stat_t
	if unix.Fstat(fd, &st) != nil {
		return false
	}

	return st.Mode&unix.S_IFMT == unix.S_IFREG && st.Mode&0o111 != 0
}
