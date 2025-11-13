// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package proxmox

import (
	"context"
	"fmt"
	"os"

	"github.com/siderolabs/go-pointer"
	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/bundle"
	configconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/configpatcher"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/generate"
	networkcfg "github.com/siderolabs/talos/pkg/machinery/config/types/network"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/provision"
	"github.com/siderolabs/talos/pkg/provision/providers/vm"
)

type provisioner struct {
	vm.Provisioner
	client *Client
	config *Config
}

// Config holds Proxmox provisioner configuration.
type Config struct {
	URL      string
	Username string
	Password string
	Token    string
	Secret   string
	Node     string
	Storage  string
	Bridge   string
	Insecure bool
}

// NewProvisioner initializes Proxmox provisioner.
func NewProvisioner(ctx context.Context) (provision.Provisioner, error) {
	// Load configuration from environment variables
	cfg := &Config{
		URL:      os.Getenv("PROXMOX_URL"),
		Username: os.Getenv("PROXMOX_USERNAME"),
		Password: os.Getenv("PROXMOX_PASSWORD"),
		Token:    os.Getenv("PROXMOX_TOKEN"),
		Secret:   os.Getenv("PROXMOX_SECRET"),
		Node:     os.Getenv("PROXMOX_NODE"),
		Storage:  os.Getenv("PROXMOX_STORAGE"),
		Bridge:   os.Getenv("PROXMOX_BRIDGE"),
		Insecure: os.Getenv("PROXMOX_INSECURE") == "true",
	}

	// Validate configuration
	if err := validateConfig(cfg); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Default values
	if cfg.Bridge == "" {
		cfg.Bridge = "vmbr0"
	}

	// Create Proxmox client
	client, err := NewClient(cfg.URL, cfg.Insecure)
	if err != nil {
		return nil, fmt.Errorf("failed to create Proxmox client: %w", err)
	}

	// Authenticate
	if cfg.Username != "" && cfg.Password != "" {
		if err := client.LoginWithUsernamePassword(ctx, cfg.Username, cfg.Password); err != nil {
			return nil, fmt.Errorf("failed to authenticate with username/password: %w", err)
		}
	} else if cfg.Token != "" && cfg.Secret != "" {
		if err := client.LoginWithToken(ctx, cfg.Token, cfg.Secret); err != nil {
			return nil, fmt.Errorf("failed to authenticate with token: %w", err)
		}
	} else {
		return nil, fmt.Errorf("either PROXMOX_USERNAME/PROXMOX_PASSWORD or PROXMOX_TOKEN/PROXMOX_SECRET must be set")
	}

	p := &provisioner{
		Provisioner: vm.Provisioner{
			Name: "proxmox",
		},
		client: client,
		config: cfg,
	}

	return p, nil
}

