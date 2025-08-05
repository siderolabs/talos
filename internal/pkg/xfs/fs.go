// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package xfs provides an extended file system interface that includes
// additional methods for writing files and directories, as well as utility
// functions for reading, writing, and manipulating files and directories
// within a specified file system.
package xfs

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var random = rand.New(rand.NewSource(time.Now().UnixNano()))

// ErrNotMounted is returned when a filesystem does not have a mount file descriptor.
var ErrNotMounted = errors.New("filesystem does not have a mount file descriptor")

// Creator is an interface for creating file system handles.
type Creator interface {
	io.Closer
	Create() (int, error)
}

// Mounter is an interface for mounting filesystems to a directory.
type Mounter interface {
	MountAt(path string) (string, error)
	UnmountFrom(path string) error
}

// FS is an interface that extends the standard fs.FS interface with Write capabilities.
type FS interface {
	fs.FS

	MountPoint() string
	FileDescriptor() int

	Mkdir(name string, perm os.FileMode) error
	OpenFile(name string, flags int, perm os.FileMode) (File, error)
	Remove(name string) error
}

// File is an interface that extends the standard fs.File interface with additional methods for writing.
type File interface {
	fs.File
	io.Closer
	io.Reader
	io.ReaderAt
	io.Seeker
	io.Writer
	io.WriterAt
}

// Interface guard.
var _ File = (*os.File)(nil)

// ReadFile wraps fs.ReadFile to read a file from the specified FileSystem.
func ReadFile(fsys FS, name string) ([]byte, error) {
	return fs.ReadFile(fsys, name)
}

// ReadDir wraps fs.ReadDir to read a directory from the specified FileSystem.
func ReadDir(fsys FS, name string) ([]fs.DirEntry, error) {
	return fs.ReadDir(fsys, name)
}

// Stat wraps fs.Stat to get the file or directory information from the specified FileSystem.
func Stat(fsys FS, name string) (fs.FileInfo, error) {
	return fs.Stat(fsys, name)
}

// WriteFile is equivalent of os.WriteFile acting on specified FileSystem.
func WriteFile(fsys FS, name string, data []byte, perm os.FileMode) error {
	f, err := fsys.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}

	_, err = f.Write(data)
	if err1 := f.Close(); err1 != nil && err == nil {
		err = err1
	}

	return err
}

// Open wraps (FS).Open.
func Open(fsys FS, name string) (fs.File, error) {
	return fsys.Open(name)
}

// OpenFile wraps (FS).OpenFile.
func OpenFile(fsys FS, name string, flags int, perm os.FileMode) (fs.File, error) {
	return fsys.OpenFile(name, flags, perm)
}

// Mkdir wraps (FS).Mkdir.
func Mkdir(fsys FS, name string, perm os.FileMode) error {
	err := fsys.Mkdir(name, perm)
	if err != nil {
		return fmt.Errorf("%w: %s", err, name)
	}

	return nil
}

// MkdirAll is equivalent of os.MkdirAll acting on specified FileSystem.
func MkdirAll(fsys FS, name string, perm os.FileMode) error {
	components := SplitPath(name)

	for i := range len(components) + 1 {
		dir := filepath.Join(components[:i]...)
		if dir == "" {
			// empty name, continue...
			continue
		}

		err := fsys.Mkdir(dir, perm)
		if err != nil && !errors.Is(err, fs.ErrExist) {
			return fmt.Errorf("%w: %s", err, dir)
		}
	}

	return nil
}

// MkdirTemp creates a temporary directory in the specified directory with a given pattern.
func MkdirTemp(fsys FS, dir, pattern string) (string, error) {
	if dir == "" {
		dir = os.TempDir()
	}

	if pattern == "" {
		pattern = "tmp"
	}

	if strings.Count(pattern, "*") > 1 {
		return "", fmt.Errorf("pattern %q must contain at most one '*'", pattern)
	}

	const maxAttempts = 10000

	for range maxAttempts {
		suffix := strconv.Itoa(random.Intn(1_000_000_000))

		name := strings.Replace(pattern, "*", suffix, 1)
		if !strings.Contains(pattern, "*") {
			name = pattern + suffix
		}

		tempDir := filepath.Join(dir, name)

		err := MkdirAll(fsys, tempDir, 0o777)
		if err == nil {
			return tempDir, nil
		}

		if errors.Is(err, fs.ErrExist) {
			continue
		}

		return "", err
	}

	return "", fmt.Errorf("failed to create temporary directory after %d attempts", maxAttempts)
}

// Remove wraps (FS).Remove.
func Remove(fsys FS, name string) error {
	return fsys.Remove(name)
}

// RemoveAll is equivalent of os.RemoveAll acting on specified FileSystem.
func RemoveAll(fsys FS, name string) (err error) {
	var f fs.File

	f, err = fsys.Open(name)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}

		return err
	}

	stat, err := f.Stat()
	if err != nil {
		return err
	}

	if err := f.Close(); err != nil {
		return err
	}

	if !stat.IsDir() {
		return fsys.Remove(name)
	}

	entries, err := ReadDir(fsys, name)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		childPath := filepath.Join(name, entry.Name())
		if err := RemoveAll(fsys, childPath); err != nil {
			return err
		}
	}

	return fsys.Remove(name)
}

// SplitPath splits a path into its components, similar to filepath.Split but returns all parts.
func SplitPath(path string) []string {
	var parts []string

	for {
		dir, file := filepath.Split(path)
		if file != "" {
			parts = append([]string{file}, parts...)
		}

		if dir == "" || dir == path {
			break
		}

		path = filepath.Clean(dir)
	}

	return parts
}
