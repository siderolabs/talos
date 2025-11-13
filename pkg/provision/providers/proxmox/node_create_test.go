// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package proxmox

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/provision"
	"github.com/siderolabs/talos/pkg/provision/providers/vm"
	"github.com/stretchr/testify/assert"
)

func TestProvisioner_createNode_Validation(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var response ProxmoxResponse

		if strings.Contains(r.URL.Path, "/nodes") && !strings.Contains(r.URL.Path, "/storage") {
			response = ProxmoxResponse{
				Data: json.RawMessage(`[{"node": "node1"}]`),
			}
		} else if strings.Contains(r.URL.Path, "/storage") {
			response = ProxmoxResponse{
				Data: json.RawMessage(`[{"storage": "local-lvm", "type": "lvm", "content": "images,iso"}]`),
			}
		} else if strings.Contains(r.URL.Path, "/qemu") {
			// Return empty VMs list for findAvailableVMID
			response = ProxmoxResponse{
				Data: json.RawMessage(`[]`),
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Save original env vars
	originalURL := os.Getenv("PROXMOX_URL")
	originalToken := os.Getenv("PROXMOX_TOKEN")
	originalSecret := os.Getenv("PROXMOX_SECRET")

	defer func() {
		if originalURL != "" {
			os.Setenv("PROXMOX_URL", originalURL)
		} else {
			os.Unsetenv("PROXMOX_URL")
		}
		if originalToken != "" {
			os.Setenv("PROXMOX_TOKEN", originalToken)
		} else {
			os.Unsetenv("PROXMOX_TOKEN")
		}
		if originalSecret != "" {
			os.Setenv("PROXMOX_SECRET", originalSecret)
		} else {
			os.Unsetenv("PROXMOX_SECRET")
		}
	}()

	// Set environment variables
	os.Setenv("PROXMOX_URL", server.URL)
	os.Setenv("PROXMOX_TOKEN", "test-token")
	os.Setenv("PROXMOX_SECRET", "test-secret")
	os.Unsetenv("PROXMOX_USERNAME")
	os.Unsetenv("PROXMOX_PASSWORD")

	ctx := context.Background()

	// Create provisioner
	prov, err := NewProvisioner(ctx)
	if err != nil {
		t.Fatalf("NewProvisioner() error = %v", err)
	}
	defer prov.Close()

	// Create temporary state directory path (don't create the directory yet)
	tmpDir := filepath.Join(os.TempDir(), fmt.Sprintf("talos-test-node-%d", time.Now().UnixNano()))

	// Create state (this will create the directory)
	state, err := provision.NewState(tmpDir, "proxmox", "test-cluster-validation")
	if err != nil {
		t.Fatalf("Failed to create state: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	clusterReq := provision.ClusterRequest{
		Name: "test-cluster",
		Network: provision.NetworkRequest{
			CIDRs: []netip.Prefix{
				netip.MustParsePrefix("10.0.0.0/24"),
			},
			GatewayAddrs: []netip.Addr{
				netip.MustParseAddr("10.0.0.1"),
			},
		},
	}

	// Create options
	opts := provision.Options{
		LogWriter: os.Stdout,
	}

	tests := []struct {
		name        string
		nodeReq     provision.NodeRequest
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid node",
			nodeReq: provision.NodeRequest{
				Name:       "test-node",
				IPs:        []netip.Addr{netip.MustParseAddr("10.0.0.10")},
				Type:       machine.TypeControlPlane,
				Memory:     2 * 1024 * 1024 * 1024, // 2GB
				NanoCPUs:   2 * 1000 * 1000 * 1000, // 2 CPUs
				Disks: []*provision.Disk{
					{Size: 20 * 1024 * 1024 * 1024}, // 20GB
				},
			},
			expectError: false,
		},
		{
			name: "insufficient memory",
			nodeReq: provision.NodeRequest{
				Name:       "test-node",
				IPs:        []netip.Addr{netip.MustParseAddr("10.0.0.10")},
				Type:       machine.TypeControlPlane,
				Memory:     1 * 1024 * 1024 * 1024, // 1GB - insufficient
				NanoCPUs:   2 * 1000 * 1000 * 1000,
				Disks: []*provision.Disk{
					{Size: 20 * 1024 * 1024 * 1024},
				},
			},
			expectError: true,
			errorMsg:    "memory must be at least 2GB",
		},
		{
			name: "no disks",
			nodeReq: provision.NodeRequest{
				Name:     "test-node",
				IPs:      []netip.Addr{netip.MustParseAddr("10.0.0.10")},
				Type:     machine.TypeControlPlane,
				Memory:   2 * 1024 * 1024 * 1024,
				NanoCPUs: 2 * 1000 * 1000 * 1000,
				Disks:    []*provision.Disk{}, // No disks
			},
			expectError: true,
			errorMsg:    "at least one disk is required",
		},
		{
			name: "insufficient disk size",
			nodeReq: provision.NodeRequest{
				Name:     "test-node",
				IPs:      []netip.Addr{netip.MustParseAddr("10.0.0.10")},
				Type:     machine.TypeControlPlane,
				Memory:   2 * 1024 * 1024 * 1024,
				NanoCPUs: 2 * 1000 * 1000 * 1000,
				Disks: []*provision.Disk{
					{Size: 5 * 1024 * 1024 * 1024}, // 5GB - insufficient
				},
			},
			expectError: true,
			errorMsg:    "disk 0 size must be at least 10GB",
		},
	}

	// Cast to concrete type to access private methods
	p := prov.(*provisioner)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset provisioner client to ensure clean state
			p.client.authenticated = true

			_, err := p.createNode(ctx, state, clusterReq, tt.nodeReq, &opts, "node1", "local-lvm", "local-lvm")

			if tt.expectError {
				if err == nil {
					t.Errorf("createNode() expected error, got none")
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("createNode() error = %v, expected to contain %s", err, tt.errorMsg)
				}
			} else {
				// For successful cases, we expect an error due to mock limitations
				// (missing actual cloud-init ISO creation tools, etc.)
				// But the validation should pass
				if err != nil && (strings.Contains(err.Error(), "memory") ||
					strings.Contains(err.Error(), "disk") ||
					strings.Contains(err.Error(), "CPU")) {
					t.Errorf("createNode() unexpected validation error: %v", err)
				}
			}
		})
	}
}

