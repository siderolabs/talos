// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vm

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"time"

	"github.com/siderolabs/go-retry/retry"
	"go4.org/netipx"

	"github.com/siderolabs/talos/pkg/provision"
)

// vmnetKeepalivePidFile is the pidfile of the keepalive VM that holds the vmnet bridge open.
const vmnetKeepalivePidFile = "vmnet-keepalive.pid"

// vmnetKeepaliveQMPSocket is the QMP socket of the keepalive VM, used once at start to idle it.
const vmnetKeepaliveQMPSocket = "vmnet-keepalive.qmp"

// CreateNetwork on darwin brings up the vmnet bridge and holds it open for the whole network lifetime.
//
// On darwin the vmnet bridge exists only while at least one vmnet endpoint is open, and qemu opens
// one per machine. Without a keepalive the bridge would not exist until the first machine starts, so
// the dhcpd (which waits for the bridge) would deadlock on a cluster whose first node is created
// after it, such as an all-PXE cluster. The bridge would also vanish whenever every machine is
// powered off, and the gateway IP and any host service bound to it would go with it. The keepalive VM
// holds one endpoint open for the network's lifetime, the same way the Linux provider owns its bridge
// directly.
func (p *Provisioner) CreateNetwork(ctx context.Context, state *provision.State, network provision.NetworkRequest, options provision.Options) error {
	if len(network.GatewayAddrs) == 0 {
		return errors.New("network has no gateway address")
	}

	if len(network.CIDRs) == 0 {
		return errors.New("network has no CIDRs")
	}

	gateway, cidr := network.GatewayAddrs[0], network.CIDRs[0]

	if !gateway.Is4() || !cidr.Addr().Is4() {
		return fmt.Errorf("darwin vmnet requires an IPv4 network, got gateway %s in %s", gateway, cidr)
	}

	pidPath := state.GetRelativePath(vmnetKeepalivePidFile)

	cmd, err := startVmnetKeepalive(state, gateway, cidr)
	if err != nil {
		return fmt.Errorf("error starting vmnet keepalive: %w", err)
	}

	if err = os.WriteFile(pidPath, []byte(strconv.Itoa(cmd.Process.Pid)), os.ModePerm); err != nil {
		_ = cmd.Process.Kill() //nolint:errcheck

		return fmt.Errorf("error recording vmnet keepalive pid: %w", err)
	}

	// The keepalive brings the bridge up with the gateway address assigned to it. Observe the interface
	// carrying that address (rather than predicting its name) and record it, so the dhcpd that binds to
	// it finds a ready interface.
	bridgeName, err := waitForInterfaceWithAddr(ctx, gateway)
	if err != nil {
		return errors.Join(fmt.Errorf("error waiting for the vmnet bridge to come up: %w", err), StopProcessByPidfile(pidPath))
	}

	state.BridgeName = bridgeName

	return nil
}

// DestroyNetwork stops the vmnet keepalive, releasing the bridge. All machines are already destroyed
// by the time this is called, so this finally tears the bridge down.
func (p *Provisioner) DestroyNetwork(state *provision.State) error {
	return StopProcessByPidfile(state.GetRelativePath(vmnetKeepalivePidFile))
}

// startVmnetKeepalive launches a minimal, paused qemu VM whose only job is to open one vmnet endpoint
// on the network's subnet, which creates the bridge and holds it open.
//
// The VM starts paused (-S) and is then resumed and stopped once over QMP. qemu's vmnet backend arms its
// "packets available" callback when the interface is created and disarms it only when the VM state changes
// to stopped. A VM that never ran would keep the callback armed, with nothing reading the packets, and
// would busy-loop at over 100% CPU. The resume/stop round trip disarms it while the interface (and so the
// bridge) stays open.
func startVmnetKeepalive(state *provision.State, gateway netip.Addr, cidr netip.Prefix) (*exec.Cmd, error) {
	// The end address is the last usable host of the subnet, i.e., the broadcast address minus one.
	netdev := fmt.Sprintf(
		"vmnet-shared,id=n0,start-address=%s,end-address=%s,subnet-mask=%s",
		gateway, netipx.RangeOfPrefix(cidr).To().Prev(), net.IP(net.CIDRMask(cidr.Bits(), 32)),
	)

	qemuBinary, machineType, err := hostQemu()
	if err != nil {
		return nil, err
	}

	qmpPath := state.GetRelativePath(vmnetKeepaliveQMPSocket)

	logFile, err := os.OpenFile(state.GetRelativePath("vmnet-keepalive.log"), os.O_APPEND|os.O_CREATE|os.O_RDWR, 0o666)
	if err != nil {
		return nil, err
	}

	defer logFile.Close() //nolint:errcheck

	cmd := exec.Command(qemuBinary, //nolint:noctx // runs in the background for the network lifetime
		"-machine", machineType,
		"-m", "32",
		"-nographic",
		"-S",
		"-netdev", netdev,
		"-device", "virtio-net-pci,netdev=n0",
		"-monitor", "none",
		"-serial", "none",
		"-qmp", "unix:"+qmpPath+",server,nowait",
	)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	setDetachedProcess(cmd)

	if err = cmd.Start(); err != nil {
		return nil, err
	}

	if err = pauseVmnetKeepalive(qmpPath); err != nil {
		_ = cmd.Process.Kill() //nolint:errcheck

		return nil, fmt.Errorf("error idling the keepalive VM: %w", err)
	}

	return cmd, nil
}

