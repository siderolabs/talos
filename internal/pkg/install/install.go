/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package install

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/talos-systems/talos/internal/pkg/blockdevice/bootloader/syslinux"
	"github.com/talos-systems/talos/internal/pkg/constants"
	"github.com/talos-systems/talos/pkg/userdata"
)

// Install fetches the necessary data locations and copies or extracts
// to the target locations
// nolint: gocyclo
func Install(args string, data *userdata.UserData) (err error) {
	if data.Install == nil {
		return nil
	}

	var exists bool
	if exists, err = Exists(); err != nil {
		return err
	}

	if exists {
		log.Println("found existing installation, skipping install step")
		return nil
	}

	manifest := NewManifest(data)
	for _, targets := range manifest.Targets {
		for _, target := range targets {
			// Handle any pre-setup work that's required
			switch target.Label {
			case constants.BootPartitionLabel:
				// Install/Update the bootloader.
				if err = syslinux.Prepare(target.Device); err != nil {
					return err
				}
			case constants.DataPartitionLabel:
				// Do nothing
				continue
			case constants.NextRootPartitionLabel():
				// Do nothing
				continue
			}

			// Handles the download and extraction of assets
			if err = target.Install(); err != nil {
				return err
			}
		}
	}

	extlinuxconf := &syslinux.ExtlinuxConf{
		Default: constants.CurrentRootPartitionLabel(),
		Labels: []*syslinux.ExtlinuxConfLabel{
			{
				Root:   constants.CurrentRootPartitionLabel(),
				Kernel: filepath.Join("/", constants.CurrentRootPartitionLabel(), filepath.Base(data.Install.Boot.Kernel)),
				Initrd: filepath.Join("/", constants.CurrentRootPartitionLabel(), filepath.Base(data.Install.Boot.Initramfs)),
				Append: args,
			},
		},
	}
	if err = syslinux.Install(filepath.Join(constants.NewRoot, constants.BootMountPoint), extlinuxconf); err != nil {
		return err
	}

	if err = ioutil.WriteFile(filepath.Join(constants.NewRoot, constants.BootMountPoint, "installed"), []byte{}, 0400); err != nil {
		return err
	}

	return nil
}

// Simple extract function
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
