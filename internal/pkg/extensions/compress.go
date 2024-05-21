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
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
)

// List of globs and destinations for early CPU ucode.
//
// See https://www.kernel.org/doc/html/v6.1/x86/microcode.html#early-load-microcode.
//
// We need to repackage the ucode blobs matching the glob into the destination concatenating
// them all together.
// The resulting blobs should be placed into uncompressed cpio archive prepended to the normal (compressed) initramfs.
var earlyCPUUcode = []struct {
	glob, dst string
}{
	{"/lib/firmware/intel-ucode/*", "kernel/x86/microcode/GenuineIntel.bin"},
	{"/lib/firmware/amd-ucode/microcode_amd*.bin", "kernel/x86/microcode/AuthenticAMD.bin"},
}

// List of paths to be moved to the future initramfs.
var initramfsPaths = []string{
	constants.FirmwarePath,
}

// Compress builds the squashfs image in the specified destination folder.
//
// Components which should be placed to the initramfs are moved to the initramfsPath.
// Ucode components are moved into a separate designated location.
func (ext *Extension) Compress(squashPath, initramfsPath string, quirks quirks.Quirks) (string, error) {
	if err := ext.handleUcode(initramfsPath); err != nil {
		return "", err
	}

	for _, path := range initramfsPaths {
		if _, err := os.Stat(filepath.Join(ext.RootfsPath(), path)); err == nil {
			if err = moveFiles(filepath.Join(ext.RootfsPath(), path), filepath.Join(initramfsPath, path)); err != nil {
				return "", err
			}
		}
	}

	squashPath = filepath.Join(squashPath, fmt.Sprintf("%s.sqsh", ext.Directory()))

	var compressArgs []string

	if quirks.UseZSTDCompression() {
		compressArgs = []string{"-comp", "zstd", "-Xcompression-level", "18"}
	} else {
		compressArgs = []string{"-comp", "xz", "-Xdict-size", "100%"}
	}

	cmd := exec.Command("mksquashfs", append([]string{ext.RootfsPath(), squashPath, "-all-root", "-noappend", "-no-progress"}, compressArgs...)...)
	cmd.Stderr = os.Stderr

	return squashPath, cmd.Run()
}

func appendBlob(dst io.Writer, srcPath string) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}

	defer src.Close() //nolint:errcheck

	if _, err = io.Copy(dst, src); err != nil {
		return err
	}

	if err = src.Close(); err != nil {
		return err
	}

	return os.Remove(srcPath)
}

func (ext *Extension) handleUcode(initramfsPath string) error {
	for _, ucode := range earlyCPUUcode {
		matches, err := filepath.Glob(filepath.Join(ext.RootfsPath(), ucode.glob))
		if err != nil {
			return err
		}

		if len(matches) == 0 {
			continue
		}

		// if some ucode is found, append it to the blob in the initramfs
		if err = os.MkdirAll(filepath.Dir(filepath.Join(initramfsPath, ucode.dst)), 0o755); err != nil {
			return err
		}

		dst, err := os.OpenFile(filepath.Join(initramfsPath, ucode.dst), os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
		if err != nil {
			return err
		}

		defer dst.Close() //nolint:errcheck

		for _, match := range matches {
			if err = appendBlob(dst, match); err != nil {
				return err
			}
		}

		if err = dst.Close(); err != nil {
			return err
		}
	}

	return nil
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
