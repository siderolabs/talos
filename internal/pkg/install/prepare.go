/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package install

import (
	"io"
	"log"
	"os"
	"runtime"
	"strconv"

	"github.com/autonomy/talos/internal/pkg/blockdevice"
	"github.com/autonomy/talos/internal/pkg/blockdevice/filesystem/xfs"
	"github.com/autonomy/talos/internal/pkg/blockdevice/probe"
	"github.com/autonomy/talos/internal/pkg/blockdevice/table"
	"github.com/autonomy/talos/internal/pkg/blockdevice/table/gpt/partition"
	"github.com/autonomy/talos/internal/pkg/constants"
	"github.com/autonomy/talos/internal/pkg/userdata"
	"github.com/autonomy/talos/internal/pkg/version"
	"github.com/pkg/errors"
)

// Prepare handles setting/consolidating/defaulting userdata pieces specific to
// installation
// TODO: See if this would be more appropriate in userdata
// nolint: dupl, gocyclo
func Prepare(data *userdata.UserData) (err error) {
	if data.Install == nil {
		return nil
	}

	// Root Device Init
	if data.Install.Root.Device == "" {
		return errors.Errorf("%s", "install.rootdevice is required")
	}

	if data.Install.Root.Size == 0 {
		// Set to 1G default for funzies
		data.Install.Root.Size = 2048 * 1000 * 1000
	}

	if len(data.Install.Root.Data) == 0 {
		// Should probably have a canonical location to fetch rootfs - github?/s3?
		// need to figure out how to download latest instead of hardcoding
		data.Install.Root.Data = append(data.Install.Root.Data, "https://github.com/autonomy/talos/releases/download/"+version.Tag+"/rootfs.tar.gz")
	}

	// Data Device Init
	if data.Install.Data.Device == "" {
		data.Install.Data.Device = data.Install.Root.Device
	}

	if data.Install.Data.Size == 0 {
		// Set to 1G default for funzies
		data.Install.Data.Size = 1024 * 1000 * 1000
	}

	// Boot Device Init
	if data.Install.Boot != nil {
		if data.Install.Boot.Device == "" {
			data.Install.Boot.Device = data.Install.Root.Device
		}
		if data.Install.Boot.Size == 0 {
			// Set to 512MB default for funzies
			data.Install.Boot.Size = 512 * 1000 * 1000
		}
		if len(data.Install.Boot.Data) == 0 {
			data.Install.Boot.Data = append(data.Install.Boot.Data, "https://github.com/autonomy/talos/releases/download/"+version.Tag+"/vmlinuz")
			data.Install.Boot.Data = append(data.Install.Boot.Data, "https://github.com/autonomy/talos/releases/download/"+version.Tag+"/initramfs.xz")

		}
	}

	// Verify that the disks are unused
	// Maybe a simple check against bd.UUID is more appropriate?
	if !data.Install.Wipe {
		var dev *probe.ProbedBlockDevice
		for _, device := range []string{data.Install.Boot.Device, data.Install.Root.Device, data.Install.Data.Device} {
			dev, err = probe.GetDevWithFileSystemLabel(device)
			if err != nil {
				// We continue here because we only care if we can discover the
				// device successfully and confirm that the disk is not in use.
				// TODO(andrewrynhard): We should return a custom error type here
				// that we can use to confirm the device was not found.
				continue
			}
			if dev.SuperBlock != nil {
				return errors.Errorf("target install device %s is not empty, found existing %s file system", device, dev.SuperBlock.Type())
			}
		}
	}

	// Create a map of all the devices we need to be concerned with
	devices := make(map[string]*Device)
	labeldev := make(map[string]string)

	// PR: Should we only allow boot device creation if data.Install.Wipe?
	if data.Install.Boot.Device != "" {
		devices[constants.BootPartitionLabel] = NewDevice(data.Install.Boot.Device,
			constants.BootPartitionLabel,
			data.Install.Boot.Size,
			data.Install.Wipe,
			false,
			data.Install.Boot.Data)
		labeldev[constants.BootPartitionLabel] = data.Install.Boot.Device
	}

	devices[constants.RootPartitionLabel] = NewDevice(data.Install.Root.Device,
		constants.RootPartitionLabel,
		data.Install.Root.Size,
		data.Install.Wipe,
		false,
		data.Install.Root.Data)
	labeldev[constants.RootPartitionLabel] = data.Install.Root.Device

	devices[constants.DataPartitionLabel] = NewDevice(data.Install.Data.Device,
		constants.DataPartitionLabel,
		data.Install.Data.Size,
		data.Install.Wipe,
		false,
		data.Install.Data.Data)

	labeldev[constants.DataPartitionLabel] = data.Install.Data.Device

	if data.Install.Wipe {
		log.Println("Preparing to zero out devices")
		var zero *os.File
		zero, err = os.Open("/dev/zero")
		if err != nil {
			return err
		}

		log.Println("Calculating total disk usage")
		diskSizes := make(map[string]uint, len(devices))
		for _, dev := range devices {
			// Adding 264*512b to cover partition table size
			// In theory, a GUID Partition Table disk can be up to 264 sectors in a single logical block in length.
			// Logical blocks are commonly 512 bytes or one sector in size.
			// TODO verify this against gpt.go
			diskSizes[dev.Name] += dev.Size + 164010
		}

		log.Println("Zeroing out each disk")
		var f *os.File
		for dev, size := range diskSizes {
			f, err = os.OpenFile(dev, os.O_RDWR, os.ModeDevice)
			if err != nil {
				return err
			}

			if _, err = io.CopyN(f, zero, int64(size)); err != nil {
				return err
			}

			if err = f.Close(); err != nil {
				return err
			}
		}

		if err = zero.Close(); err != nil {
			return err
		}
	}

	// Use the below to only open a block device once
	uniqueDevices := make(map[string]*blockdevice.BlockDevice)

	// Associate block device to a partition table. This allows us to
	// make use of a single partition table across an entire block device.
	log.Println("Opening block devices in preparation for partitioning")
	partitionTables := make(map[*blockdevice.BlockDevice]table.PartitionTable)
	for label, device := range labeldev {
		if dev, ok := uniqueDevices[device]; ok {
			devices[label].BlockDevice = dev
			devices[label].PartitionTable = partitionTables[dev]
			continue
		}

		var bd *blockdevice.BlockDevice

		bd, err = blockdevice.Open(device, blockdevice.WithNewGPT(data.Install.Wipe))
		if err != nil {
			return err
		}

		// nolint: errcheck
		defer bd.Close()

		var pt table.PartitionTable
		pt, err = bd.PartitionTable(!data.Install.Wipe)
		if err != nil {
			return err
		}

		uniqueDevices[device] = bd
		partitionTables[bd] = pt

		devices[label].BlockDevice = bd
		devices[label].PartitionTable = pt
	}

	// devices = Device
	if data.Install.Wipe {
		for _, label := range []string{constants.BootPartitionLabel, constants.RootPartitionLabel, constants.DataPartitionLabel} {
			// Wipe disk
			// Partition the disk
			log.Printf("Partitioning %s - %s\n", devices[label].Name, label)
			err = devices[label].Partition()
			if err != nil {
				return err
			}
		}
	}

	// Installation/preparation necessary
	if data.Install != nil {

		// uniqueDevices = blockdevice
		seen := make(map[string]interface{})
		for _, dev := range devices {
			if _, ok := seen[dev.Name]; ok {
				continue
			}
			seen[dev.Name] = nil

			err = dev.PartitionTable.Write()
			if err != nil {
				return err
			}

			// Create the device files
			log.Printf("Reread Partition Table %s\n", dev.Name)
			if err = dev.BlockDevice.RereadPartitionTable(); err != nil {
				log.Println("break here?")
				return err
			}

		}

		for _, dev := range devices {
			// Create the filesystem
			log.Printf("Formatting Partition %s - %s\n", dev.Name, dev.Label)
			err = dev.Format()
			if err != nil {
				return err
			}
		}
	}

	return err
}

