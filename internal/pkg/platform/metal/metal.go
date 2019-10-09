/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package metal

import (
	"io/ioutil"
	"net"
	"os"
	"path"

	"github.com/pkg/errors"
	"golang.org/x/sys/unix"

	"github.com/talos-systems/talos/internal/pkg/kernel"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/blockdevice/probe"
	"github.com/talos-systems/talos/pkg/config"
	"github.com/talos-systems/talos/pkg/constants"
)

const (
	mnt = "/mnt"
)

// Metal is a discoverer for non-cloud environments.
type Metal struct{}

// Name implements the platform.Platform interface.
func (b *Metal) Name() string {
	return "Metal"
}

// Configuration implements the platform.Platform interface.
func (b *Metal) Configuration() ([]byte, error) {
	var option *string
	if option = kernel.ProcCmdline().Get(constants.KernelParamConfig).First(); option == nil {
		return nil, errors.Errorf("no config option was found")
	}

	if *option == constants.UserDataCIData {
		var dev *probe.ProbedBlockDevice

		dev, err := probe.GetDevWithFileSystemLabel(constants.UserDataCIData)
		if err != nil {
			return nil, errors.Errorf("failed to find %s iso: %v", constants.UserDataCIData, err)
		}

		if err = os.Mkdir(mnt, 0700); err != nil {
			return nil, errors.Errorf("failed to mkdir: %v", err)
		}

		if err = unix.Mount(dev.Path, mnt, dev.SuperBlock.Type(), unix.MS_RDONLY, ""); err != nil {
			return nil, errors.Errorf("failed to mount iso: %v", err)
		}

		var b []byte

		b, err = ioutil.ReadFile(path.Join(mnt, "user-data"))
		if err != nil {
			return nil, errors.Errorf("read config: %s", err.Error())
		}

		if err = unix.Unmount(mnt, 0); err != nil {
			return nil, errors.Errorf("failed to unmount: %v", err)
		}

		return b, nil
	}

	return config.Download(*option)
}

// Mode implements the platform.Platform interface.
func (b *Metal) Mode() runtime.Mode {
	return runtime.Metal
}

// Hostname implements the platform.Platform interface.
func (b *Metal) Hostname() (hostname []byte, err error) {
	return nil, nil
}

// ExternalIPs provides any external addresses assigned to the instance
func (b *Metal) ExternalIPs() (addrs []net.IP, err error) {
	return addrs, err
}
