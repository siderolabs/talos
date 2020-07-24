// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package qemu

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/containernetworking/cni/libcni"
	"github.com/containernetworking/cni/pkg/types/current"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/containernetworking/plugins/pkg/testutils"
	"github.com/google/uuid"

	"github.com/talos-systems/talos/internal/pkg/cniutils"
	"github.com/talos-systems/talos/internal/pkg/inmemhttp"
	"github.com/talos-systems/talos/internal/pkg/provision"
	"github.com/talos-systems/talos/internal/pkg/provision/providers/vm"
)

// LaunchConfig is passed in to the Launch function over stdin.
type LaunchConfig struct {
	DiskPath        string
	VCPUCount       int64
	MemSize         int64
	QemuExecutable  string
	KernelImagePath string
	InitrdPath      string
	KernelArgs      string
	Config          string
	NetworkConfig   *libcni.NetworkConfigList
	CNI             provision.CNIConfig
	IP              net.IP
	CIDR            net.IPNet
	GatewayAddr     net.IP

	// filled by CNI invocation
	tapName string
	vmMAC   string
	ns      ns.NetNS

	// signals
	c chan os.Signal
}

// withCNI creates network namespace, launches CNI and passes control to the next function
// filling config with netNS and interface details.
func withCNI(config *LaunchConfig, f func(config *LaunchConfig) error) error {
	// random ID for the CNI, maps to single VM
	containerID := uuid.New().String()

	cniConfig := libcni.NewCNIConfigWithCacheDir(config.CNI.BinPath, config.CNI.CacheDir, nil)

	// create a network namespace
	ns, err := testutils.NewNS()
	if err != nil {
		return err
	}

	defer func() {
		ns.Close()              //nolint: errcheck
		testutils.UnmountNS(ns) //nolint: errcheck
	}()

	ones, _ := config.CIDR.Mask.Size()
	runtimeConf := libcni.RuntimeConf{
		ContainerID: containerID,
		NetNS:       ns.Path(),
		IfName:      "veth0",
		Args: [][2]string{
			{"IP", fmt.Sprintf("%s/%d", config.IP, ones)},
			{"GATEWAY", config.GatewayAddr.String()},
		},
	}

	ctx := context.Background()

	// attempt to clean up network in case it was deployed previously
	err = cniConfig.DelNetworkList(ctx, config.NetworkConfig, &runtimeConf)
	if err != nil {
		return fmt.Errorf("error deleting CNI network: %w", err)
	}

	res, err := cniConfig.AddNetworkList(ctx, config.NetworkConfig, &runtimeConf)
	if err != nil {
		return fmt.Errorf("error provisioning CNI network: %w", err)
	}

	defer func() {
		if e := cniConfig.DelNetworkList(ctx, config.NetworkConfig, &runtimeConf); e != nil {
			log.Printf("error cleaning up CNI: %s", e)
		}
	}()

	currentResult, err := current.NewResultFromResult(res)
	if err != nil {
		return fmt.Errorf("failed to parse cni result: %w", err)
	}

	vmIface, tapIface, err := cniutils.VMTapPair(currentResult, containerID)
	if err != nil {
		return fmt.Errorf(
			"failed to parse VM network configuration from CNI output, ensure CNI is configured with a plugin " +
				"that supports automatic VM network configuration such as tc-redirect-tap",
		)
	}

	config.tapName = tapIface.Name
	config.vmMAC = vmIface.Mac
	config.ns = ns

	return f(config)
}

// launchVM runs qemu with args built based on config.
func launchVM(config *LaunchConfig) error {
	args := []string{
		"-m", strconv.FormatInt(config.MemSize, 10),
		"-drive", fmt.Sprintf("format=raw,if=virtio,file=%s", config.DiskPath),
		"-smp", fmt.Sprintf("cpus=%d", config.VCPUCount),
		"-accel",
		"kvm",
		"-nographic",
		"-netdev", fmt.Sprintf("tap,id=net0,ifname=%s,script=no,downscript=no", config.tapName),
		"-device", fmt.Sprintf("virtio-net-pci,netdev=net0,mac=%s", config.vmMAC),
	}

	disk, err := os.Open(config.DiskPath)
	if err != nil {
		return fmt.Errorf("failed to open disk file %w", err)
	}

	// check if disk is empty
	checkSize := 512
	buf := make([]byte, checkSize)

	_, err = disk.Read(buf)
	if err != nil {
		return fmt.Errorf("failed to read disk file %w", err)
	}

	if bytes.Equal(buf, make([]byte, checkSize)) {
		args = append(args,
			"-kernel", config.KernelImagePath,
			"-initrd", config.InitrdPath,
			"-append", config.KernelArgs,
			"-no-reboot",
		)
	}

	fmt.Fprintf(os.Stderr, "starting qemu with args:\n%s\n", strings.Join(args, " "))
	cmd := exec.Command(
		config.QemuExecutable,
		args...,
	)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := ns.WithNetNSPath(config.ns.Path(), func(_ ns.NetNS) error {
		return cmd.Start()
	}); err != nil {
		return err
	}

	done := make(chan error)

	go func() {
		done <- cmd.Wait()
	}()

	select {
	case sig := <-config.c:
		fmt.Fprintf(os.Stderr, "exiting VM as signal %s was received\n", sig)

		if err := cmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill process %w", err)
		}

		return fmt.Errorf("process stopped")
	case err := <-done:
		if err != nil {
			return fmt.Errorf("process exited with error %s", err)
		}

		// graceful exit
		return nil
	}
}

// Launch a control process around qemu VM manager.
//
// This function is invoked from 'talosctl qemu-launch' hidden command
// and wraps starting, controlling 'qemu' VM process.
//
// Launch restarts VM forever until control process is stopped itself with a signal.
//
// Process is expected to receive configuration on stdin. Current working directory
// should be cluster state directory, process output should be redirected to the
// logfile in state directory.
//
// When signals SIGINT, SIGTERM are received, control process stops qemu and exits.
//
//nolint: gocyclo
func Launch() error {
	var config LaunchConfig

	if err := vm.ReadConfig(&config); err != nil {
		return err
	}

	config.c = vm.ConfigureSignals()

	httpServer, err := inmemhttp.NewServer(fmt.Sprintf("%s:0", config.GatewayAddr))
	if err != nil {
		return fmt.Errorf("error launching in-memory HTTP server: %w", err)
	}

	if err = httpServer.AddFile("config.yaml", []byte(config.Config)); err != nil {
		return err
	}

	httpServer.Serve()
	defer httpServer.Shutdown(context.Background()) //nolint: errcheck

	// patch kernel args
	config.KernelArgs = strings.ReplaceAll(config.KernelArgs, "{TALOS_CONFIG_URL}", fmt.Sprintf("http://%s/config.yaml", httpServer.GetAddr()))

	for {
		if err := withCNI(&config, launchVM); err != nil {
			return err
		}
	}
}
