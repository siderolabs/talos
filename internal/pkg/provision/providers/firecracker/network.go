// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package firecracker

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/containernetworking/cni/libcni"
	"github.com/containernetworking/plugins/pkg/testutils"
	"github.com/google/uuid"

	"github.com/talos-systems/talos/internal/pkg/provision"
	talosnet "github.com/talos-systems/talos/pkg/net"
)

func (p *provisioner) createNetwork(ctx context.Context, state *state, network provision.NetworkRequest) error {
	// bring up the bridge interface for the first time to get gateway IP assigned
	t := template.Must(template.New("bridge").Parse(bridgeTemplate))

	var buf bytes.Buffer

	err := t.Execute(&buf, struct {
		NetworkName   string
		InterfaceName string
	}{
		NetworkName:   network.Name,
		InterfaceName: state.bridgeInterfaceName,
	})
	if err != nil {
		return err
	}

	bridgeConfig, err := libcni.ConfFromBytes(buf.Bytes())
	if err != nil {
		return err
	}

	cniConfig := libcni.NewCNIConfigWithCacheDir(network.CNI.BinPath, network.CNI.CacheDir, nil)

	ns, err := testutils.NewNS()
	if err != nil {
		return err
	}

	defer testutils.UnmountNS(ns) //nolint: errcheck

	// pick a fake address to use for provisioning an interface
	fakeIP, err := talosnet.NthIPInNetwork(&network.CIDR, 2)
	if err != nil {
		return err
	}

	ones, bits := network.CIDR.IP.DefaultMask().Size()
	containerID := uuid.New().String()
	runtimeConf := libcni.RuntimeConf{
		ContainerID: containerID,
		NetNS:       ns.Path(),
		IfName:      "veth0",
		Args: [][2]string{
			{"IP", fmt.Sprintf("%s/%d", fakeIP, bits-ones)},
			{"GATEWAY", network.GatewayAddr.String()},
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

	f, err := os.Create(filepath.Join(network.CNI.ConfDir, fmt.Sprintf("%s.conflist", network.Name)))
	if err != nil {
		return err
	}

	defer f.Close() //nolint: errcheck

	err = t.Execute(f, struct {
		NetworkName   string
		InterfaceName string
	}{
		NetworkName:   network.Name,
		InterfaceName: state.bridgeInterfaceName,
	})
	if err != nil {
		return err
	}

	return f.Close()
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
	}
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
		}
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
