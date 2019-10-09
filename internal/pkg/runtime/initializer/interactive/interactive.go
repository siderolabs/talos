/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package interactive

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"golang.org/x/sys/unix"

	"github.com/talos-systems/talos/internal/pkg/installer"
	"github.com/talos-systems/talos/internal/pkg/kernel"
	"github.com/talos-systems/talos/internal/pkg/platform"
	"github.com/talos-systems/talos/pkg/blockdevice/probe"
	"github.com/talos-systems/talos/pkg/config/machine"
	"github.com/talos-systems/talos/pkg/constants"
)

// Interactive is an initializer that performs an installation by prompting the
// user.
type Interactive struct{}

// Initialize implements the Initializer interface.
func (i *Interactive) Initialize(platform platform.Platform, install machine.Install) (err error) {
	var dev *probe.ProbedBlockDevice

	dev, err = probe.GetDevWithFileSystemLabel(constants.ISOFilesystemLabel)
	if err != nil {
		return errors.Errorf("failed to find %s iso: %v", constants.ISOFilesystemLabel, err)
	}

	if err = unix.Mount(dev.Path, "/tmp", dev.SuperBlock.Type(), unix.MS_RDONLY, ""); err != nil {
		return err
	}

	for _, f := range []string{"/tmp/usr/install/vmlinuz", "/tmp/usr/install/initramfs.xz"} {
		var source []byte

		source, err = ioutil.ReadFile(f)
		if err != nil {
			return err
		}

		if err = ioutil.WriteFile("/"+filepath.Base(f), source, 0644); err != nil {
			return err
		}
	}

	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Talos configuration URL: ")

	endpoint, err := reader.ReadString('\n')
	if err != nil {
		return err
	}

	cmdline := kernel.NewDefaultCmdline()
	cmdline.Append("initrd", filepath.Join("/", "default", constants.InitramfsAsset))
	cmdline.Append(constants.KernelParamPlatform, strings.ToLower(platform.Name()))
	cmdline.Append(constants.KernelParamConfig, endpoint)

	var inst *installer.Installer

	inst, err = installer.NewInstaller(cmdline, install)
	if err != nil {
		return err
	}

	if err = inst.Install(); err != nil {
		return errors.Wrap(err, "failed to install")
	}

	return unix.Reboot(int(unix.LINUX_REBOOT_CMD_RESTART))
}
