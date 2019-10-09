/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package config

import (
	"io/ioutil"

	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/constants"
)

// Task represents the Task task.
type Task struct{}

// NewConfigTask initializes and returns a Task task.
func NewConfigTask() phase.Task {
	return &Task{}
}

// RuntimeFunc returns the runtime function.
func (task *Task) RuntimeFunc(mode runtime.Mode) phase.RuntimeFunc {
	return task.standard
}

func (task *Task) standard(args *phase.RuntimeArgs) (err error) {
	var b []byte

	if b, err = args.Platform().Configuration(); err != nil {
		return err
	}

	return ioutil.WriteFile(constants.ConfigPath, b, 0600)
}
