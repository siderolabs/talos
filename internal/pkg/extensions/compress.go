// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package extensions

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// List of paths to be moved to the future initramfs.
var initramfsPaths = []string{
	constants.FirmwarePath,
}

// Compress builds the squashfs image in the specified destination folder.
//
// Components which should be placed to the initramfs are moved to the initramfsPath.
func (ext *Extension) Compress(squashPath, initramfsPath string) (string, error) {
	for _, path := range initramfsPaths {
		if _, err := os.Stat(filepath.Join(ext.rootfsPath, path)); err == nil {
			if err = moveFiles(filepath.Join(ext.rootfsPath, path), filepath.Join(initramfsPath, path)); err != nil {
				return "", err
			}
		}
	}

	squashPath = filepath.Join(squashPath, fmt.Sprintf("%s.sqsh", ext.directory))

	cmd := exec.Command("mksquashfs", ext.rootfsPath, squashPath, "-all-root", "-noappend", "-comp", "xz", "-Xdict-size", "100%", "-no-progress")
	cmd.Stderr = os.Stderr

	return squashPath, cmd.Run()
}

func moveFiles(srcPath, dstPath string) error {
	return handleFilesOp(srcPath, dstPath, os.Remove)
}

func copyFiles(srcPath, dstPath string) error {
	return handleFilesOp(srcPath, dstPath, nil)
}

func handleFilesOp(srcPath, dstPath string, op func(string) error) error {
	st, err := os.Stat(srcPath)
	if err != nil {
		return err
	}

	if st.IsDir() {
		return handleDirectoryOp(st, srcPath, dstPath, op)
	}

	return handleFileOp(st, srcPath, dstPath, op)
}

func moveFile(st fs.FileInfo, srcPath, dstPath string) error {
	return handleFileOp(st, srcPath, dstPath, os.Remove)
}

func handleFileOp(st fs.FileInfo, srcPath, dstPath string, op func(string) error) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}

	defer src.Close() //nolint:errcheck

	dst, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY, st.Mode().Perm())
	if err != nil {
		return err
	}

	defer dst.Close() //nolint:errcheck

	_, err = io.Copy(dst, src)
	if err != nil {
		return err
	}

	if op != nil {
		return op(srcPath)
	}

	return nil
}

func handleDirectoryOp(st fs.FileInfo, srcPath, dstPath string, op func(string) error) error {
	if err := os.MkdirAll(dstPath, st.Mode().Perm()); err != nil {
		return err
	}

	contents, err := os.ReadDir(srcPath)
	if err != nil {
		return err
	}

	for _, item := range contents {
		if err = handleFilesOp(filepath.Join(srcPath, item.Name()), filepath.Join(dstPath, item.Name()), op); err != nil {
			return err
		}
	}

	if op != nil {
		return op(srcPath)
	}

	return nil
}
