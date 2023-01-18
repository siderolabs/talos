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

	"github.com/u-root/u-root/pkg/cpio"
	"github.com/ulikunitz/xz"
	"golang.org/x/sys/unix"
	"gopkg.in/freddierice/go-losetup.v1"

	"github.com/siderolabs/talos/internal/pkg/mount"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/extensions"
)

// ProvidesKernelModules returns true if the extension provides kernel modules.
func (ext *Extension) ProvidesKernelModules() bool {
	if _, err := os.Stat(filepath.Join(ext.rootfsPath, constants.DefaultKernelModulesPath)); os.IsNotExist(err) {
		return false
	}

	return true
}

// KernelModuleDirectory returns the path to the kernel modules directory.
func (ext *Extension) KernelModuleDirectory() string {
	return filepath.Join(ext.rootfsPath, constants.DefaultKernelModulesPath)
}

// GenerateKernelModuleDependencyTreeExtension generates a kernel module dependency tree extension.
// nolint:gocyclo
func GenerateKernelModuleDependencyTreeExtension(extensionsPathWithKernelModules []string, arch string) (*Extension, error) {
	log.Println("preparing to run depmod to generate kernel modules dependency tree")

	tempDir, err := os.MkdirTemp("", "ext-modules")
	if err != nil {
		return nil, err
	}

	defer logErr(func() error {
		return os.RemoveAll(tempDir)
	})

	initramfsxz, err := os.Open(fmt.Sprintf(constants.InitramfsAssetPath, arch))
	if err != nil {
		return nil, err
	}

	defer logErr(func() error {
		return initramfsxz.Close()
	})

	r, err := xz.NewReader(initramfsxz)
	if err != nil {
		return nil, err
	}

	var buff bytes.Buffer

	if _, err = io.Copy(&buff, r); err != nil {
		return nil, err
	}

	tempRootfsFile := filepath.Join(tempDir, constants.RootfsAsset)

	if err = extractRootfsFromInitramfs(buff, tempRootfsFile); err != nil {
		return nil, err
	}

	// now we are ready to mount rootfs.sqsh
	// create a mount point under tempDir
	rootfsMountPath := filepath.Join(tempDir, "rootfs-mnt")

	// create the loopback device from the squashfs file
	dev, err := losetup.Attach(tempRootfsFile, 0, true)
	if err != nil {
		return nil, err
	}

	defer logErr(func() error {
		if err = dev.Detach(); err != nil {
			return err
		}

		return dev.Remove()
	})

	// setup a temporary mount point for the squashfs file and mount it
	m := mount.NewMountPoint(dev.Path(), rootfsMountPath, "squashfs", unix.MS_RDONLY|unix.MS_I_VERSION, "", mount.WithFlags(mount.ReadOnly|mount.Shared))

	if err = m.Mount(); err != nil {
		return nil, err
	}

	defer logErr(func() error {
		return m.Unmount()
	})

	// create an overlayfs which contains the rootfs squashfs mount as the base
	// and the extension modules as subsequent lower directories
	overlays := mount.NewMountPoints()
	// writable overlayfs mount inside a container required a tmpfs mount
	overlays.Set("overlays-tmpfs", mount.NewMountPoint("tmpfs", constants.VarSystemOverlaysPath, "tmpfs", unix.MS_I_VERSION, ""))

	// append the rootfs mount point
	extensionsPathWithKernelModules = append(extensionsPathWithKernelModules, filepath.Join(rootfsMountPath, constants.DefaultKernelModulesPath))

	// create the overlayfs mount point as read write
	mp := mount.NewMountPoint("", strings.Join(extensionsPathWithKernelModules, ":"), "", unix.MS_I_VERSION, "", mount.WithFlags(mount.Overlay|mount.Shared))
	overlays.Set("overlays-mnt", mp)

	if err = mount.Mount(overlays); err != nil {
		return nil, err
	}

	defer logErr(func() error {
		return mount.Unmount(overlays)
	})

	log.Println("running depmod to generate kernel modules dependency tree")

	if err = depmod(mp.Target()); err != nil {
		return nil, err
	}

	// we want this temp directory to be present until the extension is compressed later on, so not removing it here
	kernelModulesDependencyTreeStagingDir, err := os.MkdirTemp("", "module-deps")
	if err != nil {
		return nil, err
	}

	kernelModulesDepenencyTreeDirectory := filepath.Join(kernelModulesDependencyTreeStagingDir, constants.DefaultKernelModulesPath)

	if err := os.MkdirAll(kernelModulesDepenencyTreeDirectory, 0o755); err != nil {
		return nil, err
	}

	if err := findAndMoveKernelModulesDepFiles(kernelModulesDepenencyTreeDirectory, mp.Target()); err != nil {
		return nil, err
	}

	kernelModulesDepTreeExtension := newExtension(kernelModulesDependencyTreeStagingDir, "modules.dep")
	kernelModulesDepTreeExtension.Manifest = extensions.Manifest{
		Version: constants.DefaultKernelVersion,
		Metadata: extensions.Metadata{
			Name:        "modules.dep",
			Version:     constants.DefaultKernelVersion,
			Author:      "Talos Machinery",
			Description: "Combined modules.dep for all extensions",
		},
	}

	return kernelModulesDepTreeExtension, nil
}

func logErr(f func() error) {
	// if file is already closed, ignore the error
	if err := f(); !errors.Is(err, os.ErrClosed) {
		log.Println(err)
	}
}

func extractRootfsFromInitramfs(input bytes.Buffer, rootfsFilePath string) error {
	recReader := cpio.Newc.Reader(bytes.NewReader(input.Bytes()))

	if err := cpio.ForEachRecord(recReader, func(r cpio.Record) error {
		if r.Name != constants.RootfsAsset {
			return nil
		}

		reader := io.NewSectionReader(r.ReaderAt, 0, int64(r.FileSize))
		f, err := os.Create(rootfsFilePath)
		if err != nil {
			return err
		}

		defer logErr(func() error {
			return f.Close()
		})

		_, err = io.Copy(f, reader)
		if err != nil {
			return err
		}

		return f.Close()
	}); err != nil {
		return err
	}

	return nil
}

func depmod(kernelModulesPath string) error {
	baseDir := strings.TrimSuffix(kernelModulesPath, constants.DefaultKernelModulesPath)

	cmd := exec.Command("depmod", "--all", "--basedir", baseDir, "--config", "/etc/modules.d/10-extra-modules.conf", constants.DefaultKernelVersion)
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
