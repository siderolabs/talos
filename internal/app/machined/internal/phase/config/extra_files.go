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

func (task *ExtraFiles) runtime(r runtime.Runtime) (err error) {
	var result *multierror.Error

	for _, f := range r.Config().Machine().Files() {
		// Slurp existing file if append is our op and add contents to it
		if f.Op == "append" {
			var existingFileContents []byte

			existingFileContents, err = ioutil.ReadFile(f.Path)
			if err != nil {
				result = multierror.Append(result, err)
				continue
			}

			f.Contents = string(existingFileContents) + "\n" + f.Contents
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

		if err = os.MkdirAll(filepath.Dir(p), os.ModeDir); err != nil {
			result = multierror.Append(result, err)
			continue
		}

		if err = ioutil.WriteFile(p, []byte(f.Contents), f.Permissions); err != nil {
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
