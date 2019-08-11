/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package iso

import (
	"github.com/talos-systems/talos/internal/app/machined/internal/runtime"
	"github.com/talos-systems/talos/pkg/crypto/x509"
	"github.com/talos-systems/talos/pkg/userdata"
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
			Ephemeral: &userdata.InstallDevice{
				Device: "/dev/sda",
				Size:   2048 * 1000 * 1000,
			},
		},
	}

	return data, nil
}

// Mode implements the platform.Platform interface.
func (i *ISO) Mode() runtime.Mode {
	return runtime.Interactive
}

// Hostname implements the platform.Platform interface.
func (i *ISO) Hostname() (hostname []byte, err error) {
	return nil, nil
}
