// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package files

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"strings"

	configconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
)

// Reserved names in the volume root; configs reject paths starting with "..", so they never collide.
const (
	inlineDataVolumeDataLink       = "..data"
	inlineDataVolumeDataLinkTmp    = "..data_tmp"
	inlineDataVolumeDataDirPrefix  = "..data-"
	inlineDataVolumeEntryTmpPrefix = "..tmp-"
)

// InlineDataVolumeContentsHash returns the hash identifying a set of inline data volume files.
func InlineDataVolumeContentsHash(files []configconfig.InlineDataVolumeFile) string {
	hasher := sha256.New()

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
func WriteInlineDataVolume(root *os.Root, files []configconfig.InlineDataVolumeFile) (string, error) {
	hash := InlineDataVolumeContentsHash(files)
	dataDir := inlineDataVolumeDataDirPrefix + hash

	current, err := root.Readlink(inlineDataVolumeDataLink)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("failed to read data link: %w", err)
	}

	if current != dataDir {
		if err = writeInlineDataVolumeDataDir(root, dataDir, files); err != nil {
			return "", err
		}

		if err = replaceSymlink(root, dataDir, inlineDataVolumeDataLink, inlineDataVolumeDataLinkTmp); err != nil {
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

		if err = replaceSymlink(root, target, entry, inlineDataVolumeEntryTmpPrefix+entry); err != nil {
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

	if err := root.Rename(tmpName, name); err != nil {
		// rename can't replace a directory
		if removeErr := root.RemoveAll(name); removeErr != nil {
			return errors.Join(err, removeErr)
		}

		return root.Rename(tmpName, name)
	}

	return nil
}
