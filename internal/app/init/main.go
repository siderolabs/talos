/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/containerd/containerd"
	criconstants "github.com/containerd/cri/pkg/constants"
	"github.com/pkg/errors"
	"github.com/talos-systems/talos/internal/app/init/internal/platform"
	"github.com/talos-systems/talos/internal/app/init/internal/rootfs"
	"github.com/talos-systems/talos/internal/app/init/internal/rootfs/mount"
	"github.com/talos-systems/talos/internal/app/init/pkg/network"
	"github.com/talos-systems/talos/internal/app/init/pkg/system"
	ctrdrunner "github.com/talos-systems/talos/internal/app/init/pkg/system/runner/containerd"
	"github.com/talos-systems/talos/internal/app/init/pkg/system/services"
	"github.com/talos-systems/talos/internal/pkg/constants"
	"github.com/talos-systems/talos/pkg/userdata"

	"golang.org/x/sys/unix"
)

var (
	switchRoot  *bool
	inContainer *bool
)

func init() {
	switchRoot = flag.Bool("switch-root", false, "perform a switch_root")
	inContainer = flag.Bool("in-container", false, "run Talos in a container")
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

func container() (err error) {
	log.Println("preparing container based deploy")

	log.Println("remounting volumes as shared mounts")
	targets := []string{"/", "/var/lib/kubelet", "/etc/cni"}
	for _, t := range targets {
		if err = unix.Mount("", t, "", unix.MS_SHARED, ""); err != nil {
			return err
		}
	}

	log.Printf("reading the user data: %s\n", constants.UserDataPath)
	var data *userdata.UserData
	if data, err = userdata.Open(constants.UserDataPath); err != nil {
		return err
	}

	log.Println("preparing the root filesystem")
	if err = rootfs.Prepare("", true, data); err != nil {
		return err
	}

	return nil
}

// nolint: gocyclo
func initram() (err error) {
	var initializer *mount.Initializer
	if initializer, err = mount.NewInitializer(constants.NewRoot); err != nil {
		return err
	}
	// Mount the special devices.
	if err = initializer.InitSpecial(); err != nil {
		return err
	}
	// Setup logging to /dev/kmsg.
	_, err = kmsg("[talos] [initramfs]")
	if err != nil {
		return err
	}
	// Discover the platform.
	log.Println("discovering the platform")
	var p platform.Platform
	if p, err = platform.NewPlatform(); err != nil {
		return err
	}
	log.Printf("platform is: %s", p.Name())
	// Setup basic network.
	if err = network.InitNetwork(); err != nil {
		return err
	}
	// Retrieve the user data.
	log.Printf("retrieving the user data")
	var data *userdata.UserData
	if data, err = p.UserData(); err != nil {
		log.Printf("encountered error reading userdata: %v", err)
		return err
	}
	// Setup custom network.
	if err = network.SetupNetwork(data); err != nil {
		return err
	}
	// Perform any tasks required by a particular platform.
	log.Printf("performing platform specific tasks")
	if err = p.Prepare(data); err != nil {
		return err
	}
	// Mount the owned partitions.
	log.Printf("mounting the partitions")
	if err = initializer.InitOwned(); err != nil {
		return err
	}
	// Install handles additional system setup
	if err = p.Install(data); err != nil {
		return err
	}
	// Prepare the necessary files in the rootfs.
	log.Println("preparing the root filesystem")
	if err = rootfs.Prepare(constants.NewRoot, false, data); err != nil {
		return err
	}
	// Perform the equivalent of switch_root.
	log.Println("entering the new root")
	if err = initializer.Switch(); err != nil {
		return err
	}

	return nil
}

func root() (err error) {
	if !*inContainer {
		// Setup logging to /dev/kmsg.
		if _, err = kmsg("[talos]"); err != nil {
			return fmt.Errorf("failed to setup logging to /dev/kmsg: %v", err)
		}
	}

	// Read the user data.
	log.Printf("reading the user data: %s\n", constants.UserDataPath)
	var data *userdata.UserData
	if data, err = userdata.Open(constants.UserDataPath); err != nil {
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

	go func() {
		if err := listenForPowerButton(); err != nil {
			log.Printf("WARNING: power off events will be ignored: %+v", err)
		}
	}()

	// Get a handle to the system services API.
	svcs := system.Services(data)

	// Start containerd.
	svcs.Start(&services.Containerd{})

	go startSystemServices(data)
	go startKubernetesServices(data)

	return nil
}

func startSystemServices(data *userdata.UserData) {
	var err error

	svcs := system.Services(data)

	// Import the system images.
	reqs := []*ctrdrunner.ImportRequest{
		{
			Path: "/usr/images/blockd.tar",
			Options: []containerd.ImportOpt{
				containerd.WithIndexName("talos/blockd"),
			},
		},
		{
			Path: "/usr/images/osd.tar",
			Options: []containerd.ImportOpt{
				containerd.WithIndexName("talos/osd"),
			},
		},
		{
			Path: "/usr/images/proxyd.tar",
			Options: []containerd.ImportOpt{
				containerd.WithIndexName("talos/proxyd"),
			},
		},
		{
			Path: "/usr/images/trustd.tar",
			Options: []containerd.ImportOpt{
				containerd.WithIndexName("talos/trustd"),
			},
		},
		{
			Path: "/usr/images/ntpd.tar",
			Options: []containerd.ImportOpt{
				containerd.WithIndexName("talos/ntpd"),
			},
		},
	}
	if err = ctrdrunner.Import(constants.SystemContainerdNamespace, reqs...); err != nil {
		panic(err)
	}

	log.Println("starting system services")
	// Start the services common to all nodes.
	svcs.Start(
		&services.Udevd{},
		&services.OSD{},
		&services.Blockd{},
		&services.NTPd{},
	)
	// Start the services common to all master nodes.
	if data.IsMaster() {
		svcs.Start(
			&services.Trustd{},
			&services.Proxyd{},
		)
	}

	// Launch dhclient
	// nolint: errcheck
	if data == nil || data.Networking == nil || data.Networking.OS == nil {
		network.DHCPd(network.DefaultInterface)
	} else {
		for _, netconf := range data.Networking.OS.Devices {
			if netconf.DHCP {
				network.DHCPd(netconf.Interface)
			}
		}
	}
}

func startKubernetesServices(data *userdata.UserData) {
	svcs := system.Services(data)

	// Import the Kubernetes images.

	reqs := []*ctrdrunner.ImportRequest{
		{
			Path: "/usr/images/hyperkube.tar",
		},
		{
			Path: "/usr/images/etcd.tar",
		},
		{
			Path: "/usr/images/coredns.tar",
		},
		{
			Path: "/usr/images/pause.tar",
		},
	}
	if err := ctrdrunner.Import(criconstants.K8sContainerdNamespace, reqs...); err != nil {
		panic(err)
	}

	log.Println("starting kubernetes services")
	svcs.Start(
		&services.Kubelet{},
		&services.Kubeadm{},
	)
}

func recovery() {
	if r := recover(); r != nil {
		log.Printf("recovered from: %+v\n", r)
		for i := 10; i >= 0; i-- {
			log.Printf("rebooting in %d seconds\n", i)
			time.Sleep(1 * time.Second)
		}

		// nolint: errcheck
		unix.Reboot(int(unix.LINUX_REBOOT_CMD_RESTART))
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

	if *inContainer {
		if err := container(); err != nil {
			panic(errors.Wrap(err, "failed to prepare container based deploy"))
		}
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
	panic(errors.New("unknown error"))
}
