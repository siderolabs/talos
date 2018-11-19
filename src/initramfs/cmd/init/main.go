// +build linux

package main

import "C"

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/autonomy/talos/src/initramfs/cmd/init/pkg/constants"
	"github.com/autonomy/talos/src/initramfs/cmd/init/pkg/mount"
	"github.com/autonomy/talos/src/initramfs/cmd/init/pkg/platform"
	"github.com/autonomy/talos/src/initramfs/cmd/init/pkg/rootfs"
	"github.com/autonomy/talos/src/initramfs/cmd/init/pkg/switchroot"
	"github.com/autonomy/talos/src/initramfs/cmd/init/pkg/system"
	"github.com/autonomy/talos/src/initramfs/cmd/init/pkg/system/services"
	"github.com/autonomy/talos/src/initramfs/pkg/userdata"

	"golang.org/x/sys/unix"
)

var (
	switchRoot *bool
)

func recovery() {
	// We should only reach this point if something within initram() fails.
	if r := recover(); r != nil {
		log.Printf("recovered from: %v\n", r)
	}

	select {}
}

func init() {
	switchRoot = flag.Bool("switch-root", false, "perform a switch_root")
	flag.Parse()
}

func kmsg(prefix string) error {
	out, err := os.OpenFile("/dev/kmsg", os.O_RDWR|unix.O_CLOEXEC|unix.O_NONBLOCK|unix.O_NOCTTY, 0666)
	if err != nil {
		return fmt.Errorf("failed to open /dev/kmsg: %v", err)
	}
	log.SetOutput(out)
	log.SetPrefix(prefix + " ")
	log.SetFlags(0)

	return nil
}

func initram() (err error) {
	// Read the block devices and populate the mount point definitions.
	if err = mount.Init(constants.NewRoot); err != nil {
		return
	}
	// Setup logging to /dev/kmsg.
	// NB: We should not attempt to open /dev/kmsg until after /dev is mounted.
	if err = kmsg("[talos] [initramfs]"); err != nil {
		return
	}
	// Discover the platform.
	log.Println("discovering the platform")
	p, err := platform.NewPlatform()
	if err != nil {
		return
	}
	// Retrieve the user data.
	log.Printf("retrieving the user data for the platform: %s", p.Name())
	data, err := p.UserData()
	if err != nil {
		return
	}
	log.Printf("preparing the node for the platform: %s", p.Name())
	// Perform any tasks required by a particular platform.
	if err = p.Prepare(data); err != nil {
		return
	}
	// Prepare the necessary files in the rootfs.
	log.Println("preparing the root filesystem")
	if err = rootfs.Prepare(constants.NewRoot, data); err != nil {
		return
	}
	// Unmount the ROOT and DATA block devices.
	log.Println("unmounting the ROOT and DATA partitions")
	if err = mount.Unmount(); err != nil {
		return
	}
	// Perform the equivalent of switch_root.
	log.Println("entering the new root")
	if err = switchroot.Switch(constants.NewRoot); err != nil {
		return
	}

	return nil
}

func root() (err error) {
	// Setup logging to /dev/kmsg.
	if err = kmsg("[talos]"); err != nil {
		return
	}
	// Read the user data.
	log.Printf("reading the user data: %s\n", constants.UserDataPath)
	data, err := userdata.Open(constants.UserDataPath)
	if err != nil {
		return err
	}

	// Write any user specified files to disk.
	log.Println("writing the files specified in the user data to disk")
	if err = data.WriteFiles(); err != nil {
		return err
	}

	// Set the requested environment variables.
	log.Println("setting environment variables")
	for key, val := range data.Env {
		if err = os.Setenv(key, val); err != nil {
			log.Printf("WARNING failed to set enivronment variable: %v", err)
		}
	}

	// Get a handle to the system services API.
	systemservices := system.Services(data)

	// Start the services common to all nodes.
	log.Println("starting node services")
	systemservices.Start(
		&services.Containerd{},
		&services.CRT{},
		&services.OSD{},
		&services.Blockd{},
		&services.Kubelet{},
		&services.Kubeadm{},
	)

	// Start the services common to all master nodes.
	if data.IsMaster() {
		log.Println("starting master services")
		systemservices.Start(
			&services.Trustd{},
			&services.Proxyd{},
		)
	}

	return nil
}

func main() {
	defer recovery()

	// TODO(andrewrynhard): Remove this and be explicit.
	if err := os.Setenv("PATH", constants.PATH); err != nil {
		panic(err)
	}

	if *switchRoot {
		if err := root(); err != nil {
			panic(err)
		}
	}

	if err := initram(); err != nil {
		panic(err)
	}
}
