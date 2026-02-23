// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package xfs provides an extended file system interface that includes
// additional methods for writing files and directories, as well as utility
// functions for reading, writing, and manipulating files and directories
// within a specified file system.
package xfs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// FS is an interface for creating file system handles.
type FS interface {
	Open() (int, error)
	io.Closer
	Repair(context.Context) error

	Source() string
	FSType() string
}

// Root is an interface that extends the standard fs.FS interface with Write capabilities.
//
//nolint:interfacebloat
type Root interface {
	fs.FS

	io.Closer
	OpenFS() error
	RepairFS(context.Context) error
	Fd() (int, error)

	Mkdir(name string, perm os.FileMode) error
	OpenFile(name string, flags int, perm os.FileMode) (File, error)
	Remove(name string) error
	Rename(oldname, newname string) error

	Source() string
	FSType() string
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
	Fd() uintptr
}

// Interface guard.
var _ File = (*os.File)(nil)

// ReadFile wraps fs.ReadFile to read a file from the specified FileSystem.
func ReadFile(root Root, name string) ([]byte, error) {
	return fs.ReadFile(root, name)
}

// ReadDir wraps fs.ReadDir to read a directory from the specified FileSystem.
func ReadDir(root Root, name string) ([]fs.DirEntry, error) {
	return fs.ReadDir(root, name)
}

// Stat wraps fs.Stat to get the file or directory information from the specified FileSystem.
func Stat(root Root, name string) (fs.FileInfo, error) {
	return fs.Stat(root, name)
}

// WriteFile is equivalent of os.WriteFile acting on specified FileSystem.
func WriteFile(root Root, name string, data []byte, perm os.FileMode) error {
	f, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
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
func Open(root Root, name string) (fs.File, error) {
	return root.Open(name)
}

// OpenFile wraps (FS).OpenFile.
func OpenFile(root Root, name string, flags int, perm os.FileMode) (File, error) {
	return root.OpenFile(name, flags, perm)
}

// Mkdir wraps (FS).Mkdir.
func Mkdir(root Root, name string, perm os.FileMode) error {
	return root.Mkdir(name, perm)
}

// MkdirAll is equivalent of os.MkdirAll acting on specified FileSystem.
func MkdirAll(root Root, name string, perm os.FileMode) error {
	components := SplitPath(name)

	for i := range len(components) + 1 {
		dir := filepath.Join(components[:i]...)
		if dir == "" {
			// empty name, continue...
			continue
		}

		err := root.Mkdir(dir, perm)
		if err != nil && !errors.Is(err, fs.ErrExist) {
			return fmt.Errorf("%w: %s", err, dir)
		}
	}

	return nil
}

// MkdirTemp creates a temporary directory in the specified directory with a given pattern.
func MkdirTemp(root Root, dir, pattern string) (string, error) {
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
		suffix := strconv.Itoa(rand.Int())

		name := strings.Replace(pattern, "*", suffix, 1)
		if !strings.Contains(pattern, "*") {
			name = pattern + suffix
		}

		tempDir := filepath.Join(dir, name)

		err := MkdirAll(root, tempDir, 0o777)
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
func Remove(root Root, name string) error {
	return root.Remove(name)
}

// RemoveAll is equivalent of os.RemoveAll acting on specified FileSystem.
func RemoveAll(root Root, name string) (err error) {
	var f fs.File

	f, err = root.Open(name)
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
		return root.Remove(name)
	}

	entries, err := ReadDir(root, name)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		childPath := filepath.Join(name, entry.Name())
		if err := RemoveAll(root, childPath); err != nil {
			return err
		}
	}

	return root.Remove(name)
}

// Rename wraps (FS).Rename.
func Rename(root Root, oldname, newname string) error {
	return root.Rename(oldname, newname)
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
