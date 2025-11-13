// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package proxmox

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/netip"
	"regexp"
	"strings"
	"time"

	"github.com/siderolabs/talos/pkg/provision"
	"github.com/siderolabs/talos/pkg/provision/providers/proxmox/remote"
)

// SerialDiscovery holds discovered information from VM serial output.
type SerialDiscovery struct {
	NodeIP       netip.Addr
	TLSCertFingerprint string
}

// parseSerialLog parses VM serial output to extract node IP and TLS certificate fingerprint.
// Returns discovered info and whether all expected data was found.
func parseSerialLog(r io.Reader) (discovery SerialDiscovery, complete bool, err error) {
	scanner := bufio.NewScanner(r)

	// Regex patterns for extraction
	ipPattern := regexp.MustCompile(`this machine is reachable at:\s*(\d+\.\d+\.\d+\.\d+)`)
	fpPattern := regexp.MustCompile(`server certificate issued.*?fingerprint:\s*"([^"]+)"`)

	for scanner.Scan() {
		line := scanner.Text()

		// Check for node IP
		if matches := ipPattern.FindStringSubmatch(line); len(matches) > 1 {
			if addr, err := netip.ParseAddr(matches[1]); err == nil {
				discovery.NodeIP = addr
			}
		}

		// Check for TLS fingerprint
		if matches := fpPattern.FindStringSubmatch(line); len(matches) > 1 {
			discovery.TLSCertFingerprint = matches[1]
		}
	}

	if err := scanner.Err(); err != nil {
		return discovery, false, fmt.Errorf("error reading serial output: %w", err)
	}

	// Check if we have all required information
	complete = discovery.NodeIP.IsValid() && discovery.TLSCertFingerprint != ""

	return discovery, complete, nil
}

// DiscoverNodeInfo discovers node IP and TLS fingerprint using multiple methods.
// Tries in order: guest agent, ARP table, serial output, DHCP leases.
func (p *provisioner) DiscoverNodeInfo(ctx context.Context, proxmoxNode string, vmID int, timeout time.Duration) (discovery SerialDiscovery, err error) {
	// Get VM MAC address first (needed for ARP/DHCP lookups)
	macAddress := p.extractMACFromConfig(ctx, proxmoxNode, vmID)
	if macAddress == "" {
		return discovery, fmt.Errorf("failed to get MAC address for VM %d", vmID)
	}

	// Method 1: Try Proxmox guest agent (if available)
	if ip, err := p.discoverViaGuestAgent(ctx, proxmoxNode, vmID); err == nil && ip.IsValid() {
		discovery.NodeIP = ip
		// Still need fingerprint from serial output
		if fp, err := p.discoverFingerprintFromSerial(ctx, proxmoxNode, vmID, 30*time.Second); err == nil && fp != "" {
			discovery.TLSCertFingerprint = fp
			return discovery, nil
		}
		// If we have IP but no fingerprint, return partial discovery
		if discovery.NodeIP.IsValid() {
			return discovery, nil
		}
	}

	// Method 2: Try ARP table lookup
	if ip, err := p.discoverViaARP(ctx, proxmoxNode, macAddress); err == nil && ip.IsValid() {
		discovery.NodeIP = ip
		// Try to get fingerprint
		if fp, err := p.discoverFingerprintFromSerial(ctx, proxmoxNode, vmID, 30*time.Second); err == nil && fp != "" {
			discovery.TLSCertFingerprint = fp
			return discovery, nil
		}
		if discovery.NodeIP.IsValid() {
			return discovery, nil
		}
	}

	// Method 3: Try serial output with improved polling
	discovery, err = p.discoverViaSerialOutput(ctx, proxmoxNode, vmID, timeout)
	if err == nil && discovery.NodeIP.IsValid() {
		return discovery, nil
	}

	// Method 4: Try DHCP lease file (last resort)
	if ip, err := p.discoverViaDHCPLease(ctx, proxmoxNode, macAddress); err == nil && ip.IsValid() {
		discovery.NodeIP = ip
		// Try to get fingerprint
		if fp, err := p.discoverFingerprintFromSerial(ctx, proxmoxNode, vmID, 30*time.Second); err == nil && fp != "" {
			discovery.TLSCertFingerprint = fp
			return discovery, nil
		}
		if discovery.NodeIP.IsValid() {
			return discovery, nil
		}
	}

	// If we have IP but no fingerprint, that's acceptable (fingerprint can be obtained later)
	if discovery.NodeIP.IsValid() {
		return discovery, nil
	}

	return discovery, fmt.Errorf("failed to discover IP for VM %d using all methods", vmID)
}

