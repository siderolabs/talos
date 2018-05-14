// +build linux

package main

import "C"

import (
	"flag"
	"log"
	"os"

	"github.com/autonomy/dianemo/initramfs/cmd/init/pkg/constants"
	"github.com/autonomy/dianemo/initramfs/cmd/init/pkg/mount"
	"github.com/autonomy/dianemo/initramfs/cmd/init/pkg/rootfs"
	"github.com/autonomy/dianemo/initramfs/cmd/init/pkg/service"
	"github.com/autonomy/dianemo/initramfs/cmd/init/pkg/switchroot"
	"github.com/autonomy/dianemo/initramfs/pkg/userdata"
)

var (
	switchRoot *bool
)

func recovery() {
	if r := recover(); r != nil {
		log.Printf("recovered from: %v\n", r)
	}
}

func init() {
	log.SetFlags(log.Lshortfile | log.Ldate | log.Lmicroseconds | log.Ltime)
	if err := os.Setenv("PATH", constants.PATH); err != nil {
		panic(err)
	}

	switchRoot = flag.Bool("switch-root", false, "perform a switch_root")
	flag.Parse()
}

func initram() (err error) {
	// Read the block devices and populate the mount point definitions.
	log.Println("initializing mount points")
	if err = mount.Init(constants.NewRoot); err != nil {
		return
	}
	// Download the user data.
	log.Println("downloading the user data")
	data, err := userdata.Download()
	if err != nil {
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
	// Read the user data.
	log.Println("reading the user data")
	data, err := userdata.Open(constants.UserDataPath)
	if err != nil {
		return
	}

	services := &service.Manager{
		UserData: *data,
	}

	// Start the services essential to managing the node.
	log.Println("starting OS services")
	services.Start(&service.OSD{})
	if data.Kubernetes.Init {
		services.Start(&service.ROTD{})
	}

	// Start the services essential to running Kubernetes.
	log.Println("starting Kubernetes services")
	switch data.Kubernetes.ContainerRuntime {
	case constants.ContainerRuntimeDocker:
		services.Start(&service.Docker{})
	case constants.ContainerRuntimeCRIO:
		fallthrough
	default:
		services.Start(&service.CRIO{})
	}
	services.Start(&service.Kubelet{})
	services.Start(&service.Kubeadm{})

	return nil
}

func main() {
	defer recovery()

	if *switchRoot {
		if err := root(); err != nil {
			panic(err)
		}
		select {}
	}

	if err := initram(); err != nil {
		panic(err)
	}

	// We should only reach this point if something within initram() fails.
	select {}
}
