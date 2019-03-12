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
	"strings"

	"github.com/pkg/errors"
	"github.com/talos-systems/talos/internal/pkg/blockdevice/probe"
	"github.com/talos-systems/talos/internal/pkg/constants"
	"github.com/talos-systems/talos/internal/pkg/install"
	"github.com/talos-systems/talos/internal/pkg/mount"
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
			Root: &userdata.RootDevice{
				Rootfs: "file:///rootfs.tar.gz",
				InstallDevice: userdata.InstallDevice{
					Device: "/dev/sda",
					Size:   2048 * 1000 * 1000,
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

// Prepare implements the platform.Platform interface.
func (i *ISO) Prepare(data *userdata.UserData) (err error) {
	var dev *probe.ProbedBlockDevice
	dev, err = probe.GetDevWithFileSystemLabel(constants.ISOFilesystemLabel)
	if err != nil {
		return errors.Errorf("failed to find %s iso: %v", constants.ISOFilesystemLabel, err)
	}

	mountpoint := mount.NewMountPoint(dev.Path, "/tmp", dev.SuperBlock.Type(), unix.MS_RDONLY, "")
	if err = mount.WithRetry(mountpoint); err != nil {
		return err
	}

	for _, f := range []string{"/tmp/usr/install/vmlinuz", "/tmp/usr/install/initramfs.xz", "/tmp/usr/install/rootfs.tar.gz"} {
		source, err := ioutil.ReadFile(f)
		if err != nil {
			return err
		}
		if err = ioutil.WriteFile("/"+filepath.Base(f), source, 0644); err != nil {
			return err
		}
	}

	return install.Prepare(data)
}

// Install implements the platform.Platform interface.
func (i *ISO) Install(data *userdata.UserData) error {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Talos configuration URL: ")
	endpoint, err := reader.ReadString('\n')
	if err != nil {
		return err
	}

	params := []string{
		"page_poison=1",
		"slab_nomerge",
		"pti=on",
		"nvme_core.io_timeout=4294967295",
		"consoleblank=0",
		"console=tty0",
		"console=ttyS0,9600",
		"random.trust_cpu=on",
		"talos.platform=bare-metal",
		"talos.userdata=" + endpoint,
	}

	if err := install.Install(strings.Join(params, " "), data); err != nil {
		return err
	}

	// nolint: errcheck
	unix.Reboot(int(unix.LINUX_REBOOT_CMD_RESTART))

	return nil
}
