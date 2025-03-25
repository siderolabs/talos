// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package qemu

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/alexflint/go-filemutex"
	"github.com/containernetworking/cni/libcni"
	"github.com/containernetworking/cni/pkg/types"
	types100 "github.com/containernetworking/cni/pkg/types/100"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/containernetworking/plugins/pkg/testutils"
	"github.com/containernetworking/plugins/pkg/utils"
	"github.com/coreos/go-iptables/iptables"
	"github.com/google/uuid"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-blockdevice/v2/blkid"
	sideronet "github.com/siderolabs/net"

	"github.com/siderolabs/talos/pkg/provision"
	"github.com/siderolabs/talos/pkg/provision/internal/cniutils"
	"github.com/siderolabs/talos/pkg/provision/providers/vm"
)

// LaunchConfig is passed in to the Launch function over stdin.
type LaunchConfig struct {
	StatePath string

	// VM options
	DiskPaths         []string
	DiskDrivers       []string
	DiskBlockSizes    []uint
	VCPUCount         int64
	MemSize           int64
	KernelImagePath   string
	InitrdPath        string
	ISOPath           string
	USBPath           string
	UKIPath           string
	ExtraISOPath      string
	PFlashImages      []string
	KernelArgs        string
	MonitorPath       string
	DefaultBootOrder  string
	EnableKVM         bool
	BootloaderEnabled bool
	TPM2Config        tpm2Config
	NodeUUID          uuid.UUID
	BadRTC            bool
	ArchitectureData  Arch
	WithDebugShell    bool
	IOMMUEnabled      bool

	// Talos config
	Config string

	// Network
	BridgeName        string
	NetworkConfig     *libcni.NetworkConfigList
	CNI               provision.CNIConfig
	IPs               []netip.Addr
	CIDRs             []netip.Prefix
	NoMasqueradeCIDRs []netip.Prefix
	Hostname          string
	GatewayAddrs      []netip.Addr
	MTU               int
	Nameservers       []netip.Addr

	// PXE
	TFTPServer       string
	BootFilename     string
	IPXEBootFileName string

	// API
	APIPort int

	// sd-stub
	sdStubExtraCmdline       string
	sdStubExtraCmdlineConfig string

	// filled by CNI invocation
	tapName string
	vmMAC   string
	ns      ns.NetNS

	// signals
	c chan os.Signal

	// controller
	controller *Controller
}

type tpm2Config struct {
	NodeName string
	StateDir string
}

// withCNIOperationLocked ensures that CNI operations don't run concurrently.
//
// There are race conditions in the CNI plugins that can cause a failure if called concurrently.
func withCNIOperationLocked[T any](config *LaunchConfig, f func() (T, error)) (T, error) {
	var zeroT T

	lock, err := filemutex.New(filepath.Join(config.StatePath, "cni.lock"))
	if err != nil {
		return zeroT, fmt.Errorf("failed to create CNI lock: %w", err)
	}

	if err = lock.Lock(); err != nil {
		return zeroT, fmt.Errorf("failed to acquire CNI lock: %w", err)
	}

	defer func() {
		if err := lock.Close(); err != nil {
			log.Printf("failed to release CNI lock: %s", err)
		}
	}()

	return f()
}

// withCNIOperationLockedNoResult ensures that CNI operations don't run concurrently.
func withCNIOperationLockedNoResult(config *LaunchConfig, f func() error) error {
	_, err := withCNIOperationLocked(config, func() (struct{}, error) {
		return struct{}{}, f()
	})

	return err
}

