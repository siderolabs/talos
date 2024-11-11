// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package qemu

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"slices"
)

// Arch abstracts away differences between different architectures.
type Arch string

// Arch constants.
const (
	ArchAmd64 Arch = "amd64"
	ArchArm64 Arch = "arm64"
)

// Valid checks whether the architecture is supported.
func (arch Arch) Valid() bool {
	switch arch {
	case ArchAmd64, ArchArm64:
		return true
	default:
		return false
	}
}

// QemuArch defines which qemu binary to use.
func (arch Arch) QemuArch() string {
	switch arch {
	case ArchAmd64:
		return "x86_64"
	case ArchArm64:
		return "aarch64"
	default:
		panic("unsupported architecture")
	}
}

// QemuMachine defines the machine type for qemu.
func (arch Arch) QemuMachine() string {
	switch arch {
	case ArchAmd64:
		return "q35"
	case ArchArm64:
		return "virt,gic-version=max"
	default:
		panic("unsupported architecture")
	}
}

// Console defines proper argument for the kernel to send logs to serial console.
func (arch Arch) Console() string {
	switch arch {
	case ArchAmd64:
		return "ttyS0"
	case ArchArm64:
		return "ttyAMA0,115200n8"
	default:
		panic("unsupported architecture")
	}
}

// PFlash for UEFI boot.
type PFlash struct {
	Size        int64
	SourcePaths []string
}

// PFlash returns settings for parallel flash.
func (arch Arch) PFlash(uefiEnabled bool, extraUEFISearchPaths []string) []PFlash {
	switch arch {
	case ArchArm64:
		// default search paths
		uefiSourcePathPrefixes := []string{
			"/usr/share/AAVMF", // most standard location
			"/usr/share/qemu-efi-aarch64",
			"/usr/share/OVMF",
			"/usr/share/edk2/aarch64",      // Fedora
			"/usr/share/edk2/experimental", // Fedora
		}

		// Secure boot enabled firmware files
		uefiSourceFiles := []string{
			"AAVMF_CODE.secboot.fd",        // debian, EFI vars not protected
			"QEMU_EFI.secboot.testonly.fd", // Fedora, ref: https://bugzilla.redhat.com/show_bug.cgi?id=1882135
		}

		// Non-secure boot firmware files
		uefiSourceFilesInsecure := []string{
			"AAVMF_CODE.fd",
			"QEMU_EFI.fd",
			"OVMF.stateless.fd",
		}

		// Empty vars files
		uefiVarsFiles := []string{
			"AAVMF_VARS.fd",
			"QEMU_VARS.fd",
		}

		// Append extra search paths
		uefiSourcePathPrefixes = append(uefiSourcePathPrefixes, extraUEFISearchPaths...)

		uefiSourcePaths, uefiVarsPaths := generateUEFIPFlashList(uefiSourcePathPrefixes, uefiSourceFiles, uefiVarsFiles, uefiSourceFilesInsecure)

		return []PFlash{
			{
				Size:        64 * 1024 * 1024,
				SourcePaths: uefiSourcePaths,
			},
			{
				SourcePaths: uefiVarsPaths,
				Size:        64 * 1024 * 1024,
			},
		}
	case ArchAmd64:
		if !uefiEnabled {
			return nil
		}

		// Default search paths
		uefiSourcePathPrefixes := []string{
			"/usr/share/ovmf",
			"/usr/share/OVMF",
			"/usr/share/qemu",
			"/usr/share/ovmf/x64", // Arch Linux
		}

		// Secure boot enabled firmware files
		uefiSourceFiles := []string{
			"OVMF_CODE_4M.secboot.fd",
			"OVMF_CODE.secboot.4m.fd", // Arch Linux
			"OVMF_CODE.secboot.fd",
			"OVMF.secboot.fd",
			"edk2-x86_64-secure-code.fd", // Alpine Linux
			"ovmf-x86_64-ms-4m-code.bin",
		}

		// Non-secure boot firmware files
		uefiSourceFilesInsecure := []string{
			"OVMF_CODE_4M.fd",
			"OVMF_CODE.4m.fd", // Arch Linux
			"OVMF_CODE.fd",
			"OVMF.fd",
			"ovmf-x86_64-4m-code.bin",
		}

		// Empty vars files
		uefiVarsFiles := []string{
			"OVMF_VARS_4M.fd",
			"OVMF_VARS.4m.fd", // Arch Linux
			"OVMF_VARS.fd",
			"ovmf-x86_64-4m-vars.bin",
		}

		// Append extra search paths
		uefiSourcePathPrefixes = append(uefiSourcePathPrefixes, extraUEFISearchPaths...)

		uefiSourcePaths, uefiVarsPaths := generateUEFIPFlashList(uefiSourcePathPrefixes, uefiSourceFiles, uefiVarsFiles, uefiSourceFilesInsecure)

		return []PFlash{
			{
				Size:        0,
				SourcePaths: uefiSourcePaths,
			},
			{
				Size:        0,
				SourcePaths: uefiVarsPaths,
			},
		}
	default:
		return nil
	}
}

func generateUEFIPFlashList(uefiSourcePathPrefixes, uefiSourceFiles, uefiVarsFiles, uefiSourceFilesInsecure []string) (uefiSourcePaths, uefiVarsPaths []string) {
	for _, p := range uefiSourcePathPrefixes {
		for _, f := range uefiSourceFiles {
			uefiSourcePaths = append(uefiSourcePaths, filepath.Join(p, f))
		}

		for _, f := range uefiVarsFiles {
			uefiVarsPaths = append(uefiVarsPaths, filepath.Join(p, f))
		}
	}

	for _, p := range uefiSourcePathPrefixes {
		for _, f := range uefiSourceFilesInsecure {
			uefiSourcePaths = append(uefiSourcePaths, filepath.Join(p, f))
		}
	}

	return uefiSourcePaths, uefiVarsPaths
}

// QemuExecutable returns name of qemu executable for the arch.
func (arch Arch) QemuExecutable() string {
	binaries := []string{
		"qemu-system-" + arch.QemuArch(),
		"qemu-kvm",
		"/usr/libexec/qemu-kvm",
	}

	for _, binary := range binaries {
		if path, err := exec.LookPath(binary); err == nil {
			return path
		}
	}

	return ""
}

// TPMDeviceArgs returns arguments for qemu to enable TPM device.
func (arch Arch) TPMDeviceArgs(socketPath string) []string {
	tpmDeviceArgs := []string{
		"-chardev",
		fmt.Sprintf("socket,id=chrtpm,path=%s", socketPath),
		"-tpmdev",
		"emulator,id=tpm0,chardev=chrtpm",
		"-device",
	}

	switch arch {
	case ArchAmd64:
		return slices.Concat(tpmDeviceArgs, []string{"tpm-tis,tpmdev=tpm0"})
	case ArchArm64:
		return slices.Concat(tpmDeviceArgs, []string{"tpm-tis-device,tpmdev=tpm0"})
	default:
		panic("unsupported architecture")
	}
}

// KVMArgs returns arguments for qemu to enable KVM.
func (arch Arch) KVMArgs(kvmEnabled bool) []string {
	if !kvmEnabled {
		return []string{"-machine", arch.QemuMachine()}
	}

	machineArg := arch.QemuMachine() + ",accel=kvm"

	switch arch {
	case ArchAmd64:
		machineArg += ",smm=on"

		return []string{"-machine", machineArg}
	case ArchArm64:
		// smm is not supported on aarch64
		return []string{"-machine", machineArg}
	default:
		panic("unsupported architecture")
	}
}
