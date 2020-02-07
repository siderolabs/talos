// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/go-multierror"
	"golang.org/x/sys/unix"

	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/pkg/runtime"
)

// ExtraFiles represents the ExtraFiles task.
type ExtraFiles struct{}

// NewExtraFilesTask initializes and returns an ExtraFiles task.
func NewExtraFilesTask() phase.Task {
	return &ExtraFiles{}
}

// TaskFunc returns the runtime function.
func (task *ExtraFiles) TaskFunc(mode runtime.Mode) phase.TaskFunc {
	return task.runtime
}

// nolint: gocyclo
func (task *ExtraFiles) runtime(r runtime.Runtime) (err error) {
	var result *multierror.Error

	files, err := r.Config().Machine().Files()
	if err != nil {
		return fmt.Errorf("error generating extra files: %w", err)
	}

	for _, f := range files {
		content := f.Content

		switch f.Op {
		case "create":
			if err = doesNotExists(f.Path); err != nil {
				result = multierror.Append(result, fmt.Errorf("file must not exist: %q", f.Path))
				continue
			}
		case "overwrite":
			if err = existsAndIsFile(f.Path); err != nil {
				result = multierror.Append(result, err)
				continue
			}
		case "append":
			if err = existsAndIsFile(f.Path); err != nil {
				result = multierror.Append(result, err)
				continue
			}

			var existingFileContents []byte

			existingFileContents, err = ioutil.ReadFile(f.Path)
			if err != nil {
				result = multierror.Append(result, err)
				continue
			}

			content = string(existingFileContents) + "\n" + f.Content
		default:
			result = multierror.Append(result, fmt.Errorf("unknown operation for file %q: %q", f.Path, f.Op))
			continue
		}

		// Determine if supplied path is in /var or not.
		// If not, we'll write it to /var anyways and bind mount below
		p := f.Path
		inVar := true
		explodedPath := strings.Split(
			strings.TrimLeft(f.Path, "/"),
			string(os.PathSeparator),
		)

		if explodedPath[0] != "var" {
			p = filepath.Join("/var", f.Path)
			inVar = false
		}

		// We do not want to support creating new files anywhere outside of
		// /var. If a valid use case comes up, we can reconsider then.
		if !inVar && f.Op == "create" {
			return fmt.Errorf("create operation not allowed outside of /var: %q", f.Path)
		}

		if err = os.MkdirAll(filepath.Dir(p), os.ModeDir); err != nil {
			result = multierror.Append(result, err)
			continue
		}

		if err = ioutil.WriteFile(p, []byte(content), f.Permissions); err != nil {
			result = multierror.Append(result, err)
			continue
		}

		// File path was not /var/... so we assume a bind mount is wanted
		if !inVar {
			if err = unix.Mount(p, f.Path, "", unix.MS_BIND|unix.MS_RDONLY, ""); err != nil {
				result = multierror.Append(result, fmt.Errorf("failed to create bind mount for %s: %w", p, err))
			}
		}
	}

	return result.ErrorOrNil()
}

func doesNotExists(p string) (err error) {
	_, err = os.Stat(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return err
	}

	return fmt.Errorf("file exists")
}

func existsAndIsFile(p string) (err error) {
	var info os.FileInfo

	info, err = os.Stat(p)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}

		return fmt.Errorf("file must exist: %q", p)
	}

	if !info.Mode().IsRegular() {
		return fmt.Errorf("invalid mode: %q", info.Mode().String())
	}

	return nil
}