// Device represents a single partition.
type Device struct {
	DataURLs  []string
	Label     string
	MountBase string
	Name      string

	// This seems overkill to save partition table
	// when we can get partition table from BlockDevice
	// but we want to have a shared partition table for each
	// device so we can properly append partitions and have
	// an atomic write partition operation
	PartitionTable table.PartitionTable

	// This guy might be overkill but we can clean up later
	// Made up of Name + part.No(), so maybe it's worth
	// just storing part.No() and adding a method d.PartName()
	PartitionName string

	Size uint

	BlockDevice *blockdevice.BlockDevice

	Force bool
	Test  bool
}

// NewDevice creates a Device with basic metadata. BlockDevice and PartitionTable
// need to be set outsite of this.
func NewDevice(name string, label string, size uint, force bool, test bool, data []string) *Device {
	return &Device{
		DataURLs:  data,
		Force:     force,
		Label:     label,
		MountBase: "/tmp",
		Name:      name,
		Size:      size,
		Test:      test,
	}
}

// Partition creates a new partition on the specified device
// nolint: dupl
func (d *Device) Partition() error {
	var typeID string
	switch d.Label {
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
			return errors.Errorf("%s", "unsupported cpu architecture")
		}
	case constants.DataPartitionLabel:
		// Data Partition
		typeID = "AF3DC60F-8384-7247-8E79-3D69D8477DE4"
	default:
		return errors.Errorf("%s", "unknown partition label")
	}

	part, err := d.PartitionTable.Add(uint64(d.Size), partition.WithPartitionType(typeID), partition.WithPartitionName(d.Label), partition.WithPartitionTest(d.Test))
	if err != nil {
		return err
	}

	d.PartitionName = d.Name + strconv.Itoa(int(part.No()))

	return nil
}

// Format creates a xfs filesystem on the device/partition
func (d *Device) Format() error {
	return xfs.MakeFS(d.PartitionName, xfs.WithLabel(d.Label), xfs.WithForce(d.Force))
}
