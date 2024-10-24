// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package init implements booting process.
package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/klauspost/cpuid/v2"
	"github.com/siderolabs/go-kmsg"
	"github.com/siderolabs/go-procfs/procfs"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/pkg/mount/switchroot"
	"github.com/siderolabs/talos/internal/pkg/mount/v2"
	"github.com/siderolabs/talos/internal/pkg/rng"
	"github.com/siderolabs/talos/internal/pkg/secureboot"
	"github.com/siderolabs/talos/internal/pkg/secureboot/tpm2"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/extensions"
	"github.com/siderolabs/talos/pkg/machinery/version"
)

func init() {
	// Explicitly disable memory profiling to save around 1.4MiB of memory.
	runtime.MemProfileRate = 0
}

func run() error {
	// Mount the pseudo devices.
	pseudoMountPoints := mount.Pseudo()

	if _, err := pseudoMountPoints.Mount(); err != nil {
		return err
	}

	// Setup logging to /dev/kmsg.
	if err := kmsg.SetupLogger(nil, "[talos] [initramfs]", nil); err != nil {
		return err
	}

	// Seed RNG.
	if err := rng.TPMSeed(); err != nil {
		// not making this fatal error
		log.Printf("failed to seed from the TPM: %s", err)
	}

	// extend PCR 11 with enter-initrd
	if err := tpm2.PCRExtend(secureboot.UKIPCR, []byte(secureboot.EnterInitrd)); err != nil {
		return fmt.Errorf("failed to extend PCR %d with enter-initrd: %v", secureboot.UKIPCR, err)
	}

	log.Printf("booting Talos %s", version.Tag)

	cpuInfo()

	// Mount the rootfs.
	if err := mountRootFS(); err != nil {
		return err
	}

	// Bind mount the lib/firmware if needed.
	if err := bindMountFirmware(); err != nil {
		return err
	}

	// Bind mount /.extra if needed.
	if err := bindMountExtra(); err != nil {
		return err
	}

	// Switch into the new rootfs.
	log.Println("entering the rootfs")

	return switchroot.Switch(constants.NewRoot, pseudoMountPoints)
}

func recovery() {
	// If panic is set in the kernel flags, we'll hang instead of rebooting.
	// But we still allow users to hit CTRL+ALT+DEL to try and restart when they're ready.
	// Listening for these signals also keep us from deadlocking the goroutine.
	if r := recover(); r != nil {
		log.Printf("recovered from: %+v\n", r)

		p := procfs.ProcCmdline().Get(constants.KernelParamPanic).First()
		if p != nil && *p == "0" {
			log.Printf("panic=0 kernel flag found. sleeping forever")

			exitSignal := make(chan os.Signal, 1)
			signal.Notify(exitSignal, syscall.SIGINT, syscall.SIGTERM)
			<-exitSignal
		}

		for i := 10; i >= 0; i-- {
			log.Printf("rebooting in %d seconds\n", i)
			time.Sleep(1 * time.Second)
		}
	}

	//nolint:errcheck
	unix.Reboot(unix.LINUX_REBOOT_CMD_RESTART)
}

//nolint:gocyclo
func mountRootFS() error {
	log.Println("mounting the rootfs")

	var extensionsConfig extensions.Config

	if err := extensionsConfig.Read(constants.ExtensionsConfigFile); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
	}

	// if no extensions found use plain squashfs mount
	if len(extensionsConfig.Layers) == 0 {
		squashfs, err := mount.Squashfs(constants.NewRoot, "/"+constants.RootfsAsset)
		if err != nil {
			return err
		}

		_, err = squashfs.Mount()

		return err
	}

	// otherwise compose overlay mounts
	type layer struct {
		name  string
		image string
	}

	var (
		layers         []layer
		squashfsPoints mount.Points
	)

	// going in the inverse order as earlier layers are overlayed on top of the latter ones
	for i := len(extensionsConfig.Layers) - 1; i >= 0; i-- {
		layers = append(layers, layer{
			name:  fmt.Sprintf("layer%d", i),
			image: extensionsConfig.Layers[i].Image,
		})

		log.Printf("enabling system extension %s %s", extensionsConfig.Layers[i].Metadata.Name, extensionsConfig.Layers[i].Metadata.Version)
	}

	layers = append(layers, layer{
		name:  "root",
		image: "/" + constants.RootfsAsset,
	})

	overlays := make([]string, 0, len(layers))

	for _, layer := range layers {
		point, err := mount.Squashfs(filepath.Join(constants.ExtensionLayers, layer.name), layer.image)
		if err != nil {
			return err
		}

		squashfsPoints = append(squashfsPoints, point)
	}

	squashfsUnmounter, err := squashfsPoints.Mount()
	if err != nil {
		return err
	}

	overlayPoint := mount.NewReadonlyOverlay(overlays, constants.NewRoot, mount.WithShared(), mount.WithFlags(unix.MS_I_VERSION))

	_, err = overlayPoint.Mount()
	if err != nil {
		return err
	}

	if err = squashfsUnmounter(); err != nil {
		return err
	}

	return unix.Mount(constants.ExtensionsConfigFile, filepath.Join(constants.NewRoot, constants.ExtensionsRuntimeConfigFile), "", unix.MS_BIND|unix.MS_RDONLY, "")
}

func bindMountFirmware() error {
	if _, err := os.Stat(constants.FirmwarePath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return err
	}

	log.Printf("bind mounting %s", constants.FirmwarePath)

	return unix.Mount(constants.FirmwarePath, filepath.Join(constants.NewRoot, constants.FirmwarePath), "", unix.MS_BIND|unix.MS_RDONLY, "")
}

func bindMountExtra() error {
	if _, err := os.Stat(constants.SDStubDynamicInitrdPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return err
	}

	log.Printf("bind mounting %s", constants.SDStubDynamicInitrdPath)

	return unix.Mount(constants.SDStubDynamicInitrdPath, filepath.Join(constants.NewRoot, constants.SDStubDynamicInitrdPath), "", unix.MS_BIND|unix.MS_RDONLY, "")
}

func cpuInfo() {
	log.Printf("CPU: %s, %d core(s), %d thread(s) per core", cpuid.CPU.BrandName, cpuid.CPU.PhysicalCores, cpuid.CPU.ThreadsPerCore)

	if runtime.GOARCH == "amd64" {
		log.Printf("x86_64 microarchitecture level: %d", cpuid.CPU.X64Level())

		if cpuid.CPU.X64Level() < constants.MinimumGOAMD64Level {
			if cpuid.CPU.VM() {
				log.Printf("it might be that the VM is configured with an older CPU model, please check the VM configuration")
			}

			log.Printf("x86_64 microarchitecture level %d or higher is required, halting", constants.MinimumGOAMD64Level)

			time.Sleep(365 * 24 * time.Hour)
		}
	}
}

func main() {
	defer recovery()

	if err := run(); err != nil {
		panic(fmt.Errorf("early boot failed: %w", err))
	}

	// We should never reach this point if things are working as intended.
	panic(errors.New("unknown error"))
}
