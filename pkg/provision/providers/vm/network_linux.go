// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vm

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"net"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"text/template"

	"github.com/containernetworking/cni/libcni"
	"github.com/containernetworking/plugins/pkg/testutils"
	"github.com/coreos/go-iptables/iptables"
	"github.com/florianl/go-tc"
	"github.com/florianl/go-tc/core"
	"github.com/google/uuid"
	"github.com/jsimonetti/rtnetlink/v2"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-pointer"
	sideronet "github.com/siderolabs/net"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/pkg/provision"
)

// CreateNetwork builds bridge interface name by taking part of checksum of the network name
// so that interface name is defined by network name, and different networks have
// different bridge interfaces.
//
//nolint:gocyclo
func (p *Provisioner) CreateNetwork(ctx context.Context, state *State, network provision.NetworkRequest, options provision.Options) error {
	networkNameHash := sha256.Sum256([]byte(network.Name))
	state.BridgeName = fmt.Sprintf("%s%s", "talos", hex.EncodeToString(networkNameHash[:])[:8])

	// bring up the bridge interface for the first time to get gateway IP assigned
	t := template.Must(template.New("bridge").Parse(bridgeTemplate))

	var buf bytes.Buffer

	err := t.Execute(&buf, struct {
		NetworkName   string
		InterfaceName string
		MTU           string
	}{
		NetworkName:   network.Name,
		InterfaceName: state.BridgeName,
		MTU:           strconv.Itoa(network.MTU),
	})
	if err != nil {
		return fmt.Errorf("error templating bridge CNI config: %w", err)
	}

	bridgeConfig, err := libcni.NetworkPluginConfFromBytes(buf.Bytes())
	if err != nil {
		return fmt.Errorf("error parsing bridge CNI config: %w", err)
	}

	cniConfig := libcni.NewCNIConfigWithCacheDir(network.CNI.BinPath, network.CNI.CacheDir, nil)

	ns, err := testutils.NewNS()
	if err != nil {
		return err
	}

	defer func() {
		ns.Close()              //nolint:errcheck
		testutils.UnmountNS(ns) //nolint:errcheck
	}()

	// pick a fake address to use for provisioning an interface
	fakeIPs := make([]string, len(network.CIDRs))
	for j := range fakeIPs {
		var fakeIP netip.Addr

		fakeIP, err = sideronet.NthIPInNetwork(network.CIDRs[j], 2)
		if err != nil {
			return err
		}

		fakeIPs[j] = sideronet.FormatCIDR(fakeIP, network.CIDRs[j])
	}

	gatewayAddrs := xslices.Map(network.GatewayAddrs, netip.Addr.String)

	containerID := uuid.New().String()
	runtimeConf := libcni.RuntimeConf{
		ContainerID: containerID,
		NetNS:       ns.Path(),
		IfName:      "veth0",
		Args: [][2]string{
			{"IP", strings.Join(fakeIPs, ",")},
			{"GATEWAY", strings.Join(gatewayAddrs, ",")},
			{"IgnoreUnknown", "1"},
		},
	}

	_, err = cniConfig.AddNetwork(ctx, bridgeConfig, &runtimeConf)
	if err != nil {
		return fmt.Errorf("error provisioning bridge CNI network: %w", err)
	}

	err = cniConfig.DelNetwork(ctx, bridgeConfig, &runtimeConf)
	if err != nil {
		return fmt.Errorf("error deleting bridge CNI network: %w", err)
	}

	// prepare an actual network config to be used by the VMs
	t = template.Must(template.New("network").Parse(networkTemplate))

	buf.Reset()

	err = t.Execute(&buf, struct {
		NetworkName   string
		InterfaceName string
		MTU           string
	}{
		NetworkName:   network.Name,
		InterfaceName: state.BridgeName,
		MTU:           strconv.Itoa(network.MTU),
	})
	if err != nil {
		return fmt.Errorf("error templating VM CNI config: %w", err)
	}

	if state.VMCNIConfig, err = libcni.ConfListFromBytes(buf.Bytes()); err != nil {
		return fmt.Errorf("error parsing VM CNI config: %w", err)
	}

	// allow traffic on the bridge via `DOCKER-USER` chain
	// Docker enables br-netfilter which causes layer2 packets to be filtered with iptables, but we'd like to skip that
	// if Docker is not running, this will be no-op
	//
	// See https://serverfault.com/questions/963759/docker-breaks-libvirt-bridge-network for more details
	if err = p.allowBridgeTraffic(state.BridgeName); err != nil {
		return fmt.Errorf("error configuring DOCKER-USER chain: %w", err)
	}

	// configure bridge interface with network chaos if flag is set
	if network.NetworkChaos {
		if err = p.configureNetworkChaos(network, state, options); err != nil {
			return err
		}
	}

	return nil
}

