// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import (
	"io/ioutil"
	"log"

	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/constants"
)

// SaveConfig represents the SaveConfig task.
type SaveConfig struct{}

// NewSaveConfigTask initializes and returns an Config task.
func NewSaveConfigTask() phase.Task {
	return &SaveConfig{}
}

// TaskFunc returns the runtime function.
func (task *SaveConfig) TaskFunc(mode runtime.Mode) phase.TaskFunc {
	return task.runtime
}

func (task *SaveConfig) runtime(r runtime.Runtime) (err error) {
	log.Printf("saving config %s to disk\n", r.Config().Version())

	b, err := r.Config().Bytes()
	if err != nil {
		return err
	}

	if err = ioutil.WriteFile(constants.ConfigPath, b, 0644); err != nil {
		return err
	}

	return nil
}
