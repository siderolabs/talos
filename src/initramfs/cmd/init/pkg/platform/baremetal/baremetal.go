// +build linux

package baremetal

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"github.com/autonomy/talos/src/initramfs/cmd/init/pkg/constants"
	"github.com/autonomy/talos/src/initramfs/cmd/init/pkg/kernel"
	"github.com/autonomy/talos/src/initramfs/cmd/init/pkg/mount/blkid"
	"github.com/autonomy/talos/src/initramfs/pkg/userdata"
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
func (b *BareMetal) UserData() (data userdata.UserData, err error) {
	arguments, err := kernel.ParseProcCmdline()
	if err != nil {
		return
	}

	option, ok := arguments[constants.KernelParamUserData]
	if !ok {
		return data, fmt.Errorf("no user data option was found")
	}

	if option == constants.UserDataCIData {
		var devname string
		devname, err = blkid.GetDevWithAttribute("LABEL", constants.UserDataCIData)
		if err != nil {
			return data, fmt.Errorf("failed to find %s volume: %v", constants.UserDataCIData, err)
		}
		if err = os.Mkdir(mnt, 0700); err != nil {
			return data, fmt.Errorf("failed to mkdir: %v", err)
		}
		if err = unix.Mount(devname, mnt, "iso9660", unix.MS_RDONLY, ""); err != nil {
			return data, fmt.Errorf("failed to mount: %v", err)
		}
		var dataBytes []byte
		dataBytes, err = ioutil.ReadFile(path.Join(mnt, "user-data"))
		if err != nil {
			return data, fmt.Errorf("read user data: %s", err.Error())
		}
		if err = unix.Unmount(mnt, 0); err != nil {
			return data, fmt.Errorf("failed to unmount: %v", err)
		}
		if err = yaml.Unmarshal(dataBytes, &data); err != nil {
			return data, fmt.Errorf("unmarshal user data: %s", err.Error())
		}

		return data, nil
	}

	return userdata.Download(option)
}

// Prepare implements the platform.Platform interface.
func (b *BareMetal) Prepare(data userdata.UserData) (err error) {
	return nil
}

func (b *BareMetal) Install(data userdata.UserData) (err error) {
	// No installation necessary
	if data.Install == nil {
		return nil
	}

	if data.Install.RootDevice == "" {
		return fmt.Errorf("%s", "install.rootdevice is required")
	}

	if data.Install.DataDevice == "" {
		data.Install.Device = data.Install.RootDevice
	}

	// Should probably have a canonical location to fetch rootfs - github?/s3?
	// leaving this w/o a default for now
	if data.Install.RootFSURL == "" {
		data.Install.RootFSURL = ""
	}

	// Verify that the disks are unused
	// Maybe a simple check against bd.UUID is more appropriate?
	if !data.Install.Wipe {
		for _, device := range []string{data.Install.RootDevice, data.Install.DataDevice} {
			bd := blkid.ProbeDevice(device)
			if bd.Label == "" || bd.Type == "" || bd.PartLabel == "" {
				return fmt.Errorf("%s: %s", "install device is not empty", device)
			}
		}

	}
}