// withCNI creates network namespace, launches CNI and passes control to the next function
// filling config with netNS and interface details.
//
//nolint:gocyclo
func withCNI(ctx context.Context, config *LaunchConfig, f func(config *LaunchConfig) error) error {
	// random ID for the CNI, maps to single VM
	containerID := uuid.New().String()

	cniConfig := libcni.NewCNIConfigWithCacheDir(config.CNI.BinPath, config.CNI.CacheDir, nil)

	// create a network namespace
	ns, err := testutils.NewNS()
	if err != nil {
		return err
	}

	defer func() {
		ns.Close()              //nolint:errcheck
		testutils.UnmountNS(ns) //nolint:errcheck
	}()

	ips := make([]string, len(config.IPs))
	for j := range ips {
		ips[j] = sideronet.FormatCIDR(config.IPs[j], config.CIDRs[j])
	}

	gatewayAddrs := xslices.Map(config.GatewayAddrs, netip.Addr.String)

	runtimeConf := libcni.RuntimeConf{
		ContainerID: containerID,
		NetNS:       ns.Path(),
		IfName:      "veth0",
		Args: [][2]string{
			{"IP", strings.Join(ips, ",")},
			{"GATEWAY", strings.Join(gatewayAddrs, ",")},
			{"IgnoreUnknown", "1"},
		},
	}

	// attempt to clean up network in case it was deployed previously
	err = withCNIOperationLockedNoResult(
		config,
		func() error {
			return cniConfig.DelNetworkList(ctx, config.NetworkConfig, &runtimeConf)
		},
	)
	if err != nil {
		return fmt.Errorf("error deleting CNI network: %w", err)
	}

	res, err := withCNIOperationLocked(
		config,
		func() (types.Result, error) {
			return cniConfig.AddNetworkList(ctx, config.NetworkConfig, &runtimeConf)
		},
	)
	if err != nil {
		return fmt.Errorf("error provisioning CNI network: %w", err)
	}

	defer func() {
		if e := withCNIOperationLockedNoResult(
			config,
			func() error {
				return cniConfig.DelNetworkList(ctx, config.NetworkConfig, &runtimeConf)
			},
		); e != nil {
			log.Printf("error cleaning up CNI: %s", e)
		}
	}()

	currentResult, err := types100.NewResultFromResult(res)
	if err != nil {
		return fmt.Errorf("failed to parse cni result: %w", err)
	}

	vmIface, tapIface, err := cniutils.VMTapPair(currentResult, containerID)
	if err != nil {
		return errors.New(
			"failed to parse VM network configuration from CNI output, ensure CNI is configured with a plugin " +
				"that supports automatic VM network configuration such as tc-redirect-tap")
	}

	cniChain := utils.FormatChainName(config.NetworkConfig.Name, containerID)

	ipt, err := iptables.New()
	if err != nil {
		return fmt.Errorf("failed to initialize iptables: %w", err)
	}

	// don't masquerade traffic with "broadcast" destination from the VM
	//
	// no need to clean up the rule, as CNI drops the whole chain
	if err = ipt.InsertUnique("nat", cniChain, 1, "--destination", "255.255.255.255/32", "-j", "ACCEPT"); err != nil {
		return fmt.Errorf("failed to insert iptables rule to allow broadcast traffic: %w", err)
	}

	for _, cidr := range config.NoMasqueradeCIDRs {
		if err = ipt.InsertUnique("nat", cniChain, 1, "--destination", cidr.String(), "-j", "ACCEPT"); err != nil {
			return fmt.Errorf("failed to insert iptables rule to allow non-masquerade traffic to cidr %q: %w", cidr.String(), err)
		}
	}

	config.tapName = tapIface.Name
	config.vmMAC = vmIface.Mac
	config.ns = ns

	for j := range config.CIDRs {
		nameservers := make([]netip.Addr, 0, len(config.Nameservers))

		// filter nameservers by IPv4/IPv6 matching IPs
		for i := range config.Nameservers {
			if config.IPs[j].Is6() {
				if config.Nameservers[i].Is6() {
					nameservers = append(nameservers, config.Nameservers[i])
				}
			} else {
				if config.Nameservers[i].Is4() {
					nameservers = append(nameservers, config.Nameservers[i])
				}
			}
		}

		// dump node IP/mac/hostname for dhcp
		if err = vm.DumpIPAMRecord(config.StatePath, vm.IPAMRecord{
			IP:               config.IPs[j],
			Netmask:          byte(config.CIDRs[j].Bits()),
			MAC:              vmIface.Mac,
			Hostname:         config.Hostname,
			Gateway:          config.GatewayAddrs[j],
			MTU:              config.MTU,
			Nameservers:      nameservers,
			TFTPServer:       config.TFTPServer,
			IPXEBootFilename: config.IPXEBootFileName,
		}); err != nil {
			return err
		}
	}

	return f(config)
}

