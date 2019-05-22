/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package main

import (
	"encoding/base64"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pkg/errors"
	"github.com/talos-systems/talos/internal/app/init/internal/platform"
	"github.com/talos-systems/talos/internal/app/init/internal/reg"
	"github.com/talos-systems/talos/internal/app/init/internal/rootfs"
	"github.com/talos-systems/talos/internal/app/init/internal/rootfs/mount"
	"github.com/talos-systems/talos/internal/app/init/internal/security/kspp"
	"github.com/talos-systems/talos/internal/app/init/pkg/network"
	"github.com/talos-systems/talos/internal/app/init/pkg/system"
	"github.com/talos-systems/talos/internal/app/init/pkg/system/services"
	"github.com/talos-systems/talos/internal/pkg/constants"
	"github.com/talos-systems/talos/internal/pkg/grpc/factory"
	"github.com/talos-systems/talos/internal/pkg/kernel"
	"github.com/talos-systems/talos/pkg/userdata"

	"golang.org/x/sys/unix"

	kubeadmapi "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"
)

var (
	switchRoot  *bool
	inContainer *bool
	rebootFlag  = unix.LINUX_REBOOT_CMD_RESTART
	userdataArg *string
)

func init() {
	switchRoot = flag.Bool("switch-root", false, "perform a switch_root")
	inContainer = flag.Bool("in-container", false, "run Talos in a container")
	userdataArg = flag.String("userdata", "", "the base64 encoded userdata")
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

	if *userdataArg != "" {
		log.Printf("writing the user data: %s\n", constants.UserDataPath)
		var decoded []byte
		if decoded, err = base64.StdEncoding.DecodeString(*userdataArg); err != nil {
			return err
		}
		if err = ioutil.WriteFile(constants.UserDataPath, decoded, 0400); err != nil {
			return err
		}
	}

	var data *userdata.UserData
	if data, err = userdata.Open(constants.UserDataPath); err != nil {
		return err
	}

	// Workarounds for running in a container.

	data.Services.Kubeadm.IgnorePreflightErrors = []string{"FileContent--proc-sys-net-bridge-bridge-nf-call-iptables", "Swap", "SystemVerification"}
	initConfiguration, ok := data.Services.Kubeadm.Configuration.(*kubeadmapi.InitConfiguration)
	if ok {
		initConfiguration.ClusterConfiguration.ComponentConfigs.Kubelet.FailSwapOn = false
		// See https://github.com/kubernetes/kubernetes/issues/58610#issuecomment-359552443
		max := int32(0)
		maxPerCore := int32(0)
		initConfiguration.ClusterConfiguration.ComponentConfigs.KubeProxy.Conntrack.Max = &max
		initConfiguration.ClusterConfiguration.ComponentConfigs.KubeProxy.Conntrack.MaxPerCore = &maxPerCore
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
	// Enforce KSPP kernel parameters.
	log.Println("checking for KSPP kernel parameters")
	if err = kspp.EnforceKSPPKernelParameters(); err != nil {
		return err
	}
	// Setup hostname if provided.
	var hostname *string
	if hostname = kernel.Cmdline().Get(constants.KernelParamHostname).First(); hostname != nil {
		log.Println("setting hostname")
		if err = unix.Sethostname([]byte(*hostname)); err != nil {
			return err
		}
		log.Printf("hostname is: %s", *hostname)
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

// nolint: gocyclo
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

	// Mount the extra partitions.
	log.Printf("mounting the extra partitions")
	if err = mount.ExtraDevices(data); err != nil {
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

	poweroffCh, err := listenForPowerButton()
	if err != nil {
		log.Printf("WARNING: power off events will be ignored: %+v", err)
	}

	termCh := make(chan os.Signal, 1)
	signal.Notify(termCh, syscall.SIGTERM)

	// Get a handle to the system services API.
	svcs := system.Services(data)
	defer svcs.Shutdown()

	// Instantiate internal init API
	api := reg.NewRegistrator(data)
	server := factory.NewServer(api)
	listener, err := factory.NewListener(
		factory.Network("unix"),
		factory.SocketPath(constants.InitSocketPath),
	)
	if err != nil {
		panic(err)
	}
	defer server.Stop()

	go func() {
		// nolint: errcheck
		server.Serve(listener)
	}()

	startSystemServices(data)
	startKubernetesServices(data)

	select {
	case <-api.ShutdownCh:
		log.Printf("poweroff via API received")
		// poweroff, proceed to shutdown but mark as poweroff
		rebootFlag = unix.LINUX_REBOOT_CMD_POWER_OFF
	case <-poweroffCh:
		log.Printf("poweroff via ACPI")
		// poweroff, proceed to shutdown but mark as poweroff
		rebootFlag = unix.LINUX_REBOOT_CMD_POWER_OFF
	case <-termCh:
		log.Printf("SIGTERM received, rebooting...")
	case <-api.RebootCh:
		log.Printf("reboot via API received, rebooting...")
	}

	return nil
}

func startSystemServices(data *userdata.UserData) {
	svcs := system.Services(data)

	log.Println("starting system services")
	// Start the services common to all nodes.
	svcs.Start(
		&services.Containerd{},
		&services.Udevd{},
		&services.OSD{},
		&services.NTPd{},
	)
	// Start the services common to all master nodes.
	if data.Services.Kubeadm.IsControlPlane() {
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

	log.Println("starting kubernetes services")
	svcs.Start(
		&services.Kubelet{},
		&services.Kubeadm{},
	)
}

func sync() {
	syncdone := make(chan struct{})

	go func() {
		defer close(syncdone)
		unix.Sync()
	}()

	log.Printf("waiting for sync...")

	for i := 29; i >= 0; i-- {
		select {
		case <-syncdone:
			log.Printf("sync done")
			return
		case <-time.After(time.Second):
		}
		if i != 0 {
			log.Printf("waiting %d more seconds for sync to finish", i)
		}
	}

	log.Printf("sync hasn't completed in time, aborting...")
}

func reboot() {
	// See http://man7.org/linux/man-pages/man2/reboot.2.html.
	sync()

	// nolint: errcheck
	unix.Reboot(rebootFlag)

	if *inContainer {
		return
	}

	select {}
}

func recovery() {
	if r := recover(); r != nil {
		log.Printf("recovered from: %+v\n", r)
		for i := 10; i >= 0; i-- {
			log.Printf("rebooting in %d seconds\n", i)
			time.Sleep(1 * time.Second)
		}
	}
}

func main() {
	// This is main entrypoint into init() execution, after kernel boot control is passsed
	// to this function.
	//
	// When initram() finishes, it execs into itself with -switch-root flag, so control is passed
	// once again into this function.
	//
	// When init() terminates either on normal shutdown (reboot, poweroff), or due to panic, control
	// goes through recovery() and reboot() functions below, which finalize node state - sync buffers,
	// initiate poweroff or reboot. Also on shutdown, other deferred function are called, for example
	// services are gracefully shutdown.

	// on any return from init.main(), initiate host reboot or shutdown
	defer reboot()
	// handle any panics in the main goroutine, and proceed to reboot() above
	defer recovery()

	// TODO(andrewrynhard): Remove this and be explicit.
	if err := os.Setenv("PATH", constants.PATH); err != nil {
		panic(errors.New("error setting PATH"))
	}

	switch {
	case *switchRoot:
		if err := root(); err != nil {
			panic(errors.Wrap(err, "boot failed"))
		}

		// root() hangs until reboot
	case *inContainer:
		if err := container(); err != nil {
			panic(errors.Wrap(err, "failed to prepare container based deploy"))
		}
		if err := root(); err != nil {
			panic(errors.Wrap(err, "boot failed"))
		}

		// root() hangs until reboot
	default:
		if err := initram(); err != nil {
			panic(errors.Wrap(err, "early boot failed"))
		}

		// We should never reach this point if things are working as intended.
		panic(errors.New("unknown error"))
	}
}
