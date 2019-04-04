/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package baremetal

import (
	"io/ioutil"
	"os"
	"path"

	"github.com/pkg/errors"
	"github.com/talos-systems/talos/internal/pkg/blockdevice/probe"
	"github.com/talos-systems/talos/internal/pkg/constants"
	"github.com/talos-systems/talos/internal/pkg/install"
	"github.com/talos-systems/talos/internal/pkg/kernel"
	"github.com/talos-systems/talos/pkg/userdata"

	"golang.org/x/sys/unix"

	yaml "gopkg.in/yaml.v2"
)

const (
	mnt = "/mnt"
)

// BareMetal is a discoverer for non-cloud environments.
type BareMetal struct{}

// Name implements the platform.Platform interface.
func (b *BareMetal) Name() string {
	return "Bare Metal"
}

// UserData implements the platform.Platform interface.
func (b *BareMetal) UserData() (data *userdata.UserData, err error) {
	arguments, err := kernel.ParseProcCmdline()
	if err != nil {
		return
	}

	option, ok := arguments[constants.KernelParamUserData]
	if !ok {
		return data, errors.Errorf("no user data option was found")
	}

	if option == constants.UserDataCIData {
		var dev *probe.ProbedBlockDevice
		dev, err = probe.GetDevWithFileSystemLabel(constants.UserDataCIData)
		if err != nil {
			return data, errors.Errorf("failed to find %s iso: %v", constants.UserDataCIData, err)
		}
		if err = os.Mkdir(mnt, 0700); err != nil {
			return data, errors.Errorf("failed to mkdir: %v", err)
		}
		if err = unix.Mount(dev.Path, mnt, "iso9660", unix.MS_RDONLY, ""); err != nil {
			return data, errors.Errorf("failed to mount iso: %v", err)
		}
		var dataBytes []byte
		dataBytes, err = ioutil.ReadFile(path.Join(mnt, "user-data"))
		if err != nil {
			return data, errors.Errorf("read user data: %s", err.Error())
		}
		if err = unix.Unmount(mnt, 0); err != nil {
			return data, errors.Errorf("failed to unmount: %v", err)
		}
		if err = yaml.Unmarshal(dataBytes, &data); err != nil {
			return data, errors.Errorf("unmarshal user data: %s", err.Error())
		}

		return data, nil
	}

	return userdata.Download(option, nil)
}

// Prepare implements the platform.Platform interface.
func (b *BareMetal) Prepare(data *userdata.UserData) (err error) {
	return install.Prepare(data)
}

// Install provides the functionality to install talos by
// download the necessary bits and write them to a target device
// nolint: gocyclo, dupl
func (b *BareMetal) Install(data *userdata.UserData) (err error) {
	var cmdlineBytes []byte
	cmdlineBytes, err = kernel.ReadProcCmdline()
	if err != nil {
		return err
	}
	if err = install.Install(string(cmdlineBytes), data); err != nil {
		return errors.Wrap(err, "failed to install to bare metal")
	}

	return nil
}