// discoverViaGuestAgent tries to get VM IP via Proxmox guest agent API.
func (p *provisioner) discoverViaGuestAgent(ctx context.Context, proxmoxNode string, vmID int) (netip.Addr, error) {
	// Try guest agent network-get-interfaces endpoint
	var interfaces []struct {
		Name string `json:"name"`
		IPAddresses []struct {
			IPAddress string `json:"ip-address"`
			IPAddressType string `json:"ip-address-type"`
		} `json:"ip-addresses"`
	}

	path := fmt.Sprintf("/nodes/%s/qemu/%d/agent/network-get-interfaces", proxmoxNode, vmID)
	if err := p.client.Get(ctx, path, &interfaces); err != nil {
		return netip.Addr{}, err
	}

	// Find first IPv4 address
	for _, iface := range interfaces {
		for _, addr := range iface.IPAddresses {
			if addr.IPAddressType == "ipv4" {
				if ip, err := netip.ParseAddr(addr.IPAddress); err == nil {
					return ip, nil
				}
			}
		}
	}

	return netip.Addr{}, fmt.Errorf("no IPv4 address found in guest agent response")
}

// discoverViaARP looks up VM IP in Proxmox host ARP table.
func (p *provisioner) discoverViaARP(ctx context.Context, proxmoxNode, macAddress string) (netip.Addr, error) {
	remoteConfig := &remote.Config{URL: p.config.URL}
	host := remote.ResolveHost(remoteConfig, proxmoxNode)

	// Query ARP table
	cmd := fmt.Sprintf("arp -a | grep -i '%s' | awk '{print $2}' | tr -d '()'", macAddress)
	output, err := remote.RunCommand(remoteConfig, host, cmd)
	if err != nil {
		return netip.Addr{}, err
	}

	output = strings.TrimSpace(output)
	if output == "" {
		return netip.Addr{}, fmt.Errorf("MAC address not found in ARP table")
	}

	ip, err := netip.ParseAddr(output)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("failed to parse IP from ARP: %w", err)
	}

	return ip, nil
}

// discoverViaDHCPLease looks up VM IP in DHCP lease file.
func (p *provisioner) discoverViaDHCPLease(ctx context.Context, proxmoxNode, macAddress string) (netip.Addr, error) {
	remoteConfig := &remote.Config{URL: p.config.URL}
	host := remote.ResolveHost(remoteConfig, proxmoxNode)
	// Try common DHCP lease file locations
	leaseFiles := []string{
		"/var/lib/dhcp/dhcpd.leases",
		"/var/lib/dhcpd/dhcpd.leases",
		"/var/db/dhcpd.leases",
	}

	for _, leaseFile := range leaseFiles {
		cmd := fmt.Sprintf("grep -i '%s' %s 2>/dev/null | tail -1 | awk '{print $2}'", macAddress, leaseFile)
		output, err := remote.RunCommand(remoteConfig, host, cmd)
		if err != nil {
			continue
		}

		output = strings.TrimSpace(output)
		if output == "" {
			continue
		}

		ip, err := netip.ParseAddr(output)
		if err == nil {
			return ip, nil
		}
	}

	return netip.Addr{}, fmt.Errorf("MAC address not found in DHCP leases")
}

// discoverViaSerialOutput reads VM serial output with improved polling.
func (p *provisioner) discoverViaSerialOutput(ctx context.Context, proxmoxNode string, vmID int, timeout time.Duration) (SerialDiscovery, error) {
	remoteConfig := &remote.Config{URL: p.config.URL}
	host := remote.ResolveHost(remoteConfig, proxmoxNode)
	socketPath := fmt.Sprintf("/var/run/qemu-server/%d.serial0", vmID)

	// Use longer timeout and poll with exponential backoff
	startTime := time.Now()
	pollInterval := 2 * time.Second
	maxPollInterval := 10 * time.Second

	for time.Since(startTime) < timeout {
		// Read serial output with short timeout
		cmd := fmt.Sprintf("timeout 5 socat -u UNIX-CONNECT:%s - 2>/dev/null", socketPath)
		output, err := remote.RunCommand(remoteConfig, host, cmd)
		if err != nil {
			// Wait before retry
			time.Sleep(pollInterval)
			if pollInterval < maxPollInterval {
				pollInterval *= 2
			}
			continue
		}

		// Parse the serial output
		discovery, complete, err := parseSerialLog(strings.NewReader(output))
		if err == nil && complete {
			return discovery, nil
		}

		// If we have IP but not fingerprint, keep trying for fingerprint
		if discovery.NodeIP.IsValid() && discovery.TLSCertFingerprint == "" {
			// Continue polling for fingerprint
			time.Sleep(pollInterval)
			continue
		}

		// Wait before next poll
		time.Sleep(pollInterval)
		if pollInterval < maxPollInterval {
			pollInterval *= 2
		}
	}

	return SerialDiscovery{}, fmt.Errorf("timeout reading serial output for VM %d", vmID)
}

