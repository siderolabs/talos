/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package config

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/hashicorp/go-multierror"

	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/pkg/runtime"
)

// ExtraFiles represents the ExtraFiles task.
type ExtraFiles struct{}

// NewExtraFilesTask initializes and returns an ExtraFiles task.
func NewExtraFilesTask() phase.Task {
	return &ExtraFiles{}
}

// RuntimeFunc returns the runtime function.
func (task *ExtraFiles) RuntimeFunc(mode runtime.Mode) phase.RuntimeFunc {
	return task.runtime
}

func (task *ExtraFiles) runtime(args *phase.RuntimeArgs) (err error) {
	var result *multierror.Error

	for _, f := range args.Config().Machine().Files() {
		p := filepath.Join("/var", f.Path)
		if err = os.MkdirAll(filepath.Dir(p), os.ModeDir); err != nil {
			result = multierror.Append(result, err)
		}

		if err = ioutil.WriteFile(p, []byte(f.Contents), f.Permissions); err != nil {
			result = multierror.Append(result, err)
		}
	}

	return result.ErrorOrNil()
}
