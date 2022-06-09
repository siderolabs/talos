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
	"strings"
	"syscall"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv4/server4"
	"github.com/insomniacslk/dhcp/dhcpv6"
	"github.com/insomniacslk/dhcp/dhcpv6/server6"
	"github.com/insomniacslk/dhcp/iana"
	"golang.org/x/sync/errgroup"

	"github.com/talos-systems/talos/pkg/machinery/generic/slices"
	"github.com/talos-systems/talos/pkg/provision"
)

//nolint:gocyclo
func handlerDHCP4(serverIP net.IP, statePath string) server4.Handler {
	return func(conn net.PacketConn, peer net.Addr, m *dhcpv4.DHCPv4) {
		log.Printf("DHCPv6: got %s", m.Summary())

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

		row, ok := db[m.ClientHWAddr.String()]
		if !ok {
			log.Printf("no match for MAC: %s", m.ClientHWAddr.String())

			return
		}

		match, ok := row[4]
		if !ok {
			log.Printf("no match for MAC on IPv4: %s", m.ClientHWAddr.String())

			return
		}

		resp, err := dhcpv4.NewReplyFromRequest(m,
			dhcpv4.WithNetmask(match.Netmask),
			dhcpv4.WithYourIP(match.IP),
			dhcpv4.WithOption(dhcpv4.OptHostName(match.Hostname)),
			dhcpv4.WithOption(dhcpv4.OptDNS(match.Nameservers...)),
			dhcpv4.WithOption(dhcpv4.OptRouter(match.Gateway)),
			dhcpv4.WithOption(dhcpv4.OptIPAddressLeaseTime(5*time.Minute)),
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

		switch mt := m.MessageType(); mt { //nolint:exhaustive
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

//nolint:gocyclo
func handlerDHCP6(serverHwAddr net.HardwareAddr, statePath string) server6.Handler {
	return func(conn net.PacketConn, peer net.Addr, m dhcpv6.DHCPv6) {
		log.Printf("DHCPv6: got %s", m.Summary())

		db, err := LoadIPAMRecords(statePath)
		if err != nil {
			log.Printf("failed loading the IPAM db: %s", err)

			return
		}

		if db == nil {
			return
		}

		msg, err := m.GetInnerMessage()
		if err != nil {
			log.Printf("failed loading inner message: %s", err)

			return
		}

		hwaddr, err := dhcpv6.ExtractMAC(m)
		if err != nil {
			log.Printf("error extracting hwaddr: %s", err)

			return
		}

		row, ok := db[hwaddr.String()]
		if !ok {
			log.Printf("no match for MAC: %s", hwaddr)

			return
		}

		match, ok := row[6]
		if !ok {
			log.Printf("no match for MAC on IPv6: %s", hwaddr)

			return
		}

		modifiers := []dhcpv6.Modifier{
			dhcpv6.WithDNS(match.Nameservers...),
			dhcpv6.WithFQDN(0, match.Hostname),
			dhcpv6.WithIANA(dhcpv6.OptIAAddress{
				IPv6Addr:          match.IP,
				PreferredLifetime: 5 * time.Minute,
				ValidLifetime:     5 * time.Minute,
			}),
			dhcpv6.WithServerID(dhcpv6.Duid{
				Type:          dhcpv6.DUID_LLT,
				HwType:        iana.HWTypeEthernet,
				Time:          dhcpv6.GetTime(),
				LinkLayerAddr: serverHwAddr,
			}),
		}

		var resp *dhcpv6.Message

		switch msg.MessageType { //nolint:exhaustive
		case dhcpv6.MessageTypeSolicit:
			resp, err = dhcpv6.NewAdvertiseFromSolicit(msg, modifiers...)
		case dhcpv6.MessageTypeRequest:
			resp, err = dhcpv6.NewReplyFromMessage(msg, modifiers...)
		default:
			log.Printf("unsupported message type %s", msg.Summary())
		}

		if err != nil {
			log.Printf("failure building response: %s", err)

			return
		}

		_, err = conn.WriteTo(resp.ToBytes(), peer)
		if err != nil {
			log.Printf("failure sending response: %s", err)
		}
	}
}

// DHCPd entrypoint.
func DHCPd(ifName string, ips []net.IP, statePath string) error {
	iface, err := net.InterfaceByName(ifName)
	if err != nil {
		return fmt.Errorf("error looking up interface: %w", err)
	}

	var eg errgroup.Group

	for _, ip := range ips {
		ip := ip

		eg.Go(func() error {
			if ip.To4() == nil {
				server, err := server6.NewServer(
					ifName,
					nil,
					handlerDHCP6(iface.HardwareAddr, statePath),
					server6.WithDebugLogger(),
				)
				if err != nil {
					log.Printf("error on dhcp6 startup: %s", err)

					return err
				}

				return server.Serve()
			}

			server, err := server4.NewServer(
				ifName,
				nil,
				handlerDHCP4(ip, statePath),
				server4.WithSummaryLogger(),
			)
			if err != nil {
				log.Printf("error on dhcp4 startup: %s", err)

				return err
			}

			return server.Serve()
		})
	}

	return eg.Wait()
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

	defer logFile.Close() //nolint:errcheck

	statePath, err := state.StatePath()
	if err != nil {
		return err
	}

	gatewayAddrs := slices.Map(clusterReq.Network.GatewayAddrs, net.IP.String)

	args := []string{
		"dhcpd-launch",
		"--state-path", statePath,
		"--addr", strings.Join(gatewayAddrs, ","),
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
