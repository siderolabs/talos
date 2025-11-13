// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package remote

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Config holds Proxmox configuration needed for SSH operations.
type Config struct {
	URL string
}

// RunCommand executes an SSH command on the remote Proxmox node.
// The host can be overridden via PROXMOX_SSH_HOST environment variable.
func RunCommand(config *Config, host, command string) (string, error) {
	// Allow overriding SSH host explicitly
	if override := os.Getenv("PROXMOX_SSH_HOST"); override != "" {
		host = override
	}

	sshKey := os.Getenv("PROXMOX_SSH_KEY")
	if sshKey == "" {
		sshKey = filepath.Join(os.Getenv("HOME"), ".ssh", "proxmox_cluster_key")
	}

	cmd := exec.Command("ssh", "-i", sshKey, "-o", "StrictHostKeyChecking=no", "-o", "ConnectTimeout=5",
		fmt.Sprintf("root@%s", host), command)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("ssh command failed: %w, stderr: %s", err, stderr.String())
	}

	return strings.TrimSpace(stdout.String()), nil
}

// RunCommandContext executes an SSH command with context support.
func RunCommandContext(ctx context.Context, config *Config, host, command string) (string, error) {
	// Allow overriding SSH host explicitly
	if override := os.Getenv("PROXMOX_SSH_HOST"); override != "" {
		host = override
	}

	sshKey := os.Getenv("PROXMOX_SSH_KEY")
	if sshKey == "" {
		sshKey = filepath.Join(os.Getenv("HOME"), ".ssh", "proxmox_cluster_key")
	}

	cmd := exec.CommandContext(ctx, "ssh", "-i", sshKey, "-o", "StrictHostKeyChecking=no", "-o", "ConnectTimeout=5",
		fmt.Sprintf("root@%s", host), command)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("ssh command failed: %w, stderr: %s", err, stderr.String())
	}

	return strings.TrimSpace(stdout.String()), nil
}

// ResolveHost determines the best SSH hostname to reach the Proxmox node:
// 1) PROXMOX_SSH_HOST env var (highest priority)
// 2) Host part of PROXMOX_URL (without port)
// 3) Fallback to the Proxmox node name (e.g. 'laika')
func ResolveHost(config *Config, proxmoxNode string) string {
	if override := os.Getenv("PROXMOX_SSH_HOST"); override != "" {
		return override
	}

	if config != nil && config.URL != "" {
		if u, err := url.Parse(config.URL); err == nil && u.Hostname() != "" {
			return u.Hostname()
		}
	}

	return proxmoxNode
}

// ExtractHostFromURL extracts the hostname/IP from a Proxmox URL.
// Handles URLs like "https://10.10.10.10:8006" or "http://proxmox.example.com:8006"
func ExtractHostFromURL(urlStr string) string {
	if urlStr == "" {
		return ""
	}

	// Remove protocol
	if strings.HasPrefix(urlStr, "https://") {
		urlStr = strings.TrimPrefix(urlStr, "https://")
	} else if strings.HasPrefix(urlStr, "http://") {
		urlStr = strings.TrimPrefix(urlStr, "http://")
	}

	// Remove port if present
	if idx := strings.Index(urlStr, ":"); idx != -1 {
		urlStr = urlStr[:idx]
	}

	// Remove path if present
	if idx := strings.Index(urlStr, "/"); idx != -1 {
		urlStr = urlStr[:idx]
	}

	return urlStr
}

