/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package manifest

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/talos-systems/talos/pkg/blockdevice"
	"github.com/talos-systems/talos/pkg/blockdevice/filesystem/vfat"
	"github.com/talos-systems/talos/pkg/blockdevice/filesystem/xfs"
	"github.com/talos-systems/talos/pkg/blockdevice/table"
	"github.com/talos-systems/talos/pkg/blockdevice/table/gpt/partition"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/userdata"
)

// Manifest represents the instructions for preparing all block devices
// for an installation.
type Manifest struct {
	Targets map[string][]*Target
}

// Target represents an installation partition.
type Target struct {
	Label          string
	MountPoint     string
	Device         string
	FileSystemType string
	PartitionName  string
	Size           uint
	Force          bool
	Test           bool
	Assets         []*Asset
	BlockDevice    *blockdevice.BlockDevice
}

// Asset represents a file required by a target.
type Asset struct {
	Source      string
	Destination string
}

// NewManifest initializes and returns a Manifest.
func NewManifest(data *userdata.UserData) (manifest *Manifest) {
	manifest = &Manifest{
		Targets: map[string][]*Target{},
	}

	// Initialize any slices we need. Note that a boot paritition is not
	// required.

	if manifest.Targets[data.Install.Ephemeral.Device] == nil {
		manifest.Targets[data.Install.Ephemeral.Device] = []*Target{}
	}

	var bootTarget *Target
	if data.Install.Boot != nil {
		bootTarget = &Target{
			Device: data.Install.Boot.Device,
			Label:  constants.BootPartitionLabel,
			Size:   data.Install.Boot.Size,
			Force:  data.Install.Force,
			Test:   false,
			Assets: []*Asset{
				{
					Source:      data.Install.Boot.Kernel,
					Destination: filepath.Join("/", "default", filepath.Base(data.Install.Boot.Kernel)),
				},
				{
					Source:      data.Install.Boot.Initramfs,
					Destination: filepath.Join("/", "default", filepath.Base(data.Install.Boot.Initramfs)),
				},
			},
			MountPoint: constants.BootMountPoint,
		}
	}

	dataTarget := &Target{
		Device:     data.Install.Ephemeral.Device,
		Label:      constants.EphemeralPartitionLabel,
		Size:       data.Install.Ephemeral.Size,
		Force:      data.Install.Force,
		Test:       false,
		MountPoint: constants.EphemeralMountPoint,
	}

	for _, target := range []*Target{bootTarget, dataTarget} {
		if target == nil {
			continue
		}
		manifest.Targets[target.Device] = append(manifest.Targets[target.Device], target)
	}

	for _, extra := range data.Install.ExtraDevices {
		if manifest.Targets[extra.Device] == nil {
			manifest.Targets[extra.Device] = []*Target{}
		}

		for _, part := range extra.Partitions {
			extraTarget := &Target{
				Device: extra.Device,
				Size:   part.Size,
				Force:  data.Install.Force,
				Test:   false,
			}

			manifest.Targets[extra.Device] = append(manifest.Targets[extra.Device], extraTarget)
		}
	}

	return manifest
}

// ExecuteManifest partitions and formats all disks in a manifest.
func (m *Manifest) ExecuteManifest(data *userdata.UserData, manifest *Manifest) (err error) {
	for dev, targets := range manifest.Targets {
		var bd *blockdevice.BlockDevice
		if bd, err = blockdevice.Open(dev, blockdevice.WithNewGPT(data.Install.Force)); err != nil {
			return err
		}
		// nolint: errcheck
		defer bd.Close()

		for _, target := range targets {
			if err = target.Partition(bd); err != nil {
				return errors.Wrap(err, "failed to partition device")
			}
		}

		if err = bd.RereadPartitionTable(); err != nil {
			return err
		}

		for _, target := range targets {
			if err = target.Format(); err != nil {
				return errors.Wrap(err, "failed to format device")
			}
		}
	}

	return nil
}

// Partition creates a new partition on the specified device.
// nolint: dupl, gocyclo
func (t *Target) Partition(bd *blockdevice.BlockDevice) (err error) {
	log.Printf("partitioning %s - %s\n", t.Device, t.Label)

	var pt table.PartitionTable
	if pt, err = bd.PartitionTable(true); err != nil {
		return err
	}

	opts := []interface{}{partition.WithPartitionTest(t.Test)}

	switch t.Label {
	case constants.BootPartitionLabel:
		// EFI System Partition
		typeID := "C12A7328-F81F-11D2-BA4B-00A0C93EC93B"
		opts = append(opts, partition.WithPartitionType(typeID), partition.WithPartitionName(t.Label), partition.WithLegacyBIOSBootableAttribute(true))
	case constants.EphemeralPartitionLabel:
		// Ephemeral Partition
		typeID := "AF3DC60F-8384-7247-8E79-3D69D8477DE4"
		opts = append(opts, partition.WithPartitionType(typeID), partition.WithPartitionName(t.Label))
	default:
		typeID := "AF3DC60F-8384-7247-8E79-3D69D8477DE4"
		opts = append(opts, partition.WithPartitionType(typeID))
	}

	part, err := pt.Add(uint64(t.Size), opts...)
	if err != nil {
		return err
	}

	if err = pt.Write(); err != nil {
		return err
	}

	// TODO(andrewrynhard): We should really have a custom type that has all
	// the methods we need. This switch statement shows up in some form in
	// multiple places.
	switch dev := t.Device; {
	case strings.HasPrefix(dev, "/dev/nvme"):
		fallthrough
	case strings.HasPrefix(dev, "/dev/loop"):
		t.PartitionName = t.Device + "p" + strconv.Itoa(int(part.No()))
	default:
		t.PartitionName = t.Device + strconv.Itoa(int(part.No()))
	}

	return nil
}