func (p *Provisioner) allowBridgeTraffic(bridgeName string) error {
	ipt, err := iptables.New()
	if err != nil {
		return fmt.Errorf("error initializing iptables: %w", err)
	}

	chainExists, err := ipt.ChainExists("filter", "DOCKER-USER")
	if err != nil {
		return fmt.Errorf("error checking chain existence: %w", err)
	}

	if !chainExists {
		if err = ipt.NewChain("filter", "DOCKER-USER"); err != nil {
			return fmt.Errorf("error creating DOCKER-USER chain: %w", err)
		}
	}

	if err := ipt.InsertUnique("filter", "DOCKER-USER", 1, "-i", bridgeName, "-o", bridgeName, "-j", "ACCEPT"); err != nil {
		return fmt.Errorf("error inserting rule into DOCKER-USER chain: %w", err)
	}

	return nil
}

func (p *Provisioner) dropBridgeTrafficRule(bridgeName string) error {
	ipt, err := iptables.New()
	if err != nil {
		return fmt.Errorf("error initializing iptables: %w", err)
	}

	chainExists, err := ipt.ChainExists("filter", "DOCKER-USER")
	if err != nil {
		return fmt.Errorf("error checking chain existence: %w", err)
	}

	if !chainExists {
		return nil
	}

	if err := ipt.DeleteIfExists("filter", "DOCKER-USER", "-i", bridgeName, "-o", bridgeName, "-j", "ACCEPT"); err != nil {
		return fmt.Errorf("error deleting rule in DOCKER-USER chain: %w", err)
	}

	return nil
}

func getTicksInUsec() (float64, error) {
	data, err := os.ReadFile("/proc/net/psched")
	if err != nil {
		return 0, err
	}

	parts := strings.Split(strings.TrimSpace(string(data)), " ")
	if len(parts) < 3 {
		return 0, errors.New("unexpected format")
	}

	var vals [3]uint64

	for i := range vals {
		vals[i], err = strconv.ParseUint(parts[i], 16, 32)
		if err != nil {
			return 0, err
		}
	}

	// compatibility
	if vals[2] == 1000000000 {
		vals[0] = vals[1]
	}

	clockFactor := float64(vals[2]) / 1000000

	return float64(vals[0]) / float64(vals[1]) * clockFactor, nil
}

