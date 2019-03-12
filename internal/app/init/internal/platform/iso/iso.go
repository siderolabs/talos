/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package iso

import (
	"github.com/autonomy/talos/internal/pkg/crypto/x509"
	"github.com/autonomy/talos/internal/pkg/install"
	"github.com/autonomy/talos/internal/pkg/userdata"
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
			Boot: &userdata.InstallDevice{
				Device: "/dev/sda",
				Size:   512 * 1000 * 1000,
			},
			Root: &userdata.InstallDevice{
				Device: "/dev/sda",
				Size:   2048 * 1000 * 1000,
			},
			Data: &userdata.InstallDevice{
				Device: "/dev/sda",
				Size:   2048 * 1000 * 1000,
			},
			Wipe: true,
		},
	}

	return data, nil
}

// Prepare implements the platform.Platform interface.
func (i *ISO) Prepare(data *userdata.UserData) (err error) {
	return install.Prepare(data)
}

// Install implements the platform.Platform interface.
func (i *ISO) Install(data *userdata.UserData) error {
	params := "page_poison=1 slab_nomerge pti=on nvme_core.io_timeout=4294967295 consoleblank=0 console=tty0 console=ttyS0,9600 random.trust_cpu=on talos.autonomy.io/platform=bare-metal talos.autonomy.io/userdata=http://192.168.124.100:8080/master.yaml"
	if err := install.Install(params, data); err != nil {
		return err
	}
	unix.Reboot(int(unix.LINUX_REBOOT_CMD_RESTART))

	return nil
}
