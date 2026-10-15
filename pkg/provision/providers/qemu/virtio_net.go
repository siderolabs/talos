// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package qemu

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// virtioNetTunnelOffloadProperties are the virtio-net device properties (QEMU 10.2+) which offer
// GSO over UDP tunnel offload to the guest. They default to on for the current machine types.
//
// A Talos guest (Linux 6.18+) negotiates the offload, but the frames the host then delivers for a
// VXLAN-encapsulated TCP flow are rejected by the guest driver with
// `bad gso: type: 21, ... tunnel 1 tnl csum 1` and dropped, so any overlay traffic which gets
// TSO-aggregated between two VMs on the same bridge is lost. Cross-node pod traffic stalls (seen
// as NVMe-oF/TCP and Ceph I/O timeouts in the CSI tests). Turn the offload off wherever QEMU
// knows the property; older QEMU rejects unknown device properties, hence the probe.
var virtioNetTunnelOffloadProperties = []string{
	"guest_tunnel",
	"guest_tunnel_csum",
	"host_tunnel",
	"host_tunnel_csum",
}

// virtioNetDeviceProbeTimeout bounds the `qemu -device virtio-net-pci,help` run.
const virtioNetDeviceProbeTimeout = 30 * time.Second

// virtioNetDeviceProperties returns the names of the properties QEMU accepts for the virtio-net-pci device.
func virtioNetDeviceProperties(ctx context.Context, qemuExecutable string) (map[string]struct{}, error) {
	ctx, cancel := context.WithTimeout(ctx, virtioNetDeviceProbeTimeout)
	defer cancel()

	output, err := exec.CommandContext(ctx, qemuExecutable, "-device", "virtio-net-pci,help").Output()
	if err != nil {
		return nil, fmt.Errorf("error probing virtio-net-pci device properties: %w", err)
	}

	return parseDeviceProperties(output), nil
}

// parseDeviceProperties parses the output of `qemu -device <device>,help`.
//
// Every property is printed on its own indented line as `name=<type> - description (default: ...)`.
func parseDeviceProperties(help []byte) map[string]struct{} {
	properties := map[string]struct{}{}

	scanner := bufio.NewScanner(bytes.NewReader(help))

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, " ") {
			continue
		}

		name, _, found := strings.Cut(strings.TrimSpace(line), "=")
		if !found || name == "" {
			continue
		}

		properties[name] = struct{}{}
	}

	return properties
}

// virtioNetTunnelOffloadArgs returns the `-device virtio-net-pci,...` argument suffix which disables
// GSO over UDP tunnel offload, limited to the properties this QEMU supports.
//
// The result is empty for QEMU before 10.2, and starts with a comma otherwise.
func virtioNetTunnelOffloadArgs(properties map[string]struct{}) string {
	var args strings.Builder

	for _, property := range virtioNetTunnelOffloadProperties {
		if _, ok := properties[property]; !ok {
			continue
		}

		args.WriteString(",")
		args.WriteString(property)
		args.WriteString("=off")
	}

	return args.String()
}
