// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/talos-systems/go-kmsg"
	"github.com/talos-systems/go-procfs/procfs"
	"golang.org/x/sys/unix"
	"gopkg.in/freddierice/go-losetup.v1"

	"github.com/talos-systems/talos/internal/pkg/mount"
	"github.com/talos-systems/talos/internal/pkg/mount/switchroot"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/machinery/extensions"
	"github.com/talos-systems/talos/pkg/version"
)

func init() {
	// Explicitly disable memory profiling to save around 1.4MiB of memory.
	runtime.MemProfileRate = 0
}

func run() (err error) {
	// Mount the pseudo devices.
	pseudo, err := mount.PseudoMountPoints()
	if err != nil {
		return err
	}

	if err = mount.Mount(pseudo); err != nil {
		return err
	}

	// Setup logging to /dev/kmsg.
	err = kmsg.SetupLogger(nil, "[talos] [initramfs]", nil)
	if err != nil {
		return err
	}

	log.Printf("booting Talos %s", version.Tag)

	// Mount the rootfs.
	if err = mountRootFS(); err != nil {
		return err
	}

	// Bind mount the lib/firmware if needed.
	if err = bindMountFirmware(); err != nil {
		return err
	}

	// Switch into the new rootfs.
	log.Println("entering the rootfs")

	return switchroot.Switch(constants.NewRoot, pseudo)
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
		squashfs, err := mount.SquashfsMountPoints(constants.NewRoot)
		if err != nil {
			return err
		}

		return mount.Mount(squashfs)
	}

	// otherwise compose overlay mounts
	type layer struct {
		name  string
		image string
	}

	layers := []layer{}

	squashfs := mount.NewMountPoints()

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
		dev, err := losetup.Attach(layer.image, 0, true)
		if err != nil {
			return err
		}

		p := mount.NewMountPoint(dev.Path(), "/"+layer.name, "squashfs", unix.MS_RDONLY|unix.MS_I_VERSION, "", mount.WithPrefix(constants.ExtensionLayers), mount.WithFlags(mount.ReadOnly|mount.Shared))

		overlays = append(overlays, p.Target())
		squashfs.Set(layer.name, p)
	}

	if err := mount.Mount(squashfs); err != nil {
		return err
	}

	overlay := mount.NewMountPoints()
	overlay.Set(constants.NewRoot, mount.NewMountPoint(strings.Join(overlays, ":"), constants.NewRoot, "", unix.MS_I_VERSION, "", mount.WithFlags(mount.ReadOnly|mount.ReadonlyOverlay|mount.Shared)))

	if err := mount.Mount(overlay); err != nil {
		return err
	}

	if err := mount.Unmount(squashfs); err != nil {
		return err
	}

	if err := unix.Mount(constants.ExtensionsConfigFile, filepath.Join(constants.NewRoot, constants.ExtensionsRuntimeConfigFile), "", unix.MS_BIND|unix.MS_RDONLY, ""); err != nil {
		return err
	}

	return nil
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

func main() {
	defer recovery()

	if err := run(); err != nil {
		panic(fmt.Errorf("early boot failed: %w", err))
	}

	// We should never reach this point if things are working as intended.
	panic(errors.New("unknown error"))
}
