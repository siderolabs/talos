// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package rootfs

import (
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/pkg/runtime"
)

// ValidateConfig represents the ValidateConfig task.
type ValidateConfig struct{}

// NewValidateConfigTask initializes and returns a ValidateConfig task.
func NewValidateConfigTask() phase.Task {
	return &ValidateConfig{}
}

// TaskFunc returns the runtime function.
func (task *ValidateConfig) TaskFunc(mode runtime.Mode) phase.TaskFunc {
	return task.standard
}

func (task *ValidateConfig) standard(r runtime.Runtime) (err error) {
	file := "/sys/module/usb_storage/parameters/delay_use"

	_, err = os.Stat(file)
	if os.IsNotExist(err) {
		return r.Config().Validate(r.Platform().Mode())
	}

	b, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}

	val := strings.TrimSuffix(string(b), "\n")

	i, err := strconv.Atoi(val)
	if err != nil {
		return err
	}

	time.Sleep(time.Duration(i) * time.Second)

	return r.Config().Validate(r.Platform().Mode())
}
