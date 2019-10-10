/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

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

// RuntimeFunc returns the runtime function.
func (task *SaveConfig) RuntimeFunc(mode runtime.Mode) phase.RuntimeFunc {
	return task.runtime
}

func (task *SaveConfig) runtime(args *phase.RuntimeArgs) (err error) {
	log.Printf("saving config %s to disk\n", args.Config().Version())

	s, err := args.Config().String()
	if err != nil {
		return err
	}

	if err = ioutil.WriteFile(constants.ConfigPath, []byte(s), 0644); err != nil {
		return err
	}

	return nil
}
