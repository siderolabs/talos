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
	"github.com/pkg/errors"

	"golang.org/x/sys/unix"
)

var (
	switchRoot *bool
)

func init() {
	switchRoot = flag.Bool("switch-root", false, "perform a switch_root")
	flag.Parse()
}

func kmsg(prefix string) (*os.File, error) {
	out, err := os.OpenFile("/dev/kmsg", os.O_RDWR|unix.O_CLOEXEC|unix.O_NONBLOCK|unix.O_NOCTTY, 0666)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open /dev/kmsg")
	}
	log.SetOutput(out)
	log.SetPrefix(prefix + " ")
	log.SetFlags(0)

	return out, nil
}

// nolint: gocyclo
func initram() error {
	// Read the special filesystems and populate the mount point definitions.
	if err := mount.InitSpecial(constants.NewRoot); err != nil {
		return err
	}
	// Setup logging to /dev/kmsg.
	var f *os.File
	f, err := kmsg("[talos] [initramfs]")
	if err != nil {
		return err
	}
	// Discover the platform.
	log.Println("discovering the platform")
	p, err := platform.NewPlatform()
	if err != nil {
		return err
	}
	// Retrieve the user data.
	log.Printf("retrieving the user data for the platform: %s", p.Name())
	data, err := p.UserData()
	if err != nil {
		return err
	}
	// Perform rootfs/datafs installation if defined
	if err := p.Install(data); err != nil {
		return err
	}
	// Read the block devices and populate the mount point definitions.
	if err := mount.InitBlock(constants.NewRoot); err != nil {
		return err
	}
	log.Printf("preparing the node for the platform: %s", p.Name())
	// Perform any tasks required by a particular platform.
	if err := p.Prepare(data); err != nil {
		return err
	}
	// Prepare the necessary files in the rootfs.
	log.Println("preparing the root filesystem")
	if err := rootfs.Prepare(constants.NewRoot, data); err != nil {
		return err
	}
	// Unmount the ROOT and DATA block devices.
	log.Println("unmounting the ROOT and DATA partitions")
	if err := mount.Unmount(); err != nil {
		return err
	}
	// Perform the equivalent of switch_root.
	log.Println("entering the new root")
	f.Close() // nolint: errcheck
	if err := switchroot.Switch(constants.NewRoot); err != nil {
		return err
	}

	return nil
}

func root() error {
	// Setup logging to /dev/kmsg.
	if _, err := kmsg("[talos]"); err != nil {
		return fmt.Errorf("failed to setup logging to /dev/kmsg: %v", err)
	}
	// Read the user data.
	log.Printf("reading the user data: %s\n", constants.UserDataPath)
	data, err := userdata.Open(constants.UserDataPath)
	if err != nil {
		return err
	}

	// Write any user specified files to disk.
	log.Println("writing the files specified in the user data to disk")
	if err := data.WriteFiles(); err != nil {
		return err
	}

	// Set the requested environment variables.
	log.Println("setting environment variables")
	for key, val := range data.Env {
		if err := os.Setenv(key, val); err != nil {
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

func recovery() {
	if r := recover(); r != nil {
		log.Printf("recovered from: %+v\n", r)
	}

	select {}
}

func main() {
	defer recovery()

	// TODO(andrewrynhard): Remove this and be explicit.
	if err := os.Setenv("PATH", constants.PATH); err != nil {
		panic(errors.New("error setting PATH"))
	}

	if *switchRoot {
		if err := root(); err != nil {
			panic(errors.Wrap(err, "boot failed"))
		}

		// Hang forever.
		select {}
	}

	if err := initram(); err != nil {
		panic(errors.Wrap(err, "early boot failed"))
	}

	// We should never reach this point if things are working as intended.
	panic(errors.New("unkown error"))
}
