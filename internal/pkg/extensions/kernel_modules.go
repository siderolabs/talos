// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package extensions provides function to manage system extensions.
package extensions

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/klauspost/compress/zstd"
	"github.com/u-root/u-root/pkg/cpio"
	"github.com/ulikunitz/xz"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/extensions"
)

// ProvidesKernelModules returns true if the extension provides kernel modules.
func (ext *Extension) ProvidesKernelModules() bool {
	if _, err := os.Stat(ext.KernelModuleDirectory()); os.IsNotExist(err) {
		return false
	}

	return true
}

// KernelModuleDirectory returns the path to the kernel modules directory.
func (ext *Extension) KernelModuleDirectory() string {
	return filepath.Join(ext.RootfsPath(), constants.KernelModulesPath)
}

func autoDecompress(r io.Reader) (io.Reader, error) {
	var magic [4]byte

	if _, err := r.Read(magic[:]); err != nil {
		return nil, err
	}

	src := io.MultiReader(bytes.NewReader(magic[:]), r)

	// xz magic
	if bytes.Equal(magic[:], []byte{0xfd, '7', 'z', 'X'}) {
		return xz.NewReader(src)
	}

	return zstd.NewReader(src)
}

// GenerateKernelModuleDependencyTreeExtension generates a kernel module dependency tree extension.
//
//nolint:gocyclo
func GenerateKernelModuleDependencyTreeExtension(extensionPathsWithKernelModules []string, initramfsPath, scratchPath string, printFunc func(format string, v ...any)) (*Extension, error) {
	printFunc("preparing to run depmod to generate kernel modules dependency tree")

	tempDir, err := os.MkdirTemp("", "ext-modules")
	if err != nil {
		return nil, err
	}

	defer logErr("removing temporary directory", func() error {
		return os.RemoveAll(tempDir)
	})

	initramfsxz, err := os.Open(initramfsPath)
	if err != nil {
		return nil, err
	}

	defer logErr("closing initramfs", func() error {
		return initramfsxz.Close()
	})

	r, err := autoDecompress(initramfsxz)
	if err != nil {
		return nil, err
	}

	tempRootfsFile := filepath.Join(tempDir, constants.RootfsAsset)

	if err = extractRootfsFromInitramfs(r, tempRootfsFile); err != nil {
		return nil, fmt.Errorf("error extacting cpio: %w", err)
	}

	// extract /lib/modules from the squashfs under a temporary root to run depmod on it
	tempLibModules := filepath.Join(tempDir, "modules")

	if err = unsquash(tempRootfsFile, tempLibModules, constants.KernelModulesPath); err != nil {
		return nil, fmt.Errorf("error running unsquashfs: %w", err)
	}

	rootfsKernelModulesPath := filepath.Join(tempLibModules, constants.KernelModulesPath)

	// under the /lib/modules there should be the only path which is the kernel version
	contents, err := os.ReadDir(rootfsKernelModulesPath)
	if err != nil {
		return nil, err
	}

	if len(contents) != 1 || !contents[0].IsDir() {
		return nil, fmt.Errorf("invalid kernel modules path: %s", rootfsKernelModulesPath)
	}

	kernelVersionPath := contents[0].Name()

	// copy to the same location modules from all extensions
	for _, path := range extensionPathsWithKernelModules {
		if err = copyFiles(filepath.Join(path, kernelVersionPath), filepath.Join(rootfsKernelModulesPath, kernelVersionPath)); err != nil {
			return nil, fmt.Errorf("copying kernel modules from %s failed: %w", path, err)
		}
	}

	printFunc("running depmod to generate kernel modules dependency tree")

	if err = depmod(tempLibModules, kernelVersionPath); err != nil {
		return nil, fmt.Errorf("error running depmod: %w", err)
	}

	// we want the extension to survive this function, so not storing in a temporary directory
	kernelModulesDependencyTreeStagingDir := filepath.Join(scratchPath, "modules.dep")

	// we want to make sure the root directory has the right permissions.
	if err := os.MkdirAll(kernelModulesDependencyTreeStagingDir, 0o755); err != nil {
		return nil, err
	}

	kernelModulesDepenencyTreeDirectory := filepath.Join(kernelModulesDependencyTreeStagingDir, constants.KernelModulesPath, kernelVersionPath)

	if err := os.MkdirAll(kernelModulesDepenencyTreeDirectory, 0o755); err != nil {
		return nil, err
	}

	if err := findAndMoveKernelModulesDepFiles(kernelModulesDepenencyTreeDirectory, filepath.Join(rootfsKernelModulesPath, kernelVersionPath)); err != nil {
		return nil, err
	}

	kernelModulesDepTreeExtension := extensions.New(
		kernelModulesDependencyTreeStagingDir, "modules.dep",
		extensions.Manifest{
			Version: kernelVersionPath,
			Metadata: extensions.Metadata{
				Name:        "modules.dep",
				Version:     kernelVersionPath,
				Author:      "Talos Machinery",
				Description: "Combined modules.dep for all extensions",
			},
		},
	)

	return &Extension{kernelModulesDepTreeExtension}, nil
}

func logErr(msg string, f func() error) {
	// if file is already closed, ignore the error
	if err := f(); err != nil && !errors.Is(err, os.ErrClosed) {
		log.Println(msg, err)
	}
}

func extractRootfsFromInitramfs(r io.Reader, rootfsFilePath string) error {
	recReader := cpio.Newc.Reader(&discarder{r: r})

	return cpio.ForEachRecord(recReader, func(r cpio.Record) error {
		if r.Name != constants.RootfsAsset {
			return nil
		}

		reader := io.NewSectionReader(r.ReaderAt, 0, int64(r.FileSize))

		f, err := os.Create(rootfsFilePath)
		if err != nil {
			return err
		}

		defer logErr("closing rootfs", func() error {
			return f.Close()
		})

		_, err = io.Copy(f, reader)
		if err != nil {
			return err
		}

		return f.Close()
	})
}

func unsquash(squashfsPath, dest, path string) error {
	cmd := exec.Command("unsquashfs", "-d", dest, "-f", "-n", squashfsPath, path)
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func depmod(baseDir, kernelVersionPath string) error {
	baseDir = strings.TrimSuffix(baseDir, constants.KernelModulesPath)

	cmd := exec.Command("depmod", "--all", "--basedir", baseDir, "--config", "/etc/modules.d/10-extra-modules.conf", kernelVersionPath)
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func findAndMoveKernelModulesDepFiles(dest, kernelModulesPath string) error {
	fs, err := os.ReadDir(kernelModulesPath)
	if err != nil {
		return err
	}

	for _, f := range fs {
		if f.IsDir() {
			continue
		}

		if strings.HasPrefix(f.Name(), "modules.") {
			fs, err := f.Info()
			if err != nil {
				return err
			}

			if err := moveFile(fs, filepath.Join(kernelModulesPath, f.Name()), filepath.Join(dest, f.Name())); err != nil {
				return err
			}
		}
	}

	return nil
}
