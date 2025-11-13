// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package proxmox

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/provision"
)

func TestProvisioner_Destroy(t *testing.T) {
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
		} else if strings.Contains(r.URL.Path, "/qemu") && strings.Contains(r.URL.Path, "/status/current") {
			// VM status query
			response = ProxmoxResponse{
				Data: json.RawMessage(`{"status": "stopped"}`), // VM is stopped
			}
		} else if strings.Contains(r.URL.Path, "/qemu") && !strings.Contains(r.URL.Path, "/status") {
			// VM deletion
			response = ProxmoxResponse{
				Data: json.RawMessage(`{"upid": "UPID:node1:1234567890:123:qmdelete:123:user:delete VM"}`),
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

	// Create a mock cluster with nodes
	cluster := &mockCluster{
		nodes: []provision.NodeInfo{
			{
				ID:   "100",
				Name: "test-node-1",
				Type: machine.TypeControlPlane,
			},
		},
		extraNodes: []provision.NodeInfo{},
	}

	// Test Destroy
	err = prov.Destroy(ctx, cluster)
	if err != nil {
		t.Errorf("Destroy() unexpected error: %v", err)
	}
}

func TestProvisioner_destroyNode_AlreadyDeleted(t *testing.T) {
	// Create a mock server that simulates VM already deleted
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/status/current") {
			// VM status query - return 404 (VM doesn't exist)
			w.WriteHeader(http.StatusNotFound)
			response := ProxmoxResponse{
				Error: "VM does not exist",
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}

		if strings.Contains(r.URL.Path, "/qemu") && !strings.Contains(r.URL.Path, "/status") {
			// VM deletion - return 404 (VM doesn't exist)
			w.WriteHeader(http.StatusNotFound)
			response := ProxmoxResponse{
				Error: "VM does not exist",
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}

		// Default response
		response := ProxmoxResponse{
			Data: json.RawMessage(`{"status": "ok"}`),
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

	// Create temporary state directory
	tmpDir, err := os.MkdirTemp("", "talos-test-destroy-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	nodeInfo := provision.NodeInfo{
		ID:   "100",
		Name: "test-node-1",
		Type: machine.TypeControlPlane,
	}

	opts := provision.Options{
		LogWriter: os.Stdout,
	}

	// Cast to concrete type to access private methods
	p := prov.(*provisioner)

	// Test destroyNode with already deleted VM
	err = p.destroyNode(ctx, "node1", nodeInfo, tmpDir, &opts)
	if err != nil {
		t.Errorf("destroyNode() unexpected error for already deleted VM: %v", err)
	}
}

func TestProvisioner_destroyNode_RunningVM(t *testing.T) {
	// Create a mock server that simulates running VM
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var response ProxmoxResponse

		if strings.Contains(r.URL.Path, "/status/current") {
			// VM status query - return running
			response = ProxmoxResponse{
				Data: json.RawMessage(`{"status": "running"}`),
			}
		} else if strings.Contains(r.URL.Path, "/status/stop") {
			// VM stop request
			response = ProxmoxResponse{
				Data: json.RawMessage(`"UPID:node1:1234567890:123:qmstop:123:user:stop VM"`),
			}
		} else if strings.Contains(r.URL.Path, "/tasks/") && strings.Contains(r.URL.Path, "/status") {
			// Task status
			response = ProxmoxResponse{
				Data: json.RawMessage(`{"status": "stopped", "exitstatus": "OK"}`),
			}
		} else if strings.Contains(r.URL.Path, "/qemu") && !strings.Contains(r.URL.Path, "/status") {
			// VM deletion
			response = ProxmoxResponse{
				Data: json.RawMessage(`"UPID:node1:1234567890:124:qmdelete:123:user:delete VM"`),
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

	// Create temporary state directory
	tmpDir, err := os.MkdirTemp("", "talos-test-destroy-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	nodeInfo := provision.NodeInfo{
		ID:   "100",
		Name: "test-node-1",
		Type: machine.TypeControlPlane,
	}

	opts := provision.Options{
		LogWriter: os.Stdout,
	}

	// Cast to concrete type to access private methods
	p := prov.(*provisioner)

	// Test destroyNode with running VM
	err = p.destroyNode(ctx, "node1", nodeInfo, tmpDir, &opts)
	if err != nil {
		t.Errorf("destroyNode() unexpected error for running VM: %v", err)
	}
}

// mockCluster implements provision.Cluster for testing
type mockCluster struct {
	nodes      []provision.NodeInfo
	extraNodes []provision.NodeInfo
	network    provision.NetworkInfo
}

func (m *mockCluster) Provisioner() string {
	return "proxmox"
}

func (m *mockCluster) StatePath() (string, error) {
	return "/tmp/mock-cluster", nil
}

func (m *mockCluster) Info() provision.ClusterInfo {
	return provision.ClusterInfo{
		Nodes:      m.nodes,
		ExtraNodes: m.extraNodes,
		Network:    m.network,
	}
}

func (m *mockCluster) Close() error {
	return nil
}

