// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vm

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"strconv"
	"strings"
	"text/template"

	"github.com/containernetworking/cni/libcni"
	"github.com/containernetworking/plugins/pkg/testutils"
	"github.com/google/uuid"
	"github.com/jsimonetti/rtnetlink"
	talosnet "github.com/talos-systems/net"

	"github.com/talos-systems/talos/pkg/provision"
)

// CreateNetwork builds bridge interface name by taking part of checksum of the network name
// so that interface name is defined by network name, and different networks have
// different bridge interfaces.
//
//nolint:gocyclo
func (p *Provisioner) CreateNetwork(ctx context.Context, state *State, network provision.NetworkRequest) error {
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

	bridgeConfig, err := libcni.ConfFromBytes(buf.Bytes())
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
		var fakeIP net.IP

		fakeIP, err = talosnet.NthIPInNetwork(&network.CIDRs[j], 2)
		if err != nil {
			return err
		}

		fakeIPs[j] = talosnet.FormatCIDR(fakeIP, network.CIDRs[j])
	}

	gatewayAddrs := make([]string, len(network.GatewayAddrs))
	for j := range gatewayAddrs {
		gatewayAddrs[j] = network.GatewayAddrs[j].String()
	}

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