func TestProvisioner_createNodes_Empty(t *testing.T) {
	p := &provisioner{
		Provisioner: vm.Provisioner{
			Name: "proxmox",
		},
	}

	ctx := context.Background()

	// Create temporary state directory path (don't create the directory yet)
	tmpDir := filepath.Join(os.TempDir(), fmt.Sprintf("talos-test-nodes-%d", time.Now().UnixNano()))

	// Create state (this will create the directory)
	state, err := provision.NewState(tmpDir, "proxmox", "test-cluster-empty")
	if err != nil {
		t.Fatalf("Failed to create state: %v", err)
	}

	clusterReq := provision.ClusterRequest{
		Name: "test-cluster",
	}

	opts := provision.Options{
		LogWriter: os.Stdout,
	}

	// Test createNodes with empty node list
	nodeInfo, err := p.createNodes(ctx, state, clusterReq, []provision.NodeRequest{}, &opts, "node1", "local-lvm", "local-lvm")

	if err != nil {
		t.Errorf("createNodes() unexpected error with empty list: %v", err)
	}

	if len(nodeInfo) != 0 {
		t.Errorf("createNodes() returned %d nodes, expected 0", len(nodeInfo))
	}
}

func TestProvisioner_findAvailableVMID(t *testing.T) {
	// Create a mock server with some existing VMs
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var response ProxmoxResponse

		if strings.Contains(r.URL.Path, "/nodes/node1/qemu") {
			response = ProxmoxResponse{
				Data: json.RawMessage(`[
					{"vmid": 100, "name": "vm1"},
					{"vmid": 102, "name": "vm2"}
				]`),
			}
		} else if strings.Contains(r.URL.Path, "/nodes/node1/lxc") {
			response = ProxmoxResponse{
				Data: json.RawMessage(`[
					{"vmid": 101, "name": "ct1"}
				]`),
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Save original env vars
	originalURL := os.Getenv("PROXMOX_URL")
	originalToken := os.Getenv("PROXMOX_TOKEN")
	originalSecret := os.Getenv("PROXMOX_SECRET")

	defer func() {
		if originalURL != "" {
			os.Setenv("PROXMOX_URL", originalURL)
		} else {
			os.Unsetenv("PROXMOX_URL")
		}
		if originalToken != "" {
			os.Setenv("PROXMOX_TOKEN", originalToken)
		} else {
			os.Unsetenv("PROXMOX_TOKEN")
		}
		if originalSecret != "" {
			os.Setenv("PROXMOX_SECRET", originalSecret)
		} else {
			os.Unsetenv("PROXMOX_SECRET")
		}
	}()

	// Set environment variables
	os.Setenv("PROXMOX_URL", server.URL)
	os.Setenv("PROXMOX_TOKEN", "test-token")
	os.Setenv("PROXMOX_SECRET", "test-secret")
	os.Unsetenv("PROXMOX_USERNAME")
	os.Unsetenv("PROXMOX_PASSWORD")

	ctx := context.Background()

	// Create provisioner
	prov, err := NewProvisioner(ctx)
	if err != nil {
		t.Fatalf("NewProvisioner() error = %v", err)
	}
	defer prov.Close()

	// Cast to concrete type to access private methods
	p := prov.(*provisioner)

	// Test findAvailableVMID
	vmid, err := p.findAvailableVMID(ctx, "node1")
	if err != nil {
		t.Fatalf("findAvailableVMID() error = %v", err)
	}

	// Should find 103 (skipping 100, 101, 102)
	if vmid != 103 {
		t.Errorf("findAvailableVMID() = %d, expected 103", vmid)
	}
}

func TestProvisioner_validateNodeRequest_MultipleDisks(t *testing.T) {
	tests := []struct {
		name        string
		nodeReq     provision.NodeRequest
		expectError bool
		errorMsg    string
		expectResources func(*testing.T, vmResources)
	}{
		{
			name: "single disk (backwards compatibility)",
			nodeReq: provision.NodeRequest{
				Name:    "test-node",
				Memory:  2 * 1024 * 1024 * 1024, // 2GB
				NanoCPUs: 2000 * 1000 * 1000,    // 2 CPUs
				Disks: []*provision.Disk{
					{Size: 20 * 1024 * 1024 * 1024}, // 20GB
				},
			},
			expectError: false,
			expectResources: func(t *testing.T, res vmResources) {
				assert.Equal(t, int64(20), res.diskSizeGB)
				assert.Len(t, res.diskConfigs, 1)
				assert.Equal(t, int64(20), res.diskConfigs[0].sizeGB)
				assert.Equal(t, 0, res.diskConfigs[0].index)
			},
		},
		{
			name: "multiple disks (boot + data)",
			nodeReq: provision.NodeRequest{
				Name:    "storage-node",
				Memory:  4 * 1024 * 1024 * 1024, // 4GB
				NanoCPUs: 4000 * 1000 * 1000,    // 4 CPUs
				Disks: []*provision.Disk{
					{Size: 50 * 1024 * 1024 * 1024},  // 50GB boot disk
					{Size: 200 * 1024 * 1024 * 1024}, // 200GB data disk
					{Size: 1000 * 1024 * 1024 * 1024}, // 1000GB data disk
				},
			},
			expectError: false,
			expectResources: func(t *testing.T, res vmResources) {
				assert.Equal(t, int64(50), res.diskSizeGB) // Primary disk size for backwards compatibility
				assert.Len(t, res.diskConfigs, 3)

				// Check boot disk
				assert.Equal(t, int64(50), res.diskConfigs[0].sizeGB)
				assert.Equal(t, 0, res.diskConfigs[0].index)

				// Check data disks
				assert.Equal(t, int64(200), res.diskConfigs[1].sizeGB)
				assert.Equal(t, 1, res.diskConfigs[1].index)

				assert.Equal(t, int64(1000), res.diskConfigs[2].sizeGB)
				assert.Equal(t, 2, res.diskConfigs[2].index)
			},
		},
		{
			name: "boot disk too small",
			nodeReq: provision.NodeRequest{
				Name:    "test-node",
				Memory:  2 * 1024 * 1024 * 1024,
				NanoCPUs: 2000 * 1000 * 1000,
				Disks: []*provision.Disk{
					{Size: 5 * 1024 * 1024 * 1024}, // 5GB (too small for boot)
				},
			},
			expectError: true,
			errorMsg:    "disk 0 size must be at least 10GB",
		},
		{
			name: "data disk too small",
			nodeReq: provision.NodeRequest{
				Name:    "test-node",
				Memory:  2 * 1024 * 1024 * 1024,
				NanoCPUs: 2000 * 1000 * 1000,
				Disks: []*provision.Disk{
					{Size: 20 * 1024 * 1024 * 1024}, // 20GB boot disk (OK)
					{Size: 500 * 1024 * 1024},       // 0.5GB data disk (too small)
				},
			},
			expectError: true,
			errorMsg:    "disk 1 size must be at least 1GB",
		},
		{
			name: "no disks",
			nodeReq: provision.NodeRequest{
				Name:     "test-node",
				Memory:   2 * 1024 * 1024 * 1024,
				NanoCPUs: 2000 * 1000 * 1000,
				Disks:    []*provision.Disk{}, // Empty
			},
			expectError: true,
			errorMsg:    "at least one disk is required",
		},
		{
			name: "too many disks (exceeds Proxmox limit)",
			nodeReq: provision.NodeRequest{
				Name:    "test-node",
				Memory:  4 * 1024 * 1024 * 1024,
				NanoCPUs: 4000 * 1000 * 1000,
				Disks: func() []*provision.Disk {
					disks := make([]*provision.Disk, 17) // 17 disks (exceeds virtio0-virtio15 limit)
					for i := range disks {
						disks[i] = &provision.Disk{Size: 20 * 1024 * 1024 * 1024} // 20GB each
					}
					return disks
				}(),
			},
			expectError: true,
			errorMsg:    "too many disks: Proxmox supports maximum 16 virtio disks",
		},
		{
			name: "disk size exceeds maximum",
			nodeReq: provision.NodeRequest{
				Name:    "test-node",
				Memory:  2 * 1024 * 1024 * 1024,
				NanoCPUs: 2000 * 1000 * 1000,
				Disks: []*provision.Disk{
					{Size: 20 * 1024 * 1024 * 1024},                    // 20GB boot disk (OK)
					{Size: 65 * 1024 * 1024 * 1024 * 1024},             // 65TB data disk (exceeds 64TB limit)
				},
			},
			expectError: true,
			errorMsg:    "disk 1 size exceeds maximum",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &provisioner{}
			resources, err := p.validateNodeRequest(tt.nodeReq)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
				if tt.expectResources != nil {
					tt.expectResources(t, resources)
				}
			}
		})
	}
}

