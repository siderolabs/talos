// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package grub

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/talos-systems/go-blockdevice/blockdevice"

	"github.com/talos-systems/talos/pkg/machinery/constants"
)

const (
	amd64 = "amd64"
	arm64 = "arm64"
)

// Install validates the grub configuration and writes it to the disk.
//
//nolint:gocyclo
func (c *Config) Install(bootDisk, arch string) error {
	if err := c.Write(ConfigPath); err != nil {
		return err
	}

	blk, err := getBlockDeviceName(bootDisk)
	if err != nil {
		return err
	}

	loopDevice := strings.HasPrefix(blk, "/dev/loop")

	var platforms []string

	switch arch {
	case amd64:
		platforms = []string{"x86_64-efi", "i386-pc"}
	case arm64:
		platforms = []string{"arm64-efi"}
	}

	if runtime.GOARCH == amd64 && arch == amd64 && !loopDevice {
		// let grub choose the platform automatically if not building an image
		platforms = []string{""}
	}

	for _, platform := range platforms {
		args := []string{"--boot-directory=" + constants.BootMountPoint, "--efi-directory=" +
			constants.EFIMountPoint, "--removable"}

		if loopDevice {
			args = append(args, "--no-nvram")
		}

		if platform != "" {
			args = append(args, "--target="+platform)
		}

		args = append(args, blk)

		log.Printf("executing: grub-install %s", strings.Join(args, " "))

		cmd := exec.Command("grub-install", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err = cmd.Run(); err != nil {
			return fmt.Errorf("failed to install grub: %w", err)
		}
	}

	return nil
}

func getBlockDeviceName(bootDisk string) (string, error) {
	dev, err := blockdevice.Open(bootDisk)
	if err != nil {
		return "", err
	}

	//nolint:errcheck
	defer dev.Close()

	// verify that BootDisk has boot partition
	_, err = dev.GetPartition(constants.BootPartitionLabel)
	if err != nil {
		return "", err
	}

	blk := dev.Device().Name()

	return blk, nil
}
