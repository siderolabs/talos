// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package contentlibrary manages immutable SHA256 objects and create-only root aliases.
package contentlibrary

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/opencontainers/go-digest"
	"golang.org/x/sys/unix"
)

const objectDir = "objects/sha256"

// ErrObjectReferenced indicates that aliases still refer to an object.
var ErrObjectReferenced = errors.New("content library object is referenced")

// WithMutation serializes cooperating library mutations across root handles and processes.
// Callers must not invoke Publish, PublishObject, RemoveAlias, or RemoveObject inside fn.
// All publication of aliases (including private ownership witnesses) must occur under
// this guard to prevent an alias appearing between a removal scan and unlink.
func WithMutation(root *os.Root, fn func() error) (err error) {
	dir, err := root.Open(".")
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, dir.Close()) }()

	if err := unix.Flock(int(dir.Fd()), unix.LOCK_EX); err != nil {
		return err
	}

	// Closing this independently opened directory descriptor releases the lock.
	return fn()
}

// RemoveObject removes an immutable object. By default it refuses references without
// changing anything; force unlinks every referencing symlink before the object.
func RemoveObject(root *os.Root, hash string, force bool) error {
	object, err := ObjectPath(hash)
	if err != nil {
		return err
	}

	return WithMutation(root, func() error {
		if err := verifyObjectDirectories(root); err != nil {
			return err
		}

		info, err := root.Lstat(object)
		if err != nil {
			return err
		}

		if !info.Mode().IsRegular() {
			return fmt.Errorf("object %q is not a regular file", object)
		}

		references, err := objectReferences(root, object)
		if err != nil {
			return err
		}

		if len(references) != 0 && !force {
			return fmt.Errorf("%w: %s; retry with force to remove these aliases", ErrObjectReferenced, strings.Join(references, ", "))
		}

		// Symlink directory entries and their parents are durable before the object
		// entry is removed. A failed unlink/sync leaves the object for safe retry.
		if err := unlinkReferences(root, references); err != nil {
			return err
		}

		if err := root.Remove(object); err != nil {
			return err
		}

		return syncDir(root, objectDir)
	})
}

func verifyObjectDirectories(root *os.Root) error {
	for _, path := range []string{"objects", objectDir} {
		info, err := root.Lstat(path)
		if err != nil {
			return err
		}

		if !info.IsDir() {
			return fmt.Errorf("object directory %q is not a directory", path)
		}
	}

	return nil
}

func unlinkReferences(root *os.Root, references []string) error {
	for _, name := range references {
		if err := root.Remove(name); err != nil {
			return err
		}

		if err := syncDir(root, filepath.Dir(name)); err != nil {
			return err
		}
	}

	return nil
}

func objectReferences(root *os.Root, object string) ([]string, error) {
	objectInfo, err := root.Stat(object)
	if err != nil {
		return nil, err
	}

	links, err := librarySymlinks(root, ".")
	if err != nil {
		return nil, err
	}

	var references []string

	for name, target := range links {
		// A nested directory-relative link can have the same text as a CAS
		// alias while pointing at an unrelated file. Its real target wins.
		if info, err := root.Stat(name); err == nil && !os.SameFile(objectInfo, info) {
			continue
		}

		if strings.Contains(target, filepath.Base(object)) &&
			filepath.Clean(target) != object && filepath.Clean(filepath.Join(filepath.Dir(name), target)) != object {
			return nil, fmt.Errorf("unclassifiable object reference %q: %q", name, target)
		}

		if referencesObject(name, object, links, make(map[string]bool)) {
			references = append(references, name)
		}
	}

	sort.Strings(references)

	return references, nil
}

// RemoveAlias removes a validated public alias without removing its object.
func RemoveAlias(root *os.Root, name string) error {
	if !validAliasName(name) {
		return fmt.Errorf("invalid content library alias %q", name)
	}

	return WithMutation(root, func() error {
		if _, err := Resolve(root, name); err != nil {
			return err
		}

		if err := root.Remove(name); err != nil {
			return err
		}

		return syncDir(root, ".")
	})
}

// librarySymlinks walks actual directories only, never following symlink directories.
func librarySymlinks(root *os.Root, dir string) (map[string]string, error) {
	links := make(map[string]string)

	var walk func(string) error

	walk = func(parent string) error {
		folder, err := root.Open(parent)
		if err != nil {
			return err
		}

		entries, err := folder.ReadDir(-1)
		if err := errors.Join(err, folder.Close()); err != nil {
			return err
		}

		for _, entry := range entries {
			name := filepath.Join(parent, entry.Name())

			info, err := root.Lstat(name)
			if err != nil {
				return err
			}

			switch {
			case info.Mode()&os.ModeSymlink != 0:
				target, err := root.Readlink(name)
				if err != nil {
					return err
				}

				links[name] = target
			case info.IsDir():
				if err := walk(name); err != nil {
					return err
				}
			}
		}

		return nil
	}

	return links, walk(dir)
}

