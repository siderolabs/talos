// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vm

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"time"

	"github.com/mdlayher/ethernet"
	"github.com/siderolabs/go-lldp/pkg/lldp"

	"github.com/siderolabs/talos/pkg/provision"
)

const lldpPID = "lldp.pid"

// CreateLLDP starts a continuous receive-test fixture on the QEMU host.
func (p *Provisioner) CreateLLDP(state *provision.State, clusterReq provision.ClusterRequest) error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("--with-lldp requires a Linux QEMU host; use the remote provisioner")
	}

	if state.BridgeName == "" || clusterReq.Network.CLOSNoNet0 {
		return fmt.Errorf("--with-lldp requires the QEMU management bridge")
	}

	logFile, err := os.OpenFile(state.GetRelativePath("lldp.log"), os.O_APPEND|os.O_CREATE|os.O_RDWR, 0o666)
	if err != nil {
		return err
	}

	defer logFile.Close() //nolint:errcheck

	cmd := exec.Command(clusterReq.SelfExecutable, "lldp-launch", "--bridge", state.BridgeName) //nolint:noctx // runs in background
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	setDetachedProcess(cmd)

	if err = cmd.Start(); err != nil {
		return err
	}

	if err = os.WriteFile(state.GetRelativePath(lldpPID), []byte(strconv.Itoa(cmd.Process.Pid)), 0o600); err != nil {
		cmd.Process.Kill() //nolint:errcheck // best-effort cleanup; preserve the PID write error
		cmd.Wait()         //nolint:errcheck // reap the child killed above

		return fmt.Errorf("error writing LLDP PID file: %w", err)
	}

	return nil
}

// DestroyLLDP stops the host-side fixture, if enabled.
func (p *Provisioner) DestroyLLDP(state *provision.State) error {
	return StopProcessByPidfile(state.GetRelativePath(lldpPID))
}

// LLDPTestFrame is the known, continuously refreshed advertisement for receive tests.
// The port ID identifies the outgoing host bridge port.
func LLDPTestFrame(portID string) ([]byte, error) {
	frame := lldp.Frame{
		ChassisID: &lldp.ChassisID{Subtype: lldp.ChassisIDSubtypeMACAddress, ID: []byte{0x02, 0, 0, 0, 0, 1}},
		PortID:    &lldp.PortID{Subtype: lldp.PortIDSubtypeInterfaceName, ID: []byte(portID)},
		TTL:       30 * time.Second,
	}

	for _, tlv := range []lldp.TLV{
		{Type: lldp.TLVTypePortDescription, Value: []byte("Ethernet test port")},
		{Type: lldp.TLVTypeSystemName, Value: []byte("switch.example.com")},
		{Type: lldp.TLVTypeSystemDescription, Value: []byte("test switch")},
		{Type: lldp.TLVTypeManagementAddress, Value: []byte{5, 1, 192, 0, 2, 1, 2, 0, 0, 0, 10, 0}},
		{Type: lldp.TLVTypeManagementAddress, Value: []byte{17, 2, 0x20, 1, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 2, 0, 0, 0, 11, 0}},
		{Type: lldp.TLVTypeOrganizationSpecific, Value: append([]byte{0, 0x80, 0xc2, 3, 0, 100, 4}, []byte("prod")...)},
		{Type: lldp.TLVTypeOrganizationSpecific, Value: append([]byte{0, 0x80, 0xc2, 3, 0, 200, 7}, []byte("storage")...)},
	} {
		tlv.Length = uint16(len(tlv.Value))
		frame.Optional = append(frame.Optional, &tlv)
	}

	payload, err := frame.MarshalBinary()
	if err != nil {
		return nil, err
	}

	envelope := ethernet.Frame{
		Destination: net.HardwareAddr{0x01, 0x80, 0xc2, 0, 0, 0x0e},
		Source:      net.HardwareAddr{0x02, 0, 0, 0, 0, 2},
		EtherType:   lldp.EtherType,
		Payload:     payload,
	}

	return envelope.MarshalBinary()
}