// GetStorageInfo gets storage information for the specified node.
// Returns storage and uploadStorage names, or error if unable to determine.
func (p *provisioner) GetStorageInfo(ctx context.Context, node string) (storage, uploadStorage string, err error) {
	// Check if client is authenticated first
	if !p.client.authenticated {
		return "", "", fmt.Errorf("Proxmox client is not authenticated - check your PROXMOX_USERNAME, PROXMOX_PASSWORD, and PROXMOX_URL environment variables")
	}

	// Log what we're trying to do
	fmt.Fprintf(os.Stderr, "🔍 Getting storage info for Proxmox node: %s\n", node)
	fmt.Fprintf(os.Stderr, "🔍 Proxmox API URL: %s\n", p.client.baseURL)

	// Get storage if not specified
	storage = p.config.Storage
	var storages []StorageInfo

	apiPath := fmt.Sprintf("/nodes/%s/storage", node)
	fmt.Fprintf(os.Stderr, "🔍 Making API call to: %s\n", apiPath)

	if storage == "" {
		fmt.Fprintf(os.Stderr, "🔍 No storage specified in config, auto-detecting best storage...\n")
		if err := p.client.Get(ctx, apiPath, &storages); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Failed to get storage list from Proxmox API\n")
			fmt.Fprintf(os.Stderr, "❌ API Path: %s\n", apiPath)
			fmt.Fprintf(os.Stderr, "❌ Error details: %v\n", err)
			fmt.Fprintf(os.Stderr, "❌ Possible causes:\n")
			fmt.Fprintf(os.Stderr, "   • Proxmox API authentication failed\n")
			fmt.Fprintf(os.Stderr, "   • Node '%s' not found or not accessible\n", node)
			fmt.Fprintf(os.Stderr, "   • Network connectivity issues\n")
			fmt.Fprintf(os.Stderr, "   • PROXMOX_INSECURE=true may be needed for self-signed certificates\n")
			return "", "", fmt.Errorf("failed to get storage list from Proxmox API (node: %s, path: %s): %w", node, apiPath, err)
		}
		fmt.Fprintf(os.Stderr, "✅ Successfully retrieved %d storage pools\n", len(storages))
		if len(storages) == 0 {
			fmt.Fprintf(os.Stderr, "❌ No storage pools found on node %s\n", node)
			fmt.Fprintf(os.Stderr, "❌ This usually means:\n")
			fmt.Fprintf(os.Stderr, "   • The node name is incorrect\n")
			fmt.Fprintf(os.Stderr, "   • The node is not responding\n")
			fmt.Fprintf(os.Stderr, "   • Storage pools are not configured\n")
			return "", "", fmt.Errorf("no storage pools found on node %s", node)
		}

		// Log available storages
		fmt.Fprintf(os.Stderr, "📋 Available storage pools:\n")
		for i, s := range storages {
			fmt.Fprintf(os.Stderr, "   %d. %s (type: %s, content: %s, used: %d/%d bytes)\n",
				i+1, s.Storage, s.Type, s.Content, s.Used, s.Total)
		}

		// Select best storage for VM disks
		fmt.Fprintf(os.Stderr, "🎯 Selecting best storage for VM disks...\n")
		storage = selectBestStorage(storages, os.Stderr)
		if storage == "" {
			fmt.Fprintf(os.Stderr, "❌ No suitable storage pool found for VM disks\n")
			fmt.Fprintf(os.Stderr, "❌ Requirements for VM storage:\n")
			fmt.Fprintf(os.Stderr, "   • Must support 'images' content type\n")
			fmt.Fprintf(os.Stderr, "   • Must have at least 10GB free space\n")
			fmt.Fprintf(os.Stderr, "   • Must not be in excluded list (local)\n")
			return "", "", fmt.Errorf("no suitable storage pool found for VM disks - check storage configuration")
		}
		fmt.Fprintf(os.Stderr, "✅ Selected storage for VM disks: %s\n", storage)
	} else {
		fmt.Fprintf(os.Stderr, "🔍 Using configured storage: %s\n", storage)
		// Still need to get storages for ISO upload detection
		if err := p.client.Get(ctx, apiPath, &storages); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Failed to get storage list for ISO upload detection\n")
			fmt.Fprintf(os.Stderr, "❌ API Path: %s\n", apiPath)
			fmt.Fprintf(os.Stderr, "❌ Error details: %v\n", err)
			return "", "", fmt.Errorf("failed to get storage list for ISO validation (node: %s): %w", node, err)
		}
		fmt.Fprintf(os.Stderr, "✅ Retrieved storage list for validation\n")
	}

	// Find storage that supports ISO uploads
	fmt.Fprintf(os.Stderr, "🔍 Finding storage for ISO uploads...\n")
	uploadStorage = findUploadStorage(storages, storage)
	fmt.Fprintf(os.Stderr, "✅ Selected ISO upload storage: %s\n", uploadStorage)

	if uploadStorage != storage {
		fmt.Fprintf(os.Stderr, "ℹ️  Note: ISO uploads will use '%s', VM disks will use '%s'\n", uploadStorage, storage)
	}

	return storage, uploadStorage, nil
}

// GetNode gets the Proxmox node name, auto-detecting if not set.
func (p *provisioner) GetNode(ctx context.Context) (string, error) {
	if p.config.Node != "" {
		return p.config.Node, nil
	}

	var nodes []NodeStatus
	if err := p.client.Get(ctx, "/nodes", &nodes); err != nil {
		return "", fmt.Errorf("failed to get nodes: %w", err)
	}
	if len(nodes) == 0 {
		return "", fmt.Errorf("no Proxmox nodes found")
	}

	return nodes[0].Node, nil
}

// validateConfig validates Proxmox provisioner configuration.
func validateConfig(cfg *Config) error {
	if cfg.URL == "" {
		return fmt.Errorf("PROXMOX_URL is required")
	}

	// Validate authentication
	hasUsernamePassword := cfg.Username != "" && cfg.Password != ""
	hasToken := cfg.Token != "" && cfg.Secret != ""

	if !hasUsernamePassword && !hasToken {
		return fmt.Errorf("either PROXMOX_USERNAME/PROXMOX_PASSWORD or PROXMOX_TOKEN/PROXMOX_SECRET must be set")
	}

	if hasUsernamePassword && hasToken {
		return fmt.Errorf("cannot use both username/password and token authentication")
	}

	return nil
}

// Close and release resources.
func (p *provisioner) Close() error {
	return nil
}

