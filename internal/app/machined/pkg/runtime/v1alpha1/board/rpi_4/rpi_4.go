// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package rpi4

import (
	_ "embed" //nolint:gci
	"io/ioutil"

	"github.com/talos-systems/go-procfs/procfs"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/pkg/copy"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

//go:embed config.txt
var configTxt []byte

// RPi4 represents the Raspberry Pi 4 Model B.
//
// Reference: https://www.raspberrypi.org/products/raspberry-pi-4-model-b/
type RPi4 struct{}

// Name implements the runtime.Board.
func (r *RPi4) Name() string {
	return constants.BoardRPi4
}

// Install implements the runtime.Board.
func (r *RPi4) Install(disk string) (err error) {
	err = copy.Dir("/usr/install/arm64/raspberrypi-firmware/boot", "/boot/EFI")
	if err != nil {
		return err
	}

	err = copy.File("/usr/install/arm64/u-boot/rpi_4/u-boot.bin", "/boot/EFI/u-boot.bin")
	if err != nil {
		return err
	}

	return ioutil.WriteFile("/boot/EFI/config.txt", configTxt, 0o600)
}

// KernelArgs implements the runtime.Board.
func (r *RPi4) KernelArgs() procfs.Parameters {
	return []*procfs.Parameter{
		procfs.NewParameter("console").Append("tty0").Append("ttyAMA0,115200"),
		procfs.NewParameter("sysctl.kernel.kexec_load_disabled").Append("1"),
	}
}

// PartitionOptions implements the runtime.Board.
func (r *RPi4) PartitionOptions() *runtime.PartitionOptions {
	return nil
}
