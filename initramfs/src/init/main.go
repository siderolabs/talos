// +build linux

package main

import "C"

import (
	"flag"
	"log"
	"os"

	"github.com/autonomy/dianemo/initramfs/src/init/pkg/constants"
	"github.com/autonomy/dianemo/initramfs/src/init/pkg/mount"
	"github.com/autonomy/dianemo/initramfs/src/init/pkg/rootfs"
	"github.com/autonomy/dianemo/initramfs/src/init/pkg/server"
	"github.com/autonomy/dianemo/initramfs/src/init/pkg/service"
	"github.com/autonomy/dianemo/initramfs/src/init/pkg/switchroot"
	"github.com/autonomy/dianemo/initramfs/src/init/pkg/userdata"
)

var (
	switchRoot *bool
)

func hang() {
	if rec := recover(); rec != nil {
		err, ok := rec.(error)
		if ok {
			log.Printf("%s\n", err.Error())
		}
	}
	// Hang forever to avoid a kernel panic.
	select {}
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
	if err = mount.Init(constants.NewRoot); err != nil {
		return
	}
	// Download the user data.
	data, err := userdata.Download()
	if err != nil {
		return
	}
	// Prepare the necessary files in the rootfs.
	if err = rootfs.Prepare(constants.NewRoot, data); err != nil {
		return
	}
	// Unmount the ROOT and DATA block devices
	if err = mount.Unmount(); err != nil {
		return
	}
	// Perform the equivalent of switch_root.
	if err = switchroot.Switch(constants.NewRoot); err != nil {
		return
	}

	return nil
}

func root() (err error) {
	// Download the user data.
	data, err := userdata.Download()
	if err != nil {
		return
	}

	// Start the services essential to running Kubernetes.
	services := &service.Manager{
		UserData: data,
	}

	switch data.Kubernetes.ContainerRuntime {
	case constants.ContainerRuntimeDocker:
		services.Start(&service.Docker{})
	case constants.ContainerRuntimeCRIO:
		services.Start(&service.CRIO{})
	default:
		services.Start(&service.CRIO{})
	}
	services.Start(&service.Kubelet{})
	services.Start(&service.Kubeadm{})

	return server.NewServer(data.OS.Security).Listen()
}

func main() {
	defer hang()

	if !*switchRoot {
		if err := initram(); err != nil {
			panic(err)
		}
	}

	if err := root(); err != nil {
		panic(err)
	}
}
