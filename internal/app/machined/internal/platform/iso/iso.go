/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package iso

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/talos-systems/talos/internal/pkg/blockdevice/probe"
	"github.com/talos-systems/talos/internal/pkg/constants"
	"github.com/talos-systems/talos/internal/pkg/installer"
	"github.com/talos-systems/talos/internal/pkg/kernel"
	"github.com/talos-systems/talos/pkg/crypto/x509"
	"github.com/talos-systems/talos/pkg/userdata"
	"golang.org/x/sys/unix"
)

// ISO is a platform for installing Talos via an ISO image.
type ISO struct{}

// Name implements the platform.Platform interface.
func (i *ISO) Name() string {
	return "ISO"
}

// UserData implements the platform.Platform interface.
func (i *ISO) UserData() (data *userdata.UserData, err error) {
	data = &userdata.UserData{
		Security: &userdata.Security{
			OS: &userdata.OSSecurity{
				CA:       &x509.PEMEncodedCertificateAndKey{},
				Identity: &x509.PEMEncodedCertificateAndKey{},
			},
			Kubernetes: &userdata.KubernetesSecurity{
				CA: &x509.PEMEncodedCertificateAndKey{},
			},
		},
		Install: &userdata.Install{
			Force: true,
			Boot: &userdata.BootDevice{
				Kernel:    "file:///vmlinuz",
				Initramfs: "file:///initramfs.xz",
				InstallDevice: userdata.InstallDevice{
					Device: "/dev/sda",
					Size:   512 * 1000 * 1000,
				},
			},
			Data: &userdata.InstallDevice{
				Device: "/dev/sda",
				Size:   2048 * 1000 * 1000,
			},
		},
	}

	return data, nil
}

// Initialize implements the platform.Platform interface.
func (i *ISO) Initialize(data *userdata.UserData) (err error) {
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
	cmdline.Append("initrd", filepath.Join("/", "default", "initramfs.xz"))
	cmdline.Append(constants.KernelParamPlatform, "bare-metal")
	cmdline.Append(constants.KernelParamUserData, endpoint)

	inst := installer.NewInstaller(cmdline, data)
	if err = inst.Install(); err != nil {
		return errors.Wrap(err, "failed to install")
	}

	// nolint: errcheck
	unix.Reboot(int(unix.LINUX_REBOOT_CMD_RESTART))

	return nil
}
