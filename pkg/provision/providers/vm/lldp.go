// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vm

import (
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"

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
// It is deliberately independent of the runtime decoder and status schema.
func LLDPTestFrame() []byte {
	frame := []byte{0x01, 0x80, 0xc2, 0, 0, 0x0e, 0x02, 0, 0, 0, 0, 2, 0x88, 0xcc}
	appendTLV := func(kind uint16, value []byte) {
		frame = binary.BigEndian.AppendUint16(frame, kind<<9|uint16(len(value)))
		frame = append(frame, value...)
	}

	appendTLV(1, []byte{4, 0x02, 0, 0, 0, 0, 1})            // MAC chassis ID
	appendTLV(2, append([]byte{5}, []byte("swp-test1")...)) // interface-name port ID
	appendTLV(3, []byte{0, 30})                             // TTL, refreshed every second
	appendTLV(4, []byte("Ethernet test port"))
	appendTLV(5, []byte("switch.example.com"))
	appendTLV(6, []byte("test switch"))
	appendTLV(8, []byte{5, 1, 192, 0, 2, 1, 2, 0, 0, 0, 10, 0})
	appendTLV(8, []byte{17, 2, 0x20, 1, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 2, 0, 0, 0, 11, 0})
	appendTLV(127, append([]byte{0, 0x80, 0xc2, 3, 0, 100, 4}, []byte("prod")...))
	appendTLV(127, append([]byte{0, 0x80, 0xc2, 3, 0, 200, 7}, []byte("storage")...))
	appendTLV(0, nil)

	return frame
}
