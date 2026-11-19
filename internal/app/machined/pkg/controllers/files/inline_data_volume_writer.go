// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package files

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"strings"
	"syscall"

	configconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
)

// Reserved names in the volume root; configs reject paths starting with "..", so they never collide.
const (
	inlineDataVolumeDataLink      = "..data"
	inlineDataVolumeDataDirPrefix = "..data-"
	inlineDataVolumeTmpPrefix     = "..tmp-"

	inlineDataVolumeTmpLink = inlineDataVolumeTmpPrefix + "link"
)

// InlineDataVolumeContentsHash returns the hash identifying a set of inline data volume files.
// The hash is keyed, so it doesn't leak secret contents.
func InlineDataVolumeContentsHash(key []byte, files []configconfig.InlineDataVolumeFile) string {
	hasher := hmac.New(sha256.New, key)

	for _, file := range files {
		// length-prefixed to avoid ambiguity
		for _, field := range []string{file.Path, file.Contents} {
			hasher.Write(binary.BigEndian.AppendUint64(nil, uint64(len(field))))
			hasher.Write([]byte(field))
		}

		hasher.Write(binary.BigEndian.AppendUint32(nil, uint32(file.Mode.Perm())))
	}

	return hex.EncodeToString(hasher.Sum(nil))[:16]
}

// WriteInlineDataVolume renders files into root with a ConfigMap-style layout, returning their hash.
//
// Files go into `..data-<hash>`, top-level entries link into `..data`, and `..data` is swapped atomically.
//
//nolint:gocyclo
func WriteInlineDataVolume(root *os.Root, hashKey []byte, files []configconfig.InlineDataVolumeFile) (string, error) {
	hash := InlineDataVolumeContentsHash(hashKey, files)
	dataDir := inlineDataVolumeDataDirPrefix + hash

	current, err := root.Readlink(inlineDataVolumeDataLink)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("failed to read data link: %w", err)
	}

	if current != dataDir {
		// the volume is sized for two sets of files: the current one, and the one being written
		if err = removeStaleDataDirs(root, current); err != nil {
			return "", err
		}

		if err = writeInlineDataVolumeDataDir(root, dataDir, files); err != nil {
			return "", err
		}

		if err = replaceSymlink(root, dataDir, inlineDataVolumeDataLink, inlineDataVolumeTmpLink); err != nil {
			return "", fmt.Errorf("failed to swap data link: %w", err)
		}
	}

	wantedEntries := map[string]struct{}{}

	for _, file := range files {
		entry, _, _ := strings.Cut(file.Path, "/")
		wantedEntries[entry] = struct{}{}
	}

	for entry := range wantedEntries {
		target := path.Join(inlineDataVolumeDataLink, entry)

		if link, linkErr := root.Readlink(entry); linkErr == nil && link == target {
			continue
		}

		if err = replaceSymlink(root, target, entry, inlineDataVolumeTmpLink); err != nil {
			return "", fmt.Errorf("failed to link %q: %w", entry, err)
		}
	}

	dir, err := root.Open(".")
	if err != nil {
		return "", fmt.Errorf("failed to open volume root: %w", err)
	}

	names, err := dir.Readdirnames(-1)
	dir.Close() //nolint:errcheck

	if err != nil {
		return "", fmt.Errorf("failed to list volume root: %w", err)
	}

	for _, name := range names {
		if _, wanted := wantedEntries[name]; wanted || name == inlineDataVolumeDataLink || name == dataDir {
			continue
		}

		if err = root.RemoveAll(name); err != nil {
			return "", fmt.Errorf("failed to remove stale %q: %w", name, err)
		}
	}

	return hash, nil
}

// removeStaleDataDirs removes the data directories and temporary links left by interrupted writes, except keep.
func removeStaleDataDirs(root *os.Root, keep string) error {
	dir, err := root.Open(".")
	if err != nil {
		return fmt.Errorf("failed to open volume root: %w", err)
	}

	names, err := dir.Readdirnames(-1)
	dir.Close() //nolint:errcheck

	if err != nil {
		return fmt.Errorf("failed to list volume root: %w", err)
	}

	for _, name := range names {
		if name == keep {
			continue
		}

		if !strings.HasPrefix(name, inlineDataVolumeDataDirPrefix) && !strings.HasPrefix(name, inlineDataVolumeTmpPrefix) {
			continue
		}

		if err = root.RemoveAll(name); err != nil {
			return fmt.Errorf("failed to remove stale %q: %w", name, err)
		}
	}

	return nil
}

func writeInlineDataVolumeDataDir(root *os.Root, dataDir string, files []configconfig.InlineDataVolumeFile) error {
	// leftover of an interrupted write
	if err := root.RemoveAll(dataDir); err != nil {
		return fmt.Errorf("failed to clean up %q: %w", dataDir, err)
	}

	if err := root.Mkdir(dataDir, 0o755); err != nil {
		return fmt.Errorf("failed to create %q: %w", dataDir, err)
	}

	for _, file := range files {
		filePath := path.Join(dataDir, file.Path)

		if err := root.MkdirAll(path.Dir(filePath), 0o755); err != nil {
			return fmt.Errorf("failed to create directory for %q: %w", file.Path, err)
		}

		if err := root.WriteFile(filePath, []byte(file.Contents), file.Mode.Perm()); err != nil {
			return fmt.Errorf("failed to write %q: %w", file.Path, err)
		}

		// the mode passed on creation is subject to the umask
		if err := root.Chmod(filePath, file.Mode.Perm()); err != nil {
			return fmt.Errorf("failed to set mode of %q: %w", file.Path, err)
		}
	}

	return nil
}

// replaceSymlink atomically points name to target via a renamed temporary symlink.
func replaceSymlink(root *os.Root, target, name, tmpName string) error {
	if err := root.Remove(tmpName); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	if err := root.Symlink(target, tmpName); err != nil {
		return err
	}

	err := renameSymlink(root, tmpName, name)
	if err == nil {
		return nil
	}

	if removeErr := root.Remove(tmpName); removeErr != nil && !errors.Is(removeErr, fs.ErrNotExist) {
		err = errors.Join(err, removeErr)
	}

	return err
}

// renameSymlink renames the symlink tmpName over name.
//
// Rename can't replace a directory, which only then is removed first;
// on any other error name is left untouched, so it keeps working.
func renameSymlink(root *os.Root, tmpName, name string) error {
	err := root.Rename(tmpName, name)
	if err == nil || !isDirectoryRenameError(err) {
		return err
	}

	if err = root.RemoveAll(name); err != nil {
		return err
	}

	return root.Rename(tmpName, name)
}

// isDirectoryRenameError reports whether rename failed because the destination is a directory.
func isDirectoryRenameError(err error) bool {
	return errors.Is(err, syscall.EISDIR) || errors.Is(err, syscall.ENOTEMPTY) || errors.Is(err, syscall.EEXIST)
}