//nolint:gocyclo
func (p *Provisioner) configureNetworkChaos(network provision.NetworkRequest, state *State, options provision.Options) error {
	if (network.Bandwidth != 0) && (network.Latency != 0 || network.Jitter != 0 || network.PacketLoss != 0 || network.PacketReorder != 0 || network.PacketCorrupt != 0) {
		return errors.New("bandwidth and other chaos options cannot be used together")
	}

	tcnl, err := tc.Open(&tc.Config{})
	if err != nil {
		return fmt.Errorf("could not open tc: %v", err)
	}

	defer tcnl.Close() //nolint:errcheck

	link, err := net.InterfaceByName(state.BridgeName)
	if err != nil {
		return fmt.Errorf("could not get link: %v", err)
	}

	fmt.Fprintln(options.LogWriter, "network chaos enabled on interface:", state.BridgeName)

	if network.Bandwidth != 0 {
		fmt.Fprintf(options.LogWriter, "  bandwidth: %4d kbps\n", network.Bandwidth)

		ticksInUsec, err := getTicksInUsec()
		if err != nil {
			return fmt.Errorf("could not get ticks in usec: %w", err)
		}

		rate := network.Bandwidth * 1000 / 8 // rate in kbps
		latency := 0.2                       // 200ms
		burst := 50 * 1000                   // 50kb

		limit := uint32(float64(rate)*latency + float64(burst))
		buffer := uint32(1000000.0 * float64(burst) / float64(rate) * ticksInUsec)

		qdisc := tc.Object{
			Msg: tc.Msg{
				Family:  unix.AF_UNSPEC,
				Ifindex: uint32(link.Index),
				Handle:  core.BuildHandle(tc.HandleRoot, 0x0),
				Parent:  tc.HandleRoot,
				Info:    0,
			},
			Attribute: tc.Attribute{
				Kind: "tbf",
				Tbf: &tc.Tbf{
					Parms: &tc.TbfQopt{
						Limit: limit,
						Rate: tc.RateSpec{
							Rate:      uint32(rate),
							Linklayer: 1,
						},
						Buffer: buffer,
					},
				},
			},
		}

		if err := tcnl.Qdisc().Add(&qdisc); err != nil {
			return fmt.Errorf("could not add netem qdisc: %v", err)
		}
	} else {
		packetLoss := network.PacketLoss * 100
		packetReorder := network.PacketReorder * 100
		packetCorrupt := network.PacketCorrupt * 100

		fmt.Fprintf(options.LogWriter, "  jitter:            %4dms\n", network.Jitter.Milliseconds())
		fmt.Fprintf(options.LogWriter, "  latency:           %4dms\n", network.Latency.Milliseconds())
		fmt.Fprintf(options.LogWriter, "  packet loss:       %4v%%\n", packetLoss)
		fmt.Fprintf(options.LogWriter, "  packet reordering: %4v%%\n", packetReorder)
		fmt.Fprintf(options.LogWriter, "  packet corruption: %4v%%\n", packetCorrupt)

		qdisc := tc.Object{
			Msg: tc.Msg{
				Family:  unix.AF_UNSPEC,
				Ifindex: uint32(link.Index),
				Handle:  core.BuildHandle(tc.HandleRoot, 0x0),
				Parent:  tc.HandleRoot,
				Info:    0,
			},
			Attribute: tc.Attribute{
				Kind: "netem",
				Netem: &tc.Netem{
					Jitter64:  pointer.To(int64(network.Jitter)),
					Latency64: pointer.To(int64(network.Latency)),
					Qopt: tc.NetemQopt{
						Limit: 1000,
						Loss:  uint32(packetLoss / 100 * math.MaxUint32),
					},
					Corrupt: &tc.NetemCorrupt{
						Probability: uint32(packetCorrupt / 100 * math.MaxUint32),
					},
					Reorder: &tc.NetemReorder{
						Probability: uint32(packetReorder / 100 * math.MaxUint32),
					},
				},
			},
		}

		if err := tcnl.Qdisc().Add(&qdisc); err != nil {
			return fmt.Errorf("could not add netem qdisc: %v", err)
		}
	}

	return nil
}

// DestroyNetwork destroy bridge interface by name to clean up.
func (p *Provisioner) DestroyNetwork(state *State) error {
	iface, err := net.InterfaceByName(state.BridgeName)
	if err != nil {
		return fmt.Errorf("error looking up bridge interface %q: %w", state.BridgeName, err)
	}

	rtconn, err := rtnetlink.Dial(nil)
	if err != nil {
		return fmt.Errorf("error dialing rnetlink: %w", err)
	}

	if err = rtconn.Link.Delete(uint32(iface.Index)); err != nil {
		return fmt.Errorf("error deleting bridge interface: %w", err)
	}

	if err = p.dropBridgeTrafficRule(state.BridgeName); err != nil {
		return fmt.Errorf("error dropping bridge traffic rule: %w", err)
	}

	return nil
}

const bridgeTemplate = `
{
	"name": "{{ .NetworkName }}",
	"cniVersion": "0.4.0",
	"type": "bridge",
	"bridge": "{{ .InterfaceName }}",
	"ipMasq": true,
	"isGateway": true,
	"isDefaultGateway": true,
	"ipam": {
		  "type": "static"
	},
	"mtu": {{ .MTU }}
}
`

const networkTemplate = `
{
	"name": "{{ .NetworkName }}",
	"cniVersion": "0.4.0",
	"plugins": [
		{
			"type": "bridge",
			"bridge": "{{ .InterfaceName }}",
			"ipMasq": true,
			"isGateway": true,
			"isDefaultGateway": true,
			"ipam": {
				"type": "static"
			},
			"mtu": {{ .MTU }}
		},
		{
			"type": "firewall"
		},
		{
			"type": "tc-redirect-tap"
		}
	]
}
`
