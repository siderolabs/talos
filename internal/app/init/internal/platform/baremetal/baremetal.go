package baremetal

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/autonomy/talos/internal/pkg/blockdevice"
	"github.com/autonomy/talos/internal/pkg/blockdevice/filesystem/xfs"
	"github.com/autonomy/talos/internal/pkg/blockdevice/probe"
	"github.com/autonomy/talos/internal/pkg/blockdevice/table"
	"github.com/autonomy/talos/internal/pkg/blockdevice/table/gpt/partition"
	"github.com/autonomy/talos/internal/pkg/constants"
	"github.com/autonomy/talos/internal/pkg/kernel"
	"github.com/autonomy/talos/internal/pkg/userdata"
	"github.com/autonomy/talos/internal/pkg/version"
	"github.com/pkg/errors"

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

	log.Println("starting installation")

	// Root Device Init
	if data.Install.Root.Device == "" {
		return errors.Errorf("%s", "install.rootdevice is required")
	}

	if data.Install.Root.Size == 0 {
		// Set to 1G default for funzies
		data.Install.Root.Size = 1024 * 1000 * 1000
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

	if len(data.Install.Data.Data) == 0 {
		// Stub out the dir structure for `/var`
		data.Install.Data.Data = append(data.Install.Data.Data, []string{"cache", "lib", "lib/misc", "log", "mail", "opt", "run:/run", "spool", "tmp"}...)
	}

	// Boot Device Init
	if data.Install.Boot != nil {
		if data.Install.Boot.Device == "" {
			data.Install.Boot.Device = data.Install.Root.Device
		}
		if data.Install.Boot.Size == 0 {
			// Set to 512MB default for funzies
			data.Install.Boot.Size = 512 * 1000
		}
		if len(data.Install.Data.Data) == 0 {
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
		devices[constants.BootPartitionLabel] = NewDevice(data.Install.Boot.Device, constants.BootPartitionLabel, data.Install.Boot.Size, data.Install.Wipe, false, data.Install.Boot.Data)
		labeldev[constants.BootPartitionLabel] = data.Install.Boot.Device
	}

	devices[constants.RootPartitionLabel] = NewDevice(data.Install.Root.Device, constants.RootPartitionLabel, data.Install.Root.Size, data.Install.Wipe, false, data.Install.Root.Data)
	labeldev[constants.RootPartitionLabel] = data.Install.Root.Device

	devices[constants.DataPartitionLabel] = NewDevice(data.Install.Data.Device, constants.DataPartitionLabel, data.Install.Data.Size, data.Install.Wipe, false, data.Install.Data.Data)
	labeldev[constants.DataPartitionLabel] = data.Install.Data.Device

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
	for _, label := range []string{constants.BootPartitionLabel, constants.RootPartitionLabel, constants.DataPartitionLabel} {
		// Partition the disk
		log.Printf("Partitioning %s - %s\n", devices[label].Name, label)
		err = devices[label].Partition()
		if err != nil {
			return err
		}
	}

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
		err = dev.BlockDevice.RereadPartitionTable()
		if err != nil {
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
		// Mount up the new filesystem
		log.Printf("Mounting Partition %s - %s\n", dev.Name, dev.Label)
		err = dev.Mount()
		if err != nil {
			return err
		}
		// Install the necessary bits/files
		// // download / copy kernel bits to boot
		// // download / extract rootfsurl
		// // handle data dirs creation
		log.Printf("Installing Partition %s - %s\n", dev.Name, dev.Label)
		err = dev.Install()
		if err != nil {
			return err
		}
		// Unmount the disk so we can proceed to the next phase
		// as if there was no installation phase
		log.Printf("Unmounting Partition %s - %s\n", dev.Name, dev.Label)
		err = dev.Unmount()
		if err != nil {
			return err
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

// NewDevice create a Device with basic metadata. BlockDevice and PartitionTable
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

// Mount will create the mountpoint and mount the partition to MountBase/Label
// ex, /tmp/DATA
func (d *Device) Mount() error {
	var err error
	if err = os.MkdirAll(filepath.Join(d.MountBase, d.Label), os.ModeDir); err != nil {
		return err
	}
	if err = unix.Mount(d.PartitionName, filepath.Join(d.MountBase, d.Label), "xfs", 0, ""); err != nil {
		return err
	}
	return err
}

// Install downloads the necessary artifacts and creates the necessary directories
// for installation of the OS
// nolint: gocyclo
func (d *Device) Install() error {
	mountpoint := filepath.Join(d.MountBase, d.Label)

	for _, artifact := range d.DataURLs {
		// Extract artifact if necessary, otherwise place at root of partition/filesystem
		switch {
		case strings.HasPrefix(artifact, "http"):
			u, err := url.Parse(artifact)
			if err != nil {
				return err
			}

			out, err := downloader(u, d.MountBase)
			if err != nil {
				return err
			}

			// TODO add support for checksum validation of downloaded file

			// nolint: errcheck
			defer os.Remove(out.Name())
			// nolint: errcheck
			defer out.Close()

			switch {
			case strings.HasSuffix(artifact, ".tar") || strings.HasSuffix(artifact, ".tar.gz"):
				// extract tar
				err = untar(out, mountpoint)
				if err != nil {
					return err
				}
			default:
				// nothing special, download and go
				dst := strings.Split(artifact, "/")
				err = os.Rename(out.Name(), filepath.Join(mountpoint, dst[len(dst)-1]))
				if err != nil {
					return err
				}
			}
		default:
			// Local directories/links
			link := strings.Split(artifact, ":")
			if len(link) == 1 {
				if err := os.MkdirAll(filepath.Join(mountpoint, artifact), 0755); err != nil {
					return err
				}
			} else {
				if err := os.Symlink(link[1], filepath.Join(mountpoint, link[0])); err != nil && !os.IsExist(err) {
					return err
				}
			}
		}
	}
	return nil
}

// Unmount unmounts the partition
func (d *Device) Unmount() error {
	return unix.Unmount(filepath.Join(d.MountBase, d.Label), 0)
}

// Simple extract function
// nolint: gocyclo
func untar(tarball *os.File, dst string) error {

	var input io.Reader
	var err error

	if strings.HasSuffix(tarball.Name(), ".tar.gz") {
		input, err = gzip.NewReader(tarball)
		if err != nil {
			return err
		}

		// nolint: errcheck
		defer input.(*gzip.Reader).Close()
	} else {
		input = tarball
	}

	tr := tar.NewReader(input)

	for {
		var header *tar.Header

		header, err = tr.Next()

		switch {
		case err == io.EOF:
			err = nil
			return err
		case err != nil:
			return err
		case header == nil:
			continue
		}

		// the target location where the dir/file should be created
		target := filepath.Join(dst, header.Name)

		// May need to add in support for these
		/*
			// Type '1' to '6' are header-only flags and may not have a data body.
			TypeLink    = '1' // Hard link
			TypeSymlink = '2' // Symbolic link
			TypeChar    = '3' // Character device node
			TypeBlock   = '4' // Block device node
			TypeDir     = '5' // Directory
			TypeFifo    = '6' // FIFO node
		*/
		switch header.Typeflag {
		case tar.TypeDir:
			if err = os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			var output *os.File

			output, err = os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return err
			}

			if _, err = io.Copy(output, tr); err != nil {
				return err
			}

			err = output.Close()
			if err != nil {
				return err
			}
		case tar.TypeSymlink:
			dest := filepath.Join(dst, header.Name)
			source := header.Linkname
			if err := os.Symlink(source, dest); err != nil {
				return err
			}
		}
	}
}

func downloader(artifact *url.URL, base string) (*os.File, error) {
	out, err := os.Create(filepath.Join(base, filepath.Base(artifact.Path)))
	if err != nil {
		return nil, err
	}

	// Get the data
	resp, err := http.Get(artifact.String())
	if err != nil {
		return out, err
	}

	// nolint: errcheck
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		// nolint: errcheck
		out.Close()
		return nil, errors.Errorf("Failed to download %s, got %d", artifact, resp.StatusCode)
	}

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return out, err
	}

	// Reset out file position to 0 so we can immediately read from it
	_, err = out.Seek(0, 0)

	return out, err
}
