/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package userdata

import (
	"io/ioutil"
	"log"
	"os"

	"gopkg.in/yaml.v2"

	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/app/machined/internal/platform"
	"github.com/talos-systems/talos/internal/app/machined/internal/runtime"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/userdata"
)

// SaveUserData represents the SaveUserData task.
type SaveUserData struct{}

// NewSaveUserDataTask initializes and returns an UserData task.
func NewSaveUserDataTask() phase.Task {
	return &SaveUserData{}
}

// RuntimeFunc returns the runtime function.
func (task *SaveUserData) RuntimeFunc(mode runtime.Mode) phase.RuntimeFunc {
	return task.runtime
}

func (task *SaveUserData) runtime(platform platform.Platform, data *userdata.UserData) (err error) {
	if _, err = os.Stat(constants.UserDataPath); os.IsNotExist(err) {
		log.Println("saving userdata to disk")

		var dataBytes []byte
		dataBytes, err = yaml.Marshal(data)
		if err != nil {
			return err
		}

		if err = ioutil.WriteFile(constants.UserDataPath, dataBytes, 0400); err != nil {
			return err
		}

		return nil
	}

	log.Println("refusing to overwrite userdata on disk")

	return nil
}
