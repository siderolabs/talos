// +build linux

package baremetal

import (
	"archive/tar"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/autonomy/talos/src/initramfs/cmd/init/pkg/constants"
	"github.com/autonomy/talos/src/initramfs/cmd/init/pkg/fs/xfs"
	"github.com/autonomy/talos/src/initramfs/cmd/init/pkg/kernel"
	"github.com/autonomy/talos/src/initramfs/cmd/init/pkg/mount"
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

// Install provides the functionality to install talos by
// download the necessary bits and write them to a target device
// nolint: gocyclo
func (b *BareMetal) Install(data userdata.UserData) error {
	var err error

	// No installation necessary
	if data.Install == nil {
		return err
	}

	// Root Device Init
	if data.Install.Root.Device == "" {
		return fmt.Errorf("%s", "install.rootdevice is required")
	}

	if data.Install.Root.Size == 0 {
		// Set to 1G default for funzies
		data.Install.Root.Size = 1024 * 1000 * 1000 * 1000
	}

	if len(data.Install.Root.Data) == 0 {
		// Should probably have a canonical location to fetch rootfs - github?/s3?
		// leaving this w/o a default for now
		data.Install.Root.Data = append(data.Install.Root.Data, "https://github.com/autonomy/talos/releases/download/v0.1.0-alpha.13/rootfs.tar.xz")
	}

	// Data Device Init
	if data.Install.Data.Device == "" {
		data.Install.Data.Device = data.Install.Root.Device
	}

	if data.Install.Data.Size == 0 {
		// Set to 1G default for funzies
		data.Install.Data.Size = 1024 * 1000 * 1000 * 1000
	}

	if len(data.Install.Data.Data) == 0 {
		// Unsure if these are the real files or not, but gives an idea
		data.Install.Data.Data = append(data.Install.Data.Data, "https://github.com/autonomy/talos/releases/download/v0.1.0-alpha.13/blockd.tar")
		data.Install.Data.Data = append(data.Install.Data.Data, "https://github.com/autonomy/talos/releases/download/v0.1.0-alpha.13/osd.tar")
		data.Install.Data.Data = append(data.Install.Data.Data, "https://github.com/autonomy/talos/releases/download/v0.1.0-alpha.13/proxyd.tar")
		data.Install.Data.Data = append(data.Install.Data.Data, "https://github.com/autonomy/talos/releases/download/v0.1.0-alpha.13/trustd.tar")

	}

	// Boot Device Init
	if data.Install.Boot != nil {
		if data.Install.Boot.Device == "" {
			data.Install.Boot.Device = data.Install.Root.Device
		}
		if data.Install.Boot.Size == 0 {
			// Set to 1G default for funzies
			data.Install.Boot.Size = 1024 * 1000 * 1000 * 1000
		}
		if len(data.Install.Data.Data) == 0 {
			data.Install.Boot.Data = append(data.Install.Boot.Data, "https://github.com/autonomy/talos/releases/download/v0.1.0-alpha.13/vmlinuz")
			data.Install.Boot.Data = append(data.Install.Boot.Data, "https://github.com/autonomy/talos/releases/download/v0.1.0-alpha.13/initramfs.xz")
		}
	}

	// Verify that the disks are unused
	// Maybe a simple check against bd.UUID is more appropriate?
	if !data.Install.Wipe {
		var bd *mount.BlockDevice
		for _, device := range []string{data.Install.Boot.Device, data.Install.Root.Device, data.Install.Data.Device} {
			bd, err = mount.ProbeDevice(device)
			if err != nil {
				return err
			}
			if bd.LABEL == "" || bd.TYPE == "" || bd.PARTLABEL == "" {
				return fmt.Errorf("%s: %s", "target install device is not empty", device)
			}
		}

	}

	// Create a map of all the devices we need to be concerned with
	devices := make(map[string]*device)

	// PR: Should we only allow boot device creation if data.Install.Wipe?
	if data.Install.Boot.Device != "" {
		devices[constants.BootPartitionLabel] = newDevice(data.Install.Boot.Device, constants.BootPartitionLabel, data.Install.Boot.Size, data.Install.Boot.Data)
	}
	devices[constants.RootPartitionLabel] = newDevice(data.Install.Root.Device, constants.RootPartitionLabel, data.Install.Root.Size, data.Install.Root.Data)

	devices[constants.DataPartitionLabel] = newDevice(data.Install.Data.Device, constants.DataPartitionLabel, data.Install.Data.Size, data.Install.Data.Data)

	// Use the below to only open a block device once
	uniqueDevices := make(map[string]*blockdevice.BlockDevice)

	// Associate block device to a partition table. This allows us to
	// make use of a single partition table across an entire block device.
	partitionTables := make(map[*blockdevice.BlockDevice]table.PartitionTable)
	for _, device := range []string{data.Install.Boot.Device, data.Install.Root.Device, data.Install.Data.Device} {
		if dev, ok := uniqueDevices[device]; ok {
			devices[device].BlockDevice = dev
			devices[device].PartitionTable = partitionTables[dev]
			continue
		}

		var bd *blockdevice.BlockDevice

		bd, err = blockdevice.Open(device)
		if err != nil {
			return err
		}

		// nolint: errcheck
		defer bd.Close()

		// Ignoring error here since it should only happen if this is an empty disk
		// where a partition table does not already exist
		// nolint: errcheck
		pt, _ := bd.PartitionTable(!data.Install.Wipe)
		uniqueDevices[device] = bd
		partitionTables[bd] = pt

		devices[device].BlockDevice = bd
		devices[device].PartitionTable = pt
	}

	for _, dev := range devices {
		// Partition the disk
		err = dev.Partition()
		if err != nil {
			break
		}
		// Create the device files
		err = dev.BlockDevice.RereadPartitionTable()
		if err != nil {
			break
		}
		// Create the filesystem
		err = dev.Format()
		if err != nil {
			break
		}
		// Mount up the new filesystem
		err = dev.Mount()
		if err != nil {
			break
		}
		// Install the necessary bits/files
		// // download / copy kernel bits to boot
		// // download / extract rootfsurl
		// // handle data dirs creation
		err = dev.Install()
		if err != nil {
			break
		}
		// Unmount the disk so we can proceed to the next phase
		// as if there was no installation phase
		err = dev.Unmount()
		if err != nil {
			break
		}
	}

	return err
}

type device struct {
	Label    string
	Name     string
	Size     uint
	DataURLs []string

	BlockDevice *blockdevice.BlockDevice
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
}

func newDevice(name string, label string, size uint, data []string) *device {
	return &device{
		Name:     name,
		Label:    label,
		Size:     size,
		DataURLs: data,
	}
}

func (d *device) Partition() error {
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
			return fmt.Errorf("%s", "unsupported cpu architecture")
		}
	case constants.DataPartitionLabel:
		// Data Partition
		typeID = "AF3DC60F-8384-7247-8E79-3D69D8477DE4"
	default:
		return fmt.Errorf("%s", "unknown partition label")
	}

	part, err := d.PartitionTable.Add(uint64(d.Size), partition.WithPartitionType(typeID), partition.WithPartitionName(d.Label))

	d.PartitionName = d.Name + strconv.Itoa(int(part.No()))

	return err
}

