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
	os.Setenv("PATH", constants.PATH)

	switchRoot = flag.Bool("switch-root", false, "perform a switch_root")
	flag.Parse()
}

func main() {
	defer hang()
	if !*switchRoot {
		// Read the block devices and populate the mount point definitions.
		if err := mount.Init(constants.NewRoot); err != nil {
			panic(err)
		}
		// Download the user data.
		data, err := userdata.Download()
		if err != nil {
			panic(err)
		}
		// Prepare the necessary files in the rootfs.
		if err := rootfs.Prepare(constants.NewRoot, data); err != nil {
			panic(err)
		}
		// Unmount the ROOT and DATA block devices
		if err := mount.Unmount(); err != nil {
			panic(err)
		}
		// Perform the equivalent of switch_root.
		if err := switchroot.Switch(constants.NewRoot); err != nil {
			panic(err)
		}
	}

	// Download the user data.
	data, err := userdata.Download()
	if err != nil {
		panic(err)
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

	s, err := server.NewServer(data.OS.Security)
	if err != nil {
		panic(err)
	}
	s.Listen()
}
