// +build linux

package baremetal

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"runtime"

	"github.com/autonomy/talos/src/initramfs/cmd/init/pkg/constants"
	"github.com/autonomy/talos/src/initramfs/cmd/init/pkg/kernel"
	"github.com/autonomy/talos/src/initramfs/cmd/init/pkg/mount/blkid"
	"github.com/autonomy/talos/src/initramfs/pkg/blockdevice"
	"github.com/autonomy/talos/src/initramfs/pkg/blockdevice/table"
	"github.com/autonomy/talos/src/initramfs/pkg/blockdevice/table/gpt/partition"
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

	if data.Install.RootSize == 0 {
		// Set to 1G default for funzies
		data.Install.RootSize = 1024 * 1000 * 1000 * 1000
	}

	if data.Install.DataDevice == "" {
		data.Install.Device = data.Install.RootDevice
	}

	if data.Install.DataSize == 0 {
		// Set to 1G default for funzies
		data.Install.DataSize = 1024 * 1000 * 1000 * 1000
	}

	if data.Install.BootSize == 0 {
		// Set to 1G default for funzies
		data.Install.BootSize = 1024 * 1000 * 1000 * 1000
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
				return fmt.Errorf("%s: %s", "target install device is not empty", device)
			}
		}

	}

	uniqueDevices := make(map[string]table.PartitionTable)
	for _, device := range []string{data.Install.RootDevice, data.Install.DataDevice} {
		if _, ok := uniqueDevices[device]; !ok {

			bd, err := blockdevice.Open(device)
			if err != nil {
				return err
			}
		}

		// Ignore errors here since they're most likely from a partition
		// table not existing yet
		// Only read partition table if we're not going to wipe
		pt, _ := bd.PartitionTable(!data.Install.Wipe)
		uniqueDevices[device] = pt
	}

	var err error
	if data.Install.BootDevice != "" {
		// Create boot partition
		err = partitionDisk(uniqueDevices[data.Install.BootDevice], data.Install.BootSize, constants.BootPartitionLabel)
		if err != nil {
			return err
		}
	}

	// Create root partition
	err = partitionDisk(uniqueDevices[data.Install.RootDevice], data.Install.RootSize, constants.RootPartitionLabel)
	if err != nil {
		return err
	}

	// Create data partition
	err = partitionDisk(uniqueDevices[data.Install.DataDevice], data.Install.DataSize, constants.DataPartitionLabel)
	if err != nil {
		return err
	}

	// Reread partition table?

	// Close bd // drop uniqueDevices

	// look up actual device name ( data.Install.xxxDevice + partition.No ? )

	// format disk

	// download / copy kernel bits to boot

	// download / extract rootfsurl

	// handle data dirs creation
}

func partitionDisk(device table.PartitionTable, size uint, name string) error {
	var typeID string
	switch name {
	case constants.BootPartitionLabel:
		// EFI System Partition
		typeID = "C12A7328-F81F-11D2-BA4B-00A0C93EC93B"
	case constants.RootPartitionLabel:
		// Root Partition
		switch runtime.GOARCH {
		case "386":
			typeID = "44479540-F297-41B2-9AF7-D131D5F0458A"
		case "amd64":
			typeID = "4F68BCE3-E8CD-4DB1-96E7-FBCAF984B709"
		default:
			return fmt.Errorf("%s", "unsupported cpu architecture")
		}
	case constants.DataPartitionLabel:
		// Data Partition
		typeID = "AF3DC60F-8384-7247-8E79-3D69D8477DE4"
	default:
		return fmt.Errorf("%s", "unknown partition label")
	}

	_, err := devices[target].Add(size, partition.Options.WithPartitionType(typeID), partition.Options.WithPartitionName(name))
	return err
}

func formatPartition() error {

}
