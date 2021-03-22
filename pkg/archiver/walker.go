// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package archiver

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

// FileItem is unit of work for archive.
type FileItem struct {
	FullPath string
	RelPath  string
	FileInfo os.FileInfo
	Link     string
	Error    error
}

// FileType is a file type.
type FileType int

// File types.
const (
	RegularFileType FileType = iota
	DirectoryFileType
	SymlinkFileType
)

type walkerOptions struct {
	skipRoot        bool
	maxRecurseDepth int
	fnmatchPatterns []string
	types           map[FileType]struct{}
}

// WalkerOption configures Walker.
type WalkerOption func(*walkerOptions)

// WithSkipRoot skips root path if it's a directory.
func WithSkipRoot() WalkerOption {
	return func(o *walkerOptions) {
		o.skipRoot = true
	}
}

// WithMaxRecurseDepth controls maximum recursion depth while walking file tree.
//
// Value of -1 disables depth control.
func WithMaxRecurseDepth(maxDepth int) WalkerOption {
	return func(o *walkerOptions) {
		o.maxRecurseDepth = maxDepth
	}
}

// WithFnmatchPatterns filters results to match the patterns.
//
// Default is not to do any filtering.
func WithFnmatchPatterns(patterns ...string) WalkerOption {
	return func(o *walkerOptions) {
		o.fnmatchPatterns = append(o.fnmatchPatterns, patterns...)
	}
}

// WithFileTypes filters results by file types.
//
// Default is not to do any filtering.
func WithFileTypes(fileType ...FileType) WalkerOption {
	return func(o *walkerOptions) {
		o.types = make(map[FileType]struct{}, len(fileType))
		for _, t := range fileType {
			o.types[t] = struct{}{}
		}
	}
}

// Walker provides a channel of file info/paths for archival.
//
//nolint:gocyclo,cyclop
func Walker(ctx context.Context, rootPath string, options ...WalkerOption) (<-chan FileItem, error) {
	var opts walkerOptions
	opts.maxRecurseDepth = -1

	for _, o := range options {
		o(&opts)
	}

	info, err := os.Lstat(rootPath)
	if err != nil {
		return nil, err
	}

	if info.Mode()&os.ModeSymlink == os.ModeSymlink {
		rootPath, err = filepath.EvalSymlinks(rootPath)
		if err != nil {
			return nil, err
		}
	}

	ch := make(chan FileItem)

	go func() {
		defer close(ch)

		err := filepath.Walk(rootPath, func(path string, fileInfo os.FileInfo, walkErr error) error {
			item := FileItem{
				FullPath: path,
				FileInfo: fileInfo,
				Error:    walkErr,
			}

			if path == rootPath && !fileInfo.IsDir() {
				// only one file
				item.RelPath = filepath.Base(path)
			} else if item.Error == nil {
				item.RelPath, item.Error = filepath.Rel(rootPath, path)
			}

			// TODO: refactor all those `if item.Error == nil &&` conditions

			if item.Error == nil && len(opts.types) > 0 {
				var matches bool

				for t := range opts.types {
					switch t {
					case RegularFileType:
						matches = fileInfo.Mode()&os.ModeType == 0
					case DirectoryFileType:
						matches = fileInfo.Mode()&os.ModeDir != 0
					case SymlinkFileType:
						matches = fileInfo.Mode()&os.ModeSymlink != 0
					}
					if matches {
						break
					}
				}

				if !matches {
					return nil
				}
			}

			if item.Error == nil && path == rootPath && opts.skipRoot && fileInfo.IsDir() {
				// skip containing directory
				return nil
			}

			if item.Error == nil && fileInfo.Mode()&os.ModeSymlink == os.ModeSymlink {
				item.Link, item.Error = os.Readlink(path)
			}

			if item.Error == nil && len(opts.fnmatchPatterns) > 0 {
				var matches bool

				for _, pattern := range opts.fnmatchPatterns {
					if matches, _ = filepath.Match(pattern, item.RelPath); matches { //nolint:errcheck
						break
					}
				}

				if !matches {
					return nil
				}
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case ch <- item:
			}

			if item.Error == nil && fileInfo.IsDir() && atMaxDepth(opts.maxRecurseDepth, rootPath, path) {
				return filepath.SkipDir
			}

			return nil
		})
		if err != nil {
			select {
			case <-ctx.Done():
			case ch <- FileItem{Error: err}:
			}
		}
	}()

	return ch, nil
}

// OSPathSeparator is the string version of the os.PathSeparator.
const OSPathSeparator = string(os.PathSeparator)

func atMaxDepth(max int, root, cur string) bool {
	if max < 0 {
		return false
	}

	if root == cur {
		// always recurse the root directory
		return false
	}

	return (strings.Count(cur, OSPathSeparator) - strings.Count(root, OSPathSeparator)) >= max
}