// GenOptions provides a list of additional config generate options.
func (p *provisioner) GenOptions(networkReq provision.NetworkRequest, contract *config.VersionContract) ([]generate.Option, []bundle.Option) {
	hasIPv4 := false
	hasIPv6 := false

	for _, cidr := range networkReq.CIDRs {
		if cidr.Addr().Is6() {
			hasIPv6 = true
		} else {
			hasIPv4 = true
		}
	}

	// Configure Cilium CNI instead of Flannel for better performance, security, and observability
	// Cilium provides:
	// - eBPF-based networking for better performance
	// - Advanced network policies and security features
	// - Hubble observability for network visibility
	// - Can replace kube-proxy for better performance
	ciliumConfig := &v1alpha1.CNIConfig{
		CNIName: "cilium",
		CNICilium: &v1alpha1.CiliumCNIConfig{
			CiliumEnableBPFNetworking: pointer.To(true),
			CiliumDisableKubeProxy:    pointer.To(true), // Replace kube-proxy with Cilium
			CiliumEnableHubble:        pointer.To(true), // Enable Hubble for observability
			CiliumEnableHubbleRelay:   pointer.To(true), // Enable Hubble Relay for cluster-wide observability
			CiliumEnableHubbleUI:      pointer.To(false), // Disable UI by default (can be enabled later)
			CiliumExtraArgs: []string{
				"--enable-ipv4=true",
				"--enable-ipv6=" + fmt.Sprintf("%v", hasIPv6),
				"--enable-hubble=true",
				"--enable-hubble-relay=true",
			},
		},
	}

	// Proxmox uses nocloud platform (set during config generation, not here)
	genOpts := []generate.Option{
		generate.WithInstallDisk("/dev/vda"),
		generate.WithClusterCNIConfig(ciliumConfig),
	}

	var bundleOpts []bundle.Option

	if contract.MultidocNetworkConfigSupported() {
		aliasConfig := networkcfg.NewLinkAliasConfigV1Alpha1("net0")
		aliasConfig.Selector = networkcfg.LinkSelector{
			Match: cel.MustExpression(cel.ParseBooleanExpression(`link.driver == "virtio_net"`, celenv.LinkLocator())),
		}

		documents := []configconfig.Document{aliasConfig}

		if hasIPv4 {
			dhcp4Config := networkcfg.NewDHCPv4ConfigV1Alpha1("net0")
			documents = append(documents, dhcp4Config)
		} else if hasIPv6 {
			dhcp6Config := networkcfg.NewDHCPv6ConfigV1Alpha1("net0")
			documents = append(documents, dhcp6Config)
		}

		ctr, err := container.New(documents...)
		if err != nil {
			panic(err)
		}

		bundleOpts = append(bundleOpts,
			bundle.WithPatch([]configpatcher.Patch{configpatcher.NewStrategicMergePatch(ctr)}),
		)
	} else {
		virtioSelector := v1alpha1.IfaceBySelector(v1alpha1.NetworkDeviceSelector{
			NetworkDeviceKernelDriver: "virtio_net",
		})

		genOpts = append(genOpts,
			generate.WithNetworkOptions(
				v1alpha1.WithNetworkInterfaceDHCP(virtioSelector, true),
				v1alpha1.WithNetworkInterfaceDHCPv4(virtioSelector, hasIPv4),
				v1alpha1.WithNetworkInterfaceDHCPv6(virtioSelector, hasIPv6),
			),
		)
	}

	return genOpts, bundleOpts
}

// GetInClusterKubernetesControlPlaneEndpoint returns the Kubernetes control plane endpoint.
func (p *provisioner) GetInClusterKubernetesControlPlaneEndpoint(networkReq provision.NetworkRequest, controlPlanePort int) string {
	// For Proxmox, use the first control plane node IP
	if len(networkReq.GatewayAddrs) > 0 {
		return "https://" + nethelpers.JoinHostPort(networkReq.GatewayAddrs[0].String(), controlPlanePort)
	}
	return ""
}

// GetExternalKubernetesControlPlaneEndpoint returns the Kubernetes control plane endpoint.
func (p *provisioner) GetExternalKubernetesControlPlaneEndpoint(networkReq provision.NetworkRequest, controlPlanePort int) string {
	// For Proxmox, external and in-cluster endpoints are same (no load balancer)
	return p.GetInClusterKubernetesControlPlaneEndpoint(networkReq, controlPlanePort)
}

// GetTalosAPIEndpoints returns a list of Talos API endpoints.
func (p *provisioner) GetTalosAPIEndpoints(provision.NetworkRequest) []string {
	// nil means that the API of controlplane endpoints should be used
	return nil
}

// GetFirstInterface returns first network interface name.
func (p *provisioner) GetFirstInterface() v1alpha1.IfaceSelector {
	return v1alpha1.IfaceBySelector(v1alpha1.NetworkDeviceSelector{
		NetworkDeviceKernelDriver: "virtio_net",
	})
}

// GetFirstInterfaceName return the first network interface name.
func (p *provisioner) GetFirstInterfaceName() string {
	return "eth0" // nocloud platform uses eth0
}

// UserDiskName returns disk device path.
func (p *provisioner) UserDiskName(index int) string {
	// Proxmox uses /dev/vda for first disk
	if index == 0 {
		return "/dev/vda"
	}
	return fmt.Sprintf("/dev/vd%c", 'a'+byte(index))
}