// Format creates a filesystem on the device/partition.
func (t *Target) Format() error {
	if t.Label == constants.BootPartitionLabel {
		log.Printf("formatting partition %s - %s as %s\n", t.PartitionName, t.Label, "fat")
		return vfat.MakeFS(t.PartitionName, vfat.WithLabel(t.Label))
	}
	log.Printf("formatting partition %s - %s as %s\n", t.PartitionName, t.Label, "xfs")
	opts := []xfs.Option{xfs.WithForce(t.Force)}
	if t.Label != "" {
		opts = append(opts, xfs.WithLabel(t.Label))
	}
	return xfs.MakeFS(t.PartitionName, opts...)
}

// Save handles downloading the necessary assets and extracting them to
// the appropriate location.
// nolint: gocyclo
func (t *Target) Save() error {
	// Download and extract all artifacts.
	var sourceFile *os.File
	var destFile *os.File
	var err error

	if err = os.MkdirAll(t.MountPoint, os.ModeDir); err != nil {
		return err
	}

	// Extract artifact if necessary, otherwise place at root of partition/filesystem
	for _, asset := range t.Assets {
		var u *url.URL
		if u, err = url.Parse(asset.Source); err != nil {
			return err
		}

		sourceFile = nil
		destFile = nil

		// Handle fetching of asset
		switch u.Scheme {
		case "http":
			fallthrough
		case "https":
			log.Printf("downloading %s\n", asset.Source)
			dest := filepath.Join(t.MountPoint, asset.Destination)
			sourceFile, err = download(u.String(), dest)
			if err != nil {
				return err
			}
		case "file":
			source := u.Path
			dest := filepath.Join(t.MountPoint, asset.Destination)

			sourceFile, err = os.Open(source)
			if err != nil {
				return err
			}

			if err = os.MkdirAll(filepath.Dir(dest), 0700); err != nil {
				return err
			}

			destFile, err = os.Create(dest)
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported path scheme, got %s but supported %s", u.Scheme, "https://|http://|file://")
		}

		// Handle extraction/Installation
		switch {
		// tar
		case strings.HasSuffix(sourceFile.Name(), ".tar") || strings.HasSuffix(sourceFile.Name(), ".tar.gz"):
			log.Printf("extracting %s to %s\n", sourceFile.Name(), t.MountPoint)

			err = untar(sourceFile, t.MountPoint)
			if err != nil {
				log.Printf("failed to extract %s to %s\n", sourceFile.Name(), t.MountPoint)
				return err
			}

			if err = sourceFile.Close(); err != nil {
				log.Printf("failed to close %s", sourceFile.Name())
				return err
			}

			if err = os.Remove(sourceFile.Name()); err != nil {
				log.Printf("failed to remove %s", sourceFile.Name())
				return err
			}
		// single file
		case strings.HasPrefix(sourceFile.Name(), "/") && destFile != nil:
			log.Printf("copying %s to %s\n", sourceFile.Name(), destFile.Name())

			if _, err = io.Copy(destFile, sourceFile); err != nil {
				log.Printf("failed to copy %s to %s\n", sourceFile.Name(), destFile.Name())
				return err
			}

			if err = destFile.Close(); err != nil {
				log.Printf("failed to close %s", destFile.Name())
				return err
			}

			if err = sourceFile.Close(); err != nil {
				log.Printf("failed to close %s", sourceFile.Name())
				return err
			}
		default:
			if err = sourceFile.Close(); err != nil {
				log.Printf("failed to close %s", sourceFile.Name())
				return err
			}
		}
	}

	return nil
}

// nolint: gocyclo, dupl
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
			var downloadedFileput *os.File

			downloadedFileput, err = os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return err
			}

			if _, err = io.Copy(downloadedFileput, tr); err != nil {
				return err
			}

			err = downloadedFileput.Close()
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

func download(artifact, dest string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(dest), 0700); err != nil {
		return nil, err
	}
	downloadedFile, err := os.Create(dest)
	if err != nil {
		return nil, err
	}

	// Get the data
	resp, err := http.Get(artifact)
	if err != nil {
		return downloadedFile, err
	}

	// nolint: errcheck
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		// nolint: errcheck
		downloadedFile.Close()
		return nil, errors.Errorf("failed to download %s, got %d", artifact, resp.StatusCode)
	}

	// Write the body to file
	_, err = io.Copy(downloadedFile, resp.Body)
	if err != nil {
		return downloadedFile, err
	}

	// Reset downloadedFile file position to 0 so we can immediately read from it
	_, err = downloadedFile.Seek(0, 0)

	// TODO add support for checksum validation of downloaded file

	return downloadedFile, err
}
