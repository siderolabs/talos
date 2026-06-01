// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package makefs

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

// GenerateProtofile walks a filesystem tree and generates an XFS protofile.
// All files and directories will have uid/gid mapped to 0:0.
func GenerateProtofile(sourcePath string) (io.Reader, error) {
	var buf bytes.Buffer

	// Emit protofile header
	fmt.Fprintln(&buf, "/")
	fmt.Fprintln(&buf, "0 0")

	// Get stat of the root directory
	statbuf, err := os.Stat(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat path %s: %w", sourcePath, err)
	}

	if !statbuf.IsDir() {
		return nil, fmt.Errorf("path %s is not a directory", sourcePath)
	}

	// Write root directory stat
	fmt.Fprintln(&buf, statToProtoStr(statbuf))
	// Walk the tree
	if err := walkTree(&buf, sourcePath, 1); err != nil {
		return nil, err
	}

	fmt.Fprintln(&buf, "$")

	return &buf, nil
}

// statToProtoStr converts a FileInfo to a proto string.
func statToProtoStr(info fs.FileInfo) string {
	mode := info.Mode()

	var fileType rune

	switch {
	case mode.IsRegular():
		fileType = '-'
	case mode&fs.ModeCharDevice != 0:
		fileType = 'c'
	case mode&fs.ModeDevice != 0:
		fileType = 'b'
	case mode&fs.ModeNamedPipe != 0:
		fileType = 'p'
	case mode.IsDir():
		fileType = 'd'
	case mode&fs.ModeSymlink != 0:
		fileType = 'l'
	default:
		fileType = '-'
	}

	var suid, sgid rune
	if mode&fs.ModeSetuid != 0 {
		suid = 'u'
	} else {
		suid = '-'
	}

	if mode&fs.ModeSetgid != 0 {
		sgid = 'g'
	} else {
		sgid = '-'
	}

	// Extract permissions (mask out file type and special bits)
	perms := mode.Perm()

	return fmt.Sprintf("%c%c%c%03o %d %d", fileType, suid, sgid, perms, 0, 0)
}

// statToExtra computes the extras column for a protofile entry.
func statToExtra(info fs.FileInfo, fullpath string) (string, error) {
	mode := info.Mode()

	switch {
	case mode.IsRegular():
		return fmt.Sprintf(" %s", fullpath), nil
	case mode&fs.ModeCharDevice != 0, mode&fs.ModeDevice != 0:
		if sys := info.Sys(); sys != nil {
			if stat, ok := sys.(*syscall.Stat_t); ok {
				major := unix.Major(stat.Rdev)
				minor := unix.Minor(stat.Rdev)

				return fmt.Sprintf(" %d %d", major, minor), nil
			}
		}

		return " 0 0", nil
	case mode&fs.ModeSymlink != 0:
		target, err := os.Readlink(fullpath)
		if err != nil {
			return "", fmt.Errorf("failed to read symlink %s: %w", fullpath, err)
		}

		return fmt.Sprintf(" %s", target), nil
	}

	return "", nil
}

// walkTree walks the directory tree rooted by path.
//
//nolint:gocyclo
func walkTree(w io.Writer, path string, depth int) error {
	type entry struct {
		name     string
		fullpath string
		info     fs.FileInfo
		isDir    bool
	}

	var entries []entry

	err := filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip the root directory itself
		if p == path {
			return nil
		}

		// Only process direct children of the current path
		rel, err := filepath.Rel(path, p)
		if err != nil {
			return err
		}

		if strings.Contains(rel, string(filepath.Separator)) {
			return filepath.SkipDir
		}

		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("failed to stat %s: %w", p, err)
		}

		// Skip sockets
		if info.Mode()&fs.ModeSocket != 0 {
			return nil
		}

		// Validate no spaces in name
		if strings.Contains(d.Name(), " ") {
			return fmt.Errorf("spaces not allowed in file names: %s", d.Name())
		}

		entries = append(entries, entry{
			name:     d.Name(),
			fullpath: p,
			info:     info,
			isDir:    d.IsDir(),
		})

		// Skip descending into directories, as we'll recurse manually later
		if d.IsDir() {
			return filepath.SkipDir
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to walk directory %s: %w", path, err)
	}

	// Print files first
	for _, e := range entries {
		if !e.isDir {
			extra, err := statToExtra(e.info, e.fullpath)
			if err != nil {
				return err
			}

			indent := strings.Repeat(" ", depth)
			fmt.Fprintf(w, "%s%s %s%s\n", indent, e.name, statToProtoStr(e.info), extra)
		}
	}

	// Print and recurse into directories
	for _, e := range entries {
		if e.isDir {
			extra, err := statToExtra(e.info, e.fullpath)
			if err != nil {
				return err
			}

			indent := strings.Repeat(" ", depth)
			fmt.Fprintf(w, "%s%s %s%s\n", indent, e.name, statToProtoStr(e.info), extra)

			if err := walkTree(w, e.fullpath, depth+1); err != nil {
				return err
			}
		}
	}

	// Close directory marker (except for depth 1)
	if depth > 1 {
		indent := strings.Repeat(" ", depth-1)
		fmt.Fprintf(w, "%s$\n", indent)
	}

	return nil
}
