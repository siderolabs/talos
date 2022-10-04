// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package rpigeneric provides the Raspberry Pi Compute Module 4 implementation.
package rpigeneric

import (
	_ "embed"
	"os"

	"github.com/talos-systems/go-procfs/procfs"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/pkg/copy"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

//go:embed config.txt
var configTxt []byte

// RPiGeneric represents the Raspberry Pi Compute Module 4.
//
// Reference: https://www.raspberrypi.com/products/compute-module-4/
type RPiGeneric struct{}

// Name implements the runtime.Board.
func (r *RPiGeneric) Name() string {
	return constants.BoardRPiGeneric
}

// Install implements the runtime.Board.
func (r *RPiGeneric) Install(disk string) (err error) {
	err = copy.Dir("/usr/install/arm64/raspberrypi-firmware/boot", "/boot/EFI")
	if err != nil {
		return err
	}

	err = copy.File("/usr/install/arm64/u-boot/rpi_generic/u-boot.bin", "/boot/EFI/u-boot.bin")
	if err != nil {
		return err
	}

	return os.WriteFile("/boot/EFI/config.txt", configTxt, 0o600)
}

// KernelArgs implements the runtime.Board.
func (r *RPiGeneric) KernelArgs() procfs.Parameters {
	return []*procfs.Parameter{
		procfs.NewParameter("console").Append("tty0").Append("ttyAMA0,115200"),
		procfs.NewParameter("sysctl.kernel.kexec_load_disabled").Append("1"),
	}
}

// PartitionOptions implements the runtime.Board.
func (r *RPiGeneric) PartitionOptions() *runtime.PartitionOptions {
	return nil
}
