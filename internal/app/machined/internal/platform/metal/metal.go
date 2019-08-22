/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package metal

import (
	"io/ioutil"
	"os"
	"path"

	"github.com/pkg/errors"
	"github.com/talos-systems/talos/internal/app/machined/internal/runtime"
	"github.com/talos-systems/talos/internal/pkg/kernel"
	"github.com/talos-systems/talos/pkg/blockdevice/probe"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/userdata"
	"github.com/talos-systems/talos/pkg/userdata/download"

	"golang.org/x/sys/unix"

	yaml "gopkg.in/yaml.v2"
)

const (
	mnt = "/mnt"
)

// Metal is a discoverer for non-cloud environments.
type Metal struct{}

// Name implements the platform.Platform interface.
func (b *Metal) Name() string {
	return "Bare Metal"
}

// UserData implements the platform.Platform interface.
func (b *Metal) UserData() (data *userdata.UserData, err error) {
	var option *string
	if option = kernel.ProcCmdline().Get(constants.KernelParamUserData).First(); option == nil {
		return data, errors.Errorf("no user data option was found")
	}

	if *option == constants.UserDataCIData {
		var dev *probe.ProbedBlockDevice
		dev, err = probe.GetDevWithFileSystemLabel(constants.UserDataCIData)
		if err != nil {
			return data, errors.Errorf("failed to find %s iso: %v", constants.UserDataCIData, err)
		}
		if err = os.Mkdir(mnt, 0700); err != nil {
			return data, errors.Errorf("failed to mkdir: %v", err)
		}
		if err = unix.Mount(dev.Path, mnt, dev.SuperBlock.Type(), unix.MS_RDONLY, ""); err != nil {
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

	return download.Download(*option)
}

// Mode implements the platform.Platform interface.
func (b *Metal) Mode() runtime.Mode {
	return runtime.Metal
}

// Hostname implements the platform.Platform interface.
func (b *Metal) Hostname() (hostname []byte, err error) {
	return nil, nil
}