// referencesObject considers both root-relative CAS links (including nested
// witnesses) and ordinary directory-relative symlinks/chains. A loop or an
// escaping target cannot certify safety if it mentions the object's identity.
func referencesObject(name, object string, links map[string]string, visited map[string]bool) bool {
	if visited[name] {
		return false
	}

	visited[name] = true

	target := links[name]
	if target == object {
		return true
	}

	for _, candidate := range []string{filepath.Clean(target), filepath.Clean(filepath.Join(filepath.Dir(name), target))} {
		if candidate == object {
			return true
		}

		if _, exists := links[candidate]; exists && referencesObject(candidate, object, links, visited) {
			return true
		}
	}

	return false
}

func syncDir(root *os.Root, path string) error {
	dir, err := root.Open(path)
	if err != nil {
		return err
	}

	return errors.Join(dir.Sync(), dir.Close())
}

// ObjectPath returns the canonical relative path for a SHA256 identity.
func ObjectPath(hash string) (string, error) {
	d, err := digest.Parse(hash)
	if err != nil || d.Algorithm() != digest.SHA256 || d.String() != hash || len(d.Encoded()) != 64 || strings.ToLower(d.Encoded()) != d.Encoded() {
		return "", fmt.Errorf("invalid SHA256 object identity %q", hash)
	}

	return filepath.Join(objectDir, d.Encoded()), nil
}

// Resolve accepts only a root alias pointing to a canonical, regular object.
func Resolve(root *os.Root, name string) (string, error) {
	if !validAliasName(name) {
		return "", fmt.Errorf("invalid content library alias %q", name)
	}

	alias, err := root.Lstat(name)
	if err != nil {
		return "", err
	}

	if alias.Mode()&os.ModeSymlink == 0 {
		return "", fmt.Errorf("alias %q is not a symlink", name)
	}

	path, err := root.Readlink(name)
	if err != nil {
		return "", err
	}

	hash := "sha256:" + filepath.Base(path)

	canonical, err := ObjectPath(hash)
	if err != nil || path != canonical {
		return "", fmt.Errorf("noncanonical content library alias %q", name)
	}

	stat, err := root.Lstat(path)
	if err != nil {
		return "", err
	}

	if !stat.Mode().IsRegular() {
		return "", fmt.Errorf("object %q is not a regular file", path)
	}

	return path, nil
}

func validAliasName(name string) bool {
	return name != "" && !strings.HasPrefix(name, ".") &&
		!strings.ContainsAny(name, "/\\\x00") && name != "objects"
}

// Publish atomically installs a completed staged file, then creates a non-replaceable alias.
// Objects are deliberately retained even when the alias is removed.
func Publish(root *os.Root, stage, name, hash string) error {
	if !validAliasName(name) {
		return fmt.Errorf("invalid content library alias %q", name)
	}

	return WithMutation(root, func() error {
		if _, err := root.Lstat(name); err == nil {
			return fs.ErrExist
		} else if !errors.Is(err, fs.ErrNotExist) {
			return err
		}

		object, err := publishObject(root, stage, hash)
		if err != nil {
			return err
		}

		if err := root.Symlink(object, name); err != nil {
			return err
		}

		return syncDir(root, ".")
	})
}

// PublishObject installs immutable bytes under their digest without replacing an existing object.
func PublishObject(root *os.Root, stage, hash string) (string, error) {
	var object string

	err := WithMutation(root, func() error {
		var err error

		object, err = publishObject(root, stage, hash)

		return err
	})
	if err != nil {
		return "", err
	}

	return object, nil
}

// publishObject requires the caller to hold the mutation lock.
func publishObject(root *os.Root, stage, hash string) (string, error) {
	object, err := ObjectPath(hash)
	if err != nil {
		return "", err
	}

	if err = verifyFile(root, stage, hash); err != nil {
		return "", err
	}

	if err = ensureObjectDirectories(root); err != nil {
		return "", err
	}

	if err = root.Link(stage, object); err != nil {
		if !errors.Is(err, fs.ErrExist) {
			return "", err
		}

		if err = verifyFile(root, object, hash); err != nil {
			return "", err
		}
	}
	// Make every newly created directory entry durable before publishing a name.
	for _, path := range []string{objectDir, "objects", "."} {
		if err := syncDir(root, path); err != nil {
			return "", err
		}
	}

	return object, nil
}

func ensureObjectDirectories(root *os.Root) error {
	for _, path := range []string{"objects", objectDir} {
		if err := root.Mkdir(path, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
			return err
		}
	}

	return nil
}

func verifyFile(root *os.Root, name, hash string) error {
	info, err := root.Lstat(name)
	if err != nil {
		return err
	}

	if !info.Mode().IsRegular() {
		return fmt.Errorf("object %q is not a regular file", name)
	}

	f, err := root.Open(name)
	if err != nil {
		return err
	}

	h := sha256.New()

	_, copyErr := io.Copy(h, f)
	if err := errors.Join(copyErr, f.Sync(), f.Close()); err != nil {
		return err
	}

	if digest.NewDigest(digest.SHA256, h).String() != hash {
		return fmt.Errorf("object %q has conflicting bytes", name)
	}

	return nil
}
