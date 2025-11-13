// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package proxmox

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/siderolabs/go-cmd/pkg/cmd"
	"github.com/siderolabs/talos/pkg/provision"
)

// createCloudInitISO creates a cloud-init ISO with meta-data, user-data, and network-config.
func (p *provisioner) createCloudInitISO(state *provision.State, nodeName, userData string, nodeReq provision.NodeRequest, networkReq provision.NetworkRequest) (string, error) {
	isoPath := state.GetRelativePath(nodeName + "-cloud-init.iso")

	tmpDir, err := os.MkdirTemp("", "talos-cloud-init-iso")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	defer os.RemoveAll(tmpDir) //nolint:errcheck

	cidataDir := filepath.Join(tmpDir, "cidata")
	if err := os.Mkdir(cidataDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create cidata directory: %w", err)
	}

	// Create meta-data
	metaData := fmt.Sprintf(`instance-id: %s
local-hostname: %s
`, nodeName, nodeName)

	if err := os.WriteFile(filepath.Join(cidataDir, "meta-data"), []byte(metaData), 0o644); err != nil {
		return "", fmt.Errorf("failed to write meta-data: %w", err)
	}

	// Create user-data (Talos configuration)
	if err := os.WriteFile(filepath.Join(cidataDir, "user-data"), []byte(userData), 0o644); err != nil {
		return "", fmt.Errorf("failed to write user-data: %w", err)
	}

	// Create network-config (optional, for static IPs)
	if len(nodeReq.IPs) > 0 && len(networkReq.GatewayAddrs) > 0 {
		networkConfig := p.generateNetworkConfig(nodeReq, networkReq)
		if networkConfig != "" {
			if err := os.WriteFile(filepath.Join(cidataDir, "network-config"), []byte(networkConfig), 0o644); err != nil {
				return "", fmt.Errorf("failed to write network-config: %w", err)
			}
		}
	}

	// Create ISO using mkisofs, genisoimage, or xorriso
	var isoCmd string
	var isoArgs []string

	// Try mkisofs first (cdrtools), then genisoimage (cdrkit), then xorriso
	if path, err := exec.LookPath("mkisofs"); err == nil {
		isoCmd = path
		isoArgs = []string{"-joliet", "-rock", "-volid", "cidata", "-output", isoPath, cidataDir}
	} else if path, err := exec.LookPath("genisoimage"); err == nil {
		isoCmd = path
		isoArgs = []string{"-joliet", "-rock", "-volid", "cidata", "-output", isoPath, cidataDir}
	} else if path, err := exec.LookPath("xorriso"); err == nil {
		isoCmd = path
		isoArgs = []string{"-as", "mkisofs", "-joliet", "-rock", "-volid", "cidata", "-output", isoPath, cidataDir}
	} else {
		return "", fmt.Errorf("no ISO creation tool found (mkisofs, genisoimage, or xorriso). Please install one of them")
	}

	_, err = cmd.Run(isoCmd, isoArgs...)
	if err != nil {
		return "", fmt.Errorf("failed to create ISO using %s: %w", isoCmd, err)
	}

	return isoPath, nil
}

// generateNetworkConfig generates cloud-init network-config YAML.
func (p *provisioner) generateNetworkConfig(nodeReq provision.NodeRequest, networkReq provision.NetworkRequest) string {
	if len(nodeReq.IPs) == 0 {
		return "" // Use DHCP
	}

	networkConfig := "version: 2\nethernets:\n  eth0:\n"

	if len(nodeReq.IPs) > 0 {
		networkConfig += "    addresses:\n"
		for _, ip := range nodeReq.IPs {
			// Find matching CIDR
			for _, cidr := range networkReq.CIDRs {
				if cidr.Contains(ip) {
					networkConfig += fmt.Sprintf("      - %s/%d\n", ip.String(), cidr.Bits())
					break
				}
			}
		}
	}

	if len(networkReq.GatewayAddrs) > 0 {
		networkConfig += fmt.Sprintf("    gateway4: %s\n", networkReq.GatewayAddrs[0].String())
	}

	if len(networkReq.Nameservers) > 0 {
		networkConfig += "    nameservers:\n      addresses:\n"
		for _, ns := range networkReq.Nameservers {
			networkConfig += fmt.Sprintf("        - %s\n", ns.String())
		}
	}

	return networkConfig
}

