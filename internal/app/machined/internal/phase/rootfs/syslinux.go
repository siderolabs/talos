// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package rootfs

import (
	"log"
	"os"

	"github.com/talos-systems/talos/cmd/installer/pkg/bootloader/syslinux"
	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/pkg/runtime"
)

// Syslinux represents the Syslinux task.
type Syslinux struct{}

// NewSyslinuxTask initializes and returns a ClearOnce task.
func NewSyslinuxTask() phase.Task {
	return &Syslinux{}
}

// TaskFunc returns the runtime function.
func (task *Syslinux) TaskFunc(mode runtime.Mode) phase.TaskFunc {
	switch mode {
	case runtime.Container:
		return nil
	default:
		return task.standard
	}
}

func (task *Syslinux) standard(r runtime.Runtime) (err error) {
	f, err := os.OpenFile(syslinux.SyslinuxLdlinux, os.O_RDWR, 0700)
	if err != nil {
		return err
	}

	// nolint: errcheck
	defer f.Close()

	adv, err := syslinux.NewADV(f)
	if err != nil {
		return err
	}

	if ok := adv.DeleteTag(syslinux.AdvUpgrade); ok {
		log.Println("removing fallback")
	}

	if _, err = f.Write(adv); err != nil {
		return err
	}

	return nil
}