func (d *device) Format() error {
	return xfs.MakeFS(d.PartitionName)
}

func (d *device) Mount() error {
	var err error
	if err = os.MkdirAll(filepath.Join("/tmp", d.Label), os.ModeDir); err != nil {
		return err
	}
	if err = unix.Mount(d.PartitionName, filepath.Join("/tmp", d.Label), "xfs", 0, ""); err != nil {
		return err
	}
	return err
}

func (d *device) Install() error {
	for _, artifact := range d.DataURLs {
		out, err := ioutil.TempFile("", "installdata")
		if err != nil {
			return err
		}

		// nolint: errcheck
		defer os.Remove(out.Name())
		// nolint: errcheck
		defer out.Close()

		// Get the data
		resp, err := http.Get(artifact)
		if err != nil {
			return err
		}

		// nolint: errcheck
		defer resp.Body.Close()

		// Write the body to file
		_, err = io.Copy(out, resp.Body)
		if err != nil {
			return err
		}

		// Extract artifact if necessary, otherwise place at root of partition/filesystem
		// Feels kind of janky, but going to use the suffix to denote what to do
		switch {
		case strings.HasSuffix(artifact, ".tar"):
			// extract tar
			return untar(out, filepath.Join("/tmp", d.Label))
		case strings.HasSuffix(artifact, ".xz"):
			// extract xz
			// Maybe change to use gzip instead of xz to use stdlib?
		case strings.HasSuffix(artifact, ".gz"):
		case strings.HasSuffix(artifact, ".tar.gz"):
		default:
			// nothing special, download and go
			dst := strings.Split(artifact, "/")
			return os.Rename(out.Name(), filepath.Join("/tmp/", d.Label, dst[len(dst)-1]))
		}
	}
	return nil
}

func (d *device) Unmount() error {
	return unix.Unmount(filepath.Join("/tmp", d.Label), 0)
}

// https://medium.com/@skdomino/taring-untaring-files-in-go-6b07cf56bc07
// no idea if this gets what we want but seems awful close
// nolint: gocyclo
func untar(tarball *os.File, dst string) error {
	tr := tar.NewReader(tarball)

	for {
		header, err := tr.Next()

		switch {

		// if no more files are found return
		case err == io.EOF:
			return nil

			// return any other error
		case err != nil:
			return err

			// if the header is nil, just skip it (not sure how this happens)
		case header == nil:
			continue
		}

		// the target location where the dir/file should be created
		target := filepath.Join(dst, header.Name)

		// the following switch could also be done using fi.Mode(), not sure if there
		// a benefit of using one vs. the other.
		// fi := header.FileInfo()

		// check the file type
		switch header.Typeflag {

		// if its a dir and it doesn't exist create it
		case tar.TypeDir:
			if _, err := os.Stat(target); err != nil {
				if err := os.MkdirAll(target, 0755); err != nil {
					return err
				}
			}

			// if it's a file create it
		case tar.TypeReg:
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return err
			}

			// copy over contents
			if _, err := io.Copy(f, tr); err != nil {
				return err
			}

			// manually close here after each file operation; defering would cause each file close
			// to wait until all operations have completed.
			// nolint: errcheck
			f.Close()
		}
	}
}
