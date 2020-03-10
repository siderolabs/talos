// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package rootfs

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"regexp"

	"github.com/talos-systems/talos/cmd/installer/pkg/bootloader/syslinux"
	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/constants"
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
	f, err := os.OpenFile(constants.SyslinuxLdlinux, os.O_RDWR, 0700)
	if err != nil {
		return err
	}

	// nolint: errcheck
	defer f.Close()

	adv, err := syslinux.NewADV(f)
	if err != nil {
		return err
	}

	once, ok := adv.ReadTag(syslinux.AdvUpgrade)
	if !ok {
		return nil
	}

	log.Printf("updating default boot to %q", once)

	var b []byte

	if b, err = ioutil.ReadFile(constants.SyslinuxConfig); err != nil {
		return err
	}

	re := regexp.MustCompile(`^DEFAULT\s(.*)`)
	matches := re.FindSubmatch(b)

	if len(matches) != 2 {
		return fmt.Errorf("expected 2 matches, got %d", len(matches))
	}

	b = re.ReplaceAll(b, []byte(fmt.Sprintf("DEFAULT %s", once)))

	if err = ioutil.WriteFile(constants.SyslinuxConfig, b, 0600); err != nil {
		return err
	}

	adv.DeleteTag(syslinux.AdvUpgrade)

	if _, err = f.Write(adv); err != nil {
		return err
	}

	return nil
}