// discoverFingerprintFromSerial reads only the TLS fingerprint from serial output.
func (p *provisioner) discoverFingerprintFromSerial(ctx context.Context, proxmoxNode string, vmID int, timeout time.Duration) (string, error) {
	remoteConfig := &remote.Config{URL: p.config.URL}
	host := remote.ResolveHost(remoteConfig, proxmoxNode)
	socketPath := fmt.Sprintf("/var/run/qemu-server/%d.serial0", vmID)

	cmd := fmt.Sprintf("timeout %d socat -u UNIX-CONNECT:%s - 2>/dev/null | grep -i 'fingerprint' | tail -1", int(timeout.Seconds()), socketPath)
	output, err := remote.RunCommand(remoteConfig, host, cmd)
	if err != nil {
		return "", err
	}

	// Extract fingerprint from output
	fpPattern := regexp.MustCompile(`fingerprint:\s*"([^"]+)"`)
	if matches := fpPattern.FindStringSubmatch(output); len(matches) > 1 {
		return matches[1], nil
	}

	return "", fmt.Errorf("fingerprint not found in serial output")
}

// CaptureSerialOutput captures full serial console output from a VM for debugging.
// This is useful for seeing boot messages, UEFI output, and early boot logs.
// Uses multiple methods for maximum reliability:
// 1. Reads from serial log file (created via QEMU args: -serial file:/path)
// 2. Falls back to socket if file doesn't exist yet
// Returns the complete serial output as a string.
func (p *provisioner) CaptureSerialOutput(ctx context.Context, proxmoxNode string, vmID int, duration time.Duration) (string, error) {
	remoteConfig := &remote.Config{URL: p.config.URL}
	host := remote.ResolveHost(remoteConfig, proxmoxNode)

	// Method 1: Read from serial log file (most reliable for pre-boot messages)
	// This file is created by QEMU when we use: args: -chardev file,id=serial1,path=/path
	serialLogPath := fmt.Sprintf("/tmp/talos-vm-%d-serial.log", vmID)

	// Wait a bit for the file to be created and have some content
	time.Sleep(1 * time.Second)

	// Try to read from the log file first (most reliable)
	// Use tail -f to follow the file for the duration, then read the entire file
	// This captures both existing content and new content during the wait period
	cmd := fmt.Sprintf("if [ -f %s ]; then "+
		"INITIAL_SIZE=$(stat -c%%s %s 2>/dev/null || echo 0); "+
		"timeout %d tail -f %s 2>/dev/null & TAIL_PID=$!; "+
		"sleep %d; "+
		"kill $TAIL_PID 2>/dev/null; "+
		"cat %s 2>/dev/null; "+
		"else echo ''; fi",
		serialLogPath, serialLogPath, int(duration.Seconds())+1, serialLogPath, int(duration.Seconds()), serialLogPath)

	output, err := remote.RunCommand(remoteConfig, host, cmd)
	if err == nil && output != "" && !strings.Contains(output, "No such file") && len(strings.TrimSpace(output)) > 0 {
		// Got output from log file - clean up ANSI escape codes for readability
		// Remove common ANSI escape sequences (screen clearing, cursor positioning)
		cleaned := strings.ReplaceAll(output, "\033[2J", "") // Clear screen
		cleaned = strings.ReplaceAll(cleaned, "\033[01;01H", "") // Cursor to top-left
		cleaned = strings.ReplaceAll(cleaned, "\033[=3h", "") // Set mode
		cleaned = strings.TrimSpace(cleaned)
		if cleaned != "" {
			return cleaned, nil
		}
	}

	// Method 2: Fallback to socket (for VMs without file logging)
	socketPath := fmt.Sprintf("/var/run/qemu-server/%d.serial0", vmID)

	// Use socat to read from the serial socket
	// -u: unidirectional (read only)
	// Connect to the socket and read for the specified duration
	cmd = fmt.Sprintf("timeout %d socat -u UNIX-CONNECT:%s - 2>/dev/null || echo ''", int(duration.Seconds()), socketPath)

	output, err = remote.RunCommand(remoteConfig, host, cmd)
	if err != nil {
		return "", fmt.Errorf("failed to capture serial output (tried both file and socket): %w", err)
	}

	return output, nil
}

// UpdateClusterStateWithDiscovery updates the cluster state with discovered node information.
// This reconciles the endpoints as described in the deployment flow.
func UpdateClusterStateWithDiscovery(state *provision.State, vmID int, discovery SerialDiscovery) error {
	clusterInfo := state.Info()

	// Find and update the node in cluster state
	for i, node := range clusterInfo.Nodes {
		// VMID is stored as string in the ID field
		if node.ID == fmt.Sprintf("%d", vmID) {
			// Update the IP address with discovered real IP
			if discovery.NodeIP.IsValid() {
				clusterInfo.Nodes[i].IPs = []netip.Addr{discovery.NodeIP}
			}
			break
		}
	}

	// Save updated state
	return state.Save()
}
