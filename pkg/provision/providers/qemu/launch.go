// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package qemu

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/siderolabs/go-blockdevice/v2/blkid"

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
	BootloaderEnabled bool
	TPMConfig         tpmConfig
	NodeUUID          uuid.UUID
	BadRTC            bool
	ArchitectureData  Arch
	WithDebugShell    bool
	IOMMUEnabled      bool

	// Talos config
	Config string

	// PXE
	TFTPServer       string
	BootFilename     string
	IPXEBootFileName string

	// API
	APIBindAddress *net.TCPAddr

	// sd-stub
	sdStubExtraCmdline       string
	sdStubExtraCmdlineConfig string

	// platform specific Network configuration
	Network networkConfig

	VMMac string

	// signals
	c chan os.Signal

	// controller
	controller *Controller
}

type networkConfigBase struct {
	BridgeName   string
	IPs          []netip.Addr
	CIDRs        []netip.Prefix
	GatewayAddrs []netip.Addr
	Hostname     string
	MTU          int
	Nameservers  []netip.Addr
}

type tpmConfig struct {
	NodeName string
	StateDir string

	TPM2 bool
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
		"-netdev", getNetdevParams(config.Network, "net0"),
		"-device", fmt.Sprintf("virtio-net-pci,netdev=net0,mac=%s", config.VMMac),
		// TODO: uncomment the following line to get another eth interface not connected to anything
		// "-nic", "tap,model=virtio-net-pci",
		"-device", "virtio-rng-pci",
		"-device", "virtio-balloon,deflate-on-oom=on",
		"-monitor", fmt.Sprintf("unix:%s,server,nowait", config.MonitorPath),
		"-no-reboot",
		"-boot", fmt.Sprintf("order=%s,reboot-timeout=5000", bootOrder),
		"-smbios", fmt.Sprintf("type=1,uuid=%s", config.NodeUUID),
		"-chardev", fmt.Sprintf("socket,path=%s/%s.sock,server=on,wait=off,id=qga0", config.StatePath, config.Network.Hostname),
		"-device", "virtio-serial",
		"-device", "virtserialport,chardev=qga0,name=org.qemu.guest_agent.0",
		"-device", "i6300esb,id=watchdog0",
		"-watchdog-action", "pause",
	}

	if config.WithDebugShell {
		args = append(
			args,
			"-serial",
			fmt.Sprintf("unix:%s/%s.serial,server,nowait", config.StatePath, config.Network.Hostname),
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

	args = append(args, config.ArchitectureData.getMachineArgs(config.IOMMUEnabled)...)

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

	if config.TPMConfig.NodeName != "" {
		tpm2SocketPath := filepath.Join(config.TPMConfig.StateDir, "swtpm.sock")

		swtpmArgs := []string{
			"socket",
			"--tpmstate",
			fmt.Sprintf("dir=%s,mode=0644", config.TPMConfig.StateDir),
			"--ctrl",
			fmt.Sprintf("type=unixio,path=%s", tpm2SocketPath),
			// "--tpm2",
			"--pid",
			fmt.Sprintf("file=%s", filepath.Join(config.TPMConfig.StateDir, "swtpm.pid")),
			"--log",
			fmt.Sprintf("file=%s,level=20", filepath.Join(config.TPMConfig.StateDir, "swtpm.log")),
		}

		if config.TPMConfig.TPM2 {
			swtpmArgs = append(swtpmArgs, "--tpm2")
		}

		cmd := exec.Command("swtpm", swtpmArgs...)

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

	if err := startQemuCmd(config, cmd); err != nil {
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
//
//nolint:gocyclo
func Launch() error {
	var config LaunchConfig

	ctx := context.Background()

	if err := vm.ReadConfig(&config); err != nil {
		return err
	}

	config.c = vm.ConfigureSignals()
	config.controller = NewController()

	apiBindAddrs, err := netip.ParseAddr(config.APIBindAddress.IP.String())
	if err != nil {
		return err
	}

	httpServer, err := vm.NewHTTPServer(apiBindAddrs, config.APIBindAddress.Port, []byte(config.Config), config.controller)
	if err != nil {
		return err
	}

	httpServer.Serve()
	defer httpServer.Shutdown(ctx) //nolint:errcheck

	if err := patchKernelArgs(&config, httpServer.GetAddr()); err != nil {
		return err
	}

	return withNetworkContext(ctx, &config, func(config *LaunchConfig) error {
		err = dumpIpam(*config)
		if err != nil {
			return err
		}

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

func patchKernelArgs(config *LaunchConfig, httpServerAddr net.Addr) error {
	configServerAddr, err := getConfigServerAddr(httpServerAddr, *config)
	if err != nil {
		return err
	}

	config.sdStubExtraCmdline = "console=ttyS0"

	if strings.Contains(config.KernelArgs, "{TALOS_CONFIG_URL}") {
		config.KernelArgs = strings.ReplaceAll(config.KernelArgs, "{TALOS_CONFIG_URL}", fmt.Sprintf("http://%s/config.yaml", configServerAddr))
		config.sdStubExtraCmdlineConfig = fmt.Sprintf(" talos.config=http://%s/config.yaml", httpServerAddr)
	}

	return nil
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

func dumpIpam(config LaunchConfig) error {
	for j := range config.Network.CIDRs {
		nameservers := make([]netip.Addr, 0, len(config.Network.Nameservers))

		// filter nameservers by IPv4/IPv6 matching IPs
		for i := range config.Network.Nameservers {
			if config.Network.IPs[j].Is6() {
				if config.Network.Nameservers[i].Is6() {
					nameservers = append(nameservers, config.Network.Nameservers[i])
				}
			} else {
				if config.Network.Nameservers[i].Is4() {
					nameservers = append(nameservers, config.Network.Nameservers[i])
				}
			}
		}

		// dump node IP/mac/hostname for dhcp
		if err := vm.DumpIPAMRecord(config.StatePath, vm.IPAMRecord{
			IP:               config.Network.IPs[j],
			Netmask:          byte(config.Network.CIDRs[j].Bits()),
			MAC:              config.VMMac,
			Hostname:         config.Network.Hostname,
			Gateway:          config.Network.GatewayAddrs[j],
			MTU:              config.Network.MTU,
			Nameservers:      nameservers,
			TFTPServer:       config.TFTPServer,
			IPXEBootFilename: config.IPXEBootFileName,
		}); err != nil {
			return err
		}
	}

	return nil
}

func checkPartitions(config *LaunchConfig) (bool, error) {
	info, err := blkid.ProbePath(config.DiskPaths[0], blkid.WithSectorSize(config.DiskBlockSizes[0]))
	if err != nil {
		return false, fmt.Errorf("error probing disk: %w", err)
	}

	return info.Name == "gpt" && len(info.Parts) > 0, nil
}