// pauseVmnetKeepalive resumes and stops the keepalive VM over QMP, see [startVmnetKeepalive].
func pauseVmnetKeepalive(qmpPath string) error {
	var conn net.Conn

	err := retry.Constant(10*time.Second, retry.WithUnits(50*time.Millisecond)).Retry(func() error {
		var dialErr error

		conn, dialErr = net.Dial("unix", qmpPath)
		if dialErr != nil {
			return retry.ExpectedError(dialErr)
		}

		return nil
	})
	if err != nil {
		return err
	}

	defer conn.Close() //nolint:errcheck

	if err = conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return err
	}

	reader := bufio.NewReader(conn)

	// QMP greets first, and commands are accepted only after the capabilities are negotiated.
	if _, err = reader.ReadBytes('\n'); err != nil {
		return fmt.Errorf("error reading QMP greeting: %w", err)
	}

	for _, command := range []string{"qmp_capabilities", "cont", "stop"} {
		if _, err = fmt.Fprintf(conn, "{\"execute\":%q}\n", command); err != nil {
			return err
		}

		// skip asynchronous events until the reply to the command arrives
		for {
			line, readErr := reader.ReadBytes('\n')
			if readErr != nil {
				return fmt.Errorf("error reading QMP reply to %q: %w", command, readErr)
			}

			var reply struct {
				Return json.RawMessage `json:"return"`
				Error  *struct {
					Desc string `json:"desc"`
				} `json:"error"`
			}

			if err = json.Unmarshal(line, &reply); err != nil {
				return fmt.Errorf("error parsing QMP reply %q: %w", line, err)
			}

			if reply.Error != nil {
				return fmt.Errorf("QMP command %q failed: %s", command, reply.Error.Desc)
			}

			if reply.Return != nil {
				break
			}
		}
	}

	return nil
}

// waitForInterfaceWithAddr waits for the interface that carries the given address to appear and
// returns its name.
//
// The match is by address only, so a stale bridge left behind by a previous network with the same
// gateway address would be matched too. That case is not handled here, since networks with
// overlapping subnets conflict at the vmnet level anyway.
func waitForInterfaceWithAddr(ctx context.Context, addr netip.Addr) (string, error) {
	var name string

	err := retry.Constant(time.Minute, retry.WithUnits(100*time.Millisecond)).RetryWithContext(ctx, func(context.Context) error {
		ifaces, err := net.Interfaces()
		if err != nil {
			return err
		}

		for _, iface := range ifaces {
			addrs, addrsErr := iface.Addrs()
			if addrsErr != nil {
				continue
			}

			for _, a := range addrs {
				ipNet, ok := a.(*net.IPNet)
				if !ok {
					continue
				}

				if ip, ok := netip.AddrFromSlice(ipNet.IP); ok && ip.Unmap() == addr {
					name = iface.Name

					return nil
				}
			}
		}

		return retry.ExpectedError(fmt.Errorf("no interface has address %s yet", addr))
	})
	if err != nil {
		return "", err
	}

	return name, nil
}

// hostQemu returns the qemu binary and machine type to run the keepalive VM with.
//
// This choice exists because qemu ships one binary per target architecture, and the aarch64 one
// refuses to start without an explicit machine type. Real machines pick the binary matching the
// Talos architecture they boot, possibly emulating a foreign one, but the keepalive never executes
// a single instruction: it starts paused, with no disk and no OS, and its only job is to hold a
// vmnet endpoint open. Its architecture is therefore irrelevant, so it uses the host-native qemu,
// which is the one guaranteed to be installed.
func hostQemu() (binary, machineType string, err error) {
	var qemuArch string

	switch runtime.GOARCH {
	case "arm64":
		qemuArch, machineType = "aarch64", "virt"
	case "amd64":
		qemuArch, machineType = "x86_64", "q35"
	default:
		return "", "", fmt.Errorf("unsupported host architecture %q", runtime.GOARCH)
	}

	binary, err = exec.LookPath("qemu-system-" + qemuArch)
	if err != nil {
		return "", "", fmt.Errorf("qemu-system-%s not found: %w", qemuArch, err)
	}

	return binary, machineType, nil
}
