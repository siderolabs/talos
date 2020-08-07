// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vm

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv4/server4"

	"github.com/talos-systems/talos/pkg/provision"
)

//nolint: gocyclo
func handler(serverIP net.IP, statePath string) server4.Handler {
	return func(conn net.PacketConn, peer net.Addr, m *dhcpv4.DHCPv4) {
		if m.OpCode != dhcpv4.OpcodeBootRequest {
			return
		}

		db, err := LoadIPAMRecords(statePath)
		if err != nil {
			log.Printf("failed loading the IPAM db: %s", err)
			return
		}

		if db == nil {
			return
		}

		match, ok := db[m.ClientHWAddr.String()]
		if !ok {
			log.Printf("no match for MAC: %s", m.ClientHWAddr.String())
			return
		}

		resp, err := dhcpv4.NewReplyFromRequest(m,
			dhcpv4.WithNetmask(match.Netmask),
			dhcpv4.WithYourIP(match.IP),
			dhcpv4.WithOption(dhcpv4.OptHostName(match.Hostname)),
			dhcpv4.WithOption(dhcpv4.OptDNS(match.Nameservers...)),
			dhcpv4.WithOption(dhcpv4.OptRouter(match.Gateway)),
			dhcpv4.WithOption(dhcpv4.OptIPAddressLeaseTime(time.Hour)),
			dhcpv4.WithOption(dhcpv4.OptServerIdentifier(serverIP)),
		)
		if err != nil {
			log.Printf("failure building response: %s", err)
			return
		}

		if m.IsOptionRequested(dhcpv4.OptionBootfileName) {
			log.Printf("received PXE boot request from %s", m.ClientHWAddr)

			if match.TFTPServer != "" {
				log.Printf("sending PXE response to %s: %s/%s", m.ClientHWAddr, match.TFTPServer, match.IPXEBootFilename)

				resp.ServerIPAddr = net.ParseIP(match.TFTPServer)
				resp.UpdateOption(dhcpv4.OptTFTPServerName(match.TFTPServer))
				resp.UpdateOption(dhcpv4.OptBootFileName(match.IPXEBootFilename))
			}
		}

		resp.UpdateOption(dhcpv4.OptGeneric(dhcpv4.OptionInterfaceMTU, dhcpv4.Uint16(match.MTU).ToBytes()))

		switch mt := m.MessageType(); mt { //nolint: exhaustive
		case dhcpv4.MessageTypeDiscover:
			resp.UpdateOption(dhcpv4.OptMessageType(dhcpv4.MessageTypeOffer))
		case dhcpv4.MessageTypeRequest:
			resp.UpdateOption(dhcpv4.OptMessageType(dhcpv4.MessageTypeAck))
		default:
			log.Printf("unhandled message type: %v", mt)
			return
		}

		_, err = conn.WriteTo(resp.ToBytes(), peer)
		if err != nil {
			log.Printf("failure sending response: %s", err)
		}
	}
}

// DHCPd entrypoint.
func DHCPd(ifName string, ip net.IP, statePath string) error {
	server, err := server4.NewServer(ifName, nil, handler(ip, statePath), server4.WithDebugLogger())
	if err != nil {
		return err
	}

	return server.Serve()
}

const (
	dhcpPid = "dhcpd.pid"
	dhcpLog = "dhcpd.log"
)

// CreateDHCPd creates DHCPd.
func (p *Provisioner) CreateDHCPd(state *State, clusterReq provision.ClusterRequest) error {
	pidPath := state.GetRelativePath(dhcpPid)

	logFile, err := os.OpenFile(state.GetRelativePath(dhcpLog), os.O_APPEND|os.O_CREATE|os.O_RDWR, 0o666)
	if err != nil {
		return err
	}

	defer logFile.Close() //nolint: errcheck

	statePath, err := state.StatePath()
	if err != nil {
		return err
	}

	args := []string{
		"dhcpd-launch",
		"--state-path", statePath,
		"--addr", clusterReq.Network.GatewayAddr.String(),
		"--interface", state.BridgeName,
	}

	cmd := exec.Command(clusterReq.SelfExecutable, args...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true, // daemonize
	}

	if err = cmd.Start(); err != nil {
		return err
	}

	if err = ioutil.WriteFile(pidPath, []byte(strconv.Itoa(cmd.Process.Pid)), os.ModePerm); err != nil {
		return fmt.Errorf("error writing dhcp PID file: %w", err)
	}

	return nil
}

// DestroyDHCPd destoys load balancer.
func (p *Provisioner) DestroyDHCPd(state *State) error {
	pidPath := state.GetRelativePath(dhcpPid)

	return stopProcessByPidfile(pidPath)
}