func checkPartitions(config *LaunchConfig) (bool, error) {
	info, err := blkid.ProbePath(config.DiskPaths[0], blkid.WithSectorSize(config.DiskBlockSizes[0]))
	if err != nil {
		return false, fmt.Errorf("error probing disk: %w", err)
	}

	return info.Name == "gpt" && len(info.Parts) > 0, nil
}

// launchVM runs qemu with args built based on config.
//
//nolint:gocyclo,cyclop
func launchVM(config *LaunchConfig) error {
	bootOrder := config.DefaultBootOrder

	if config.controller.ForcePXEBoot() {
		bootOrder = "nc"
	}

	cpuArg := "max"

	if config.BadRTC {
		cpuArg += ",-kvmclock"
	}

	args := []string{
		"-m", strconv.FormatInt(config.MemSize, 10),
		"-smp", fmt.Sprintf("cpus=%d", config.VCPUCount),
		"-cpu", cpuArg,
		"-nographic",
		"-netdev", fmt.Sprintf("tap,id=net0,ifname=%s,script=no,downscript=no", config.tapName),
		"-device", fmt.Sprintf("virtio-net-pci,netdev=net0,mac=%s", config.vmMAC),
		// TODO: uncomment the following line to get another eth interface not connected to anything
		// "-nic", "tap,model=virtio-net-pci",
		"-device", "virtio-rng-pci",
		"-device", "virtio-balloon,deflate-on-oom=on",
		"-monitor", fmt.Sprintf("unix:%s,server,nowait", config.MonitorPath),
		"-no-reboot",
		"-boot", fmt.Sprintf("order=%s,reboot-timeout=5000", bootOrder),
		"-smbios", fmt.Sprintf("type=1,uuid=%s", config.NodeUUID),
		"-chardev", fmt.Sprintf("socket,path=%s/%s.sock,server=on,wait=off,id=qga0", config.StatePath, config.Hostname),
		"-device", "virtio-serial",
		"-device", "virtserialport,chardev=qga0,name=org.qemu.guest_agent.0",
		"-device", "i6300esb,id=watchdog0",
		"-watchdog-action",
		"pause",
	}

	if config.WithDebugShell {
		args = append(
			args,
			"-serial",
			fmt.Sprintf("unix:%s/%s.serial,server,nowait", config.StatePath, config.Hostname),
		)
	}

	var (
		scsiAttached, ahciAttached, nvmeAttached, megaraidAttached bool
		ahciBus                                                    int
	)

	for i, disk := range config.DiskPaths {
		driver := config.DiskDrivers[i]
		blockSize := config.DiskBlockSizes[i]

		switch driver {
		case "virtio":
			args = append(args,
				"-drive", fmt.Sprintf("id=virtio%d,format=raw,if=none,file=%s,cache=none", i, disk),
				"-device", fmt.Sprintf("virtio-blk-pci,drive=virtio%d,logical_block_size=%d,physical_block_size=%d", i, blockSize, blockSize),
			)
		case "ide":
			args = append(args, "-drive", fmt.Sprintf("format=raw,if=ide,file=%s,cache=none,", disk))
		case "ahci":
			if !ahciAttached {
				args = append(args, "-device", "ahci,id=ahci0")
				ahciAttached = true
			}

			args = append(args,
				"-drive", fmt.Sprintf("id=ide%d,format=raw,if=none,file=%s", i, disk),
				"-device", fmt.Sprintf("ide-hd,drive=ide%d,bus=ahci0.%d", i, ahciBus),
			)

			ahciBus++
		case "scsi":
			if !scsiAttached {
				args = append(args, "-device", "virtio-scsi-pci,id=scsi0")
				scsiAttached = true
			}

			args = append(args,
				"-drive", fmt.Sprintf("id=scsi%d,format=raw,if=none,file=%s,discard=unmap,aio=native,cache=none", i, disk),
				"-device", fmt.Sprintf("scsi-hd,drive=scsi%d,bus=scsi0.0,logical_block_size=%d,physical_block_size=%d", i, blockSize, blockSize),
			)
		case "nvme":
			if !nvmeAttached {
				// [TODO]: once Talos is fixed, use multipath NVME: https://qemu-project.gitlab.io/qemu/system/devices/nvme.html
				args = append(args,
					"-device", "nvme,id=nvme-ctrl-0,serial=deadbeef",
				)
				nvmeAttached = true
			}

			args = append(args,
				"-drive", fmt.Sprintf("id=nvme%d,format=raw,if=none,file=%s,discard=unmap,aio=native,cache=none", i, disk),
				"-device", fmt.Sprintf("nvme-ns,drive=nvme%d,logical_block_size=%d,physical_block_size=%d", i, blockSize, blockSize),
			)
		case "megaraid":
			if !megaraidAttached {
				args = append(args,
					"-device", "megasas-gen2,id=scsi1")

				megaraidAttached = true
			}

			args = append(args,
				"-drive", fmt.Sprintf("id=scsi%d,format=raw,if=none,file=%s,discard=unmap,aio=native,cache=none", i, disk),
				"-device", fmt.Sprintf("scsi-hd,drive=scsi%d,bus=scsi1.0,channel=0,scsi-id=%d,lun=0,logical_block_size=%d,physical_block_size=%d", i, i, blockSize, blockSize),
			)
		default:
			return fmt.Errorf("unsupported disk driver %q", driver)
		}
	}

	args = append(args, config.ArchitectureData.KVMArgs(config.EnableKVM, config.IOMMUEnabled)...)

	pflashArgs := make([]string, 2*len(config.PFlashImages))
	for i := range config.PFlashImages {
		pflashArgs[2*i] = "-drive"
		pflashArgs[2*i+1] = fmt.Sprintf("file=%s,format=raw,if=pflash", config.PFlashImages[i])
	}

	args = append(args, pflashArgs...)

	if config.ExtraISOPath != "" {
		args = append(args,
			"-drive",
			fmt.Sprintf("id=cdrom1,file=%s,media=cdrom", config.ExtraISOPath),
		)
	}

	// check if disk is empty/wiped
	diskBootable, err := checkPartitions(config)
	if err != nil {
		return err
	}

	if config.TPM2Config.NodeName != "" {
		tpm2SocketPath := filepath.Join(config.TPM2Config.StateDir, "swtpm.sock")

		cmd := exec.Command("swtpm", []string{
			"socket",
			"--tpmstate",
			fmt.Sprintf("dir=%s,mode=0644", config.TPM2Config.StateDir),
			"--ctrl",
			fmt.Sprintf("type=unixio,path=%s", tpm2SocketPath),
			"--tpm2",
			"--pid",
			fmt.Sprintf("file=%s", filepath.Join(config.TPM2Config.StateDir, "swtpm.pid")),
			"--log",
			fmt.Sprintf("file=%s,level=20", filepath.Join(config.TPM2Config.StateDir, "swtpm.log")),
		}...)

		log.Printf("starting swtpm: %s", cmd.String())

		if err := cmd.Start(); err != nil {
			return err
		}

		if err := waitForFileToExist(tpm2SocketPath, 5*time.Second); err != nil {
			return err
		}

		args = append(args,
			config.ArchitectureData.TPMDeviceArgs(tpm2SocketPath)...,
		)
	}

	// ref: https://wiki.qemu.org/Features/VT-d
	if config.IOMMUEnabled {
		args = append(args,
			"-device", "intel-iommu,intremap=on,device-iotlb=on",
			"-device", "ioh3420,id=pcie.1,chassis=1",
			"-device", "virtio-net-pci,bus=pcie.1,netdev=net1,disable-legacy=on,disable-modern=off,iommu_platform=on,ats=on",
			"-netdev", "tap,id=net1,vhostforce=on,script=no,downscript=no",
		)
	}

	if !diskBootable || !config.BootloaderEnabled {
		switch {
		case config.ISOPath != "":
			args = append(args,
				"-drive",
				fmt.Sprintf("id=cdrom0,file=%s,media=cdrom", config.ISOPath),
			)
		case config.USBPath != "":
			args = append(args,
				"-drive", fmt.Sprintf("if=none,id=stick,format=raw,read-only=on,file=%s", config.USBPath),
				"-device", "nec-usb-xhci,id=xhci",
				"-device", "usb-storage,bus=xhci.0,drive=stick,removable=on",
			)
		case config.UKIPath != "":
			args = append(args,
				"-kernel", config.UKIPath,
				"-append", config.KernelArgs,
			)
			config.sdStubExtraCmdline += config.sdStubExtraCmdlineConfig
		case config.KernelImagePath != "":
			args = append(args,
				"-kernel", config.KernelImagePath,
				"-initrd", config.InitrdPath,
				"-append", config.KernelArgs,
			)
			config.sdStubExtraCmdline += config.sdStubExtraCmdlineConfig
		}
	}

	args = append(args,
		"-smbios", fmt.Sprintf("type=11,value=io.systemd.stub.kernel-cmdline-extra=%s", config.sdStubExtraCmdline),
	)

	if config.BadRTC {
		args = append(args,
			"-rtc",
			"base=2011-11-11T11:11:00,clock=rt",
		)
	}

	fmt.Fprintf(os.Stderr, "starting %s with args:\n%s\n", config.ArchitectureData.QemuExecutable(), strings.Join(args, " "))
	cmd := exec.Command(
		config.ArchitectureData.QemuExecutable(),
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

	for {
		select {
		case sig := <-config.c:
			fmt.Fprintf(os.Stderr, "exiting VM as signal %s was received\n", sig)

			if err := cmd.Process.Kill(); err != nil {
				return fmt.Errorf("failed to kill process %w", err)
			}

			<-done

			return errors.New("process stopped")
		case err := <-done:
			if err != nil {
				return fmt.Errorf("process exited with error %s", err)
			}

			// graceful exit
			return nil
		case command := <-config.controller.CommandsCh():
			if command == VMCommandStop {
				fmt.Fprintf(os.Stderr, "exiting VM as stop command via API was received\n")

				if err := cmd.Process.Kill(); err != nil {
					return fmt.Errorf("failed to kill process %w", err)
				}

				<-done

				return nil
			}
		}
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
func Launch() error {
	var config LaunchConfig

	ctx := context.Background()

	if err := vm.ReadConfig(&config); err != nil {
		return err
	}

	config.c = vm.ConfigureSignals()
	config.controller = NewController()

	httpServer, err := vm.NewHTTPServer(config.GatewayAddrs[0], config.APIPort, []byte(config.Config), config.controller)
	if err != nil {
		return err
	}

	httpServer.Serve()
	defer httpServer.Shutdown(ctx) //nolint:errcheck

	// patch kernel args
	config.sdStubExtraCmdline = "console=ttyS0"

	if strings.Contains(config.KernelArgs, "{TALOS_CONFIG_URL}") {
		config.KernelArgs = strings.ReplaceAll(config.KernelArgs, "{TALOS_CONFIG_URL}", fmt.Sprintf("http://%s/config.yaml", httpServer.GetAddr()))
		config.sdStubExtraCmdlineConfig = fmt.Sprintf(" talos.config=http://%s/config.yaml", httpServer.GetAddr())
	}

	return withCNI(ctx, &config, func(config *LaunchConfig) error {
		for {
			for config.controller.PowerState() != PoweredOn {
				select {
				case <-config.controller.CommandsCh():
					// machine might have been powered on
				case sig := <-config.c:
					fmt.Fprintf(os.Stderr, "exiting stopped launcher as signal %s was received\n", sig)

					return errors.New("process stopped")
				}
			}

			if err := launchVM(config); err != nil {
				return err
			}
		}
	})
}

func waitForFileToExist(path string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if _, err := os.Stat(path); err == nil {
				return nil
			}
		}

		time.Sleep(100 * time.Millisecond)
	}
}
