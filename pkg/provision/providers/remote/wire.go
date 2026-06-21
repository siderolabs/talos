// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package remote

import (
	"encoding/json"
	"fmt"
	"net/netip"

	"github.com/google/uuid"
	"github.com/siderolabs/go-procfs/procfs"

	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
	"github.com/siderolabs/talos/pkg/provision"
)

// wireClusterRequest mirrors provision.ClusterRequest with non-JSON-encodable
// fields (config.Provider, *procfs.Cmdline, quirks.Quirks) replaced by their
// canonical serialized forms.
type wireClusterRequest struct {
	Name              string                      `json:"name"`
	Network           provision.NetworkRequest    `json:"network"`
	Nodes             []wireNodeRequest           `json:"nodes"`
	Image             string                      `json:"image,omitempty"`
	KernelPath        string                      `json:"kernel_path,omitempty"`
	InitramfsPath     string                      `json:"initramfs_path,omitempty"`
	ISOPath           string                      `json:"iso_path,omitempty"`
	USBPath           string                      `json:"usb_path,omitempty"`
	UKIPath           string                      `json:"uki_path,omitempty"`
	DiskImagePath     string                      `json:"disk_image_path,omitempty"`
	IPXEBootScript    string                      `json:"ipxe_boot_script,omitempty"`
	KMSEndpoint       string                      `json:"kms_endpoint,omitempty"`
	SelfExecutable    string                      `json:"self_executable,omitempty"`
	StateDirectory    string                      `json:"state_directory,omitempty"`
	SiderolinkRequest provision.SiderolinkRequest `json:"siderolink_request"`
}

// wireNodeRequest mirrors provision.NodeRequest. Containers-only fields
// (Mounts) are dropped — the remote provisioner is QEMU-only.
type wireNodeRequest struct {
	Name                  string                          `json:"name"`
	IPs                   []netip.Addr                    `json:"ips,omitempty"`
	Type                  machine.Type                    `json:"type"`
	TalosVersion          string                          `json:"talos_version,omitempty"`
	Config                []byte                          `json:"config,omitempty"`
	ConfigInjectionMethod provision.ConfigInjectionMethod `json:"config_injection_method,omitempty"`
	NanoCPUs              int64                           `json:"nano_cpus,omitempty"`
	Memory                int64                           `json:"memory,omitempty"`
	Disks                 []*provision.Disk               `json:"disks,omitempty"`
	Ports                 []string                        `json:"ports,omitempty"`
	SkipInjectingConfig   bool                            `json:"skip_injecting_config,omitempty"`
	DefaultBootOrder      string                          `json:"default_boot_order,omitempty"`
	ExtraKernelArgs       string                          `json:"extra_kernel_args,omitempty"`
	SDStubKernelArgs      string                          `json:"sd_stub_kernel_args,omitempty"`
	UUID                  *uuid.UUID                      `json:"uuid,omitempty"`
	BadRTC                bool                            `json:"bad_rtc,omitempty"`
	PXEBooted             bool                            `json:"pxe_booted,omitempty"`
	TFTPServer            string                          `json:"tftp_server,omitempty"`
	IPXEBootFilename      string                          `json:"ipxe_boot_filename,omitempty"`
}

// wireCluster is the JSON-encoded form of a provision.Cluster.
type wireCluster struct {
	ProvisionerName string                `json:"provisioner"`
	StatePath       string                `json:"state_path"`
	Info            provision.ClusterInfo `json:"info"`
}

// MarshalClusterRequest converts a ClusterRequest to its wire form (with
// config.Provider and procfs.Cmdline serialized to their canonical
// representations) and JSON-encodes it.
//
// The remote provisioner is QEMU-only, so NodeRequest.Mounts (container-only)
// is dropped.
func MarshalClusterRequest(req provision.ClusterRequest) ([]byte, error) {
	w := wireClusterRequest{
		Name:              req.Name,
		Network:           req.Network,
		Nodes:             make([]wireNodeRequest, 0, len(req.Nodes)),
		Image:             req.Image,
		KernelPath:        req.KernelPath,
		InitramfsPath:     req.InitramfsPath,
		ISOPath:           req.ISOPath,
		USBPath:           req.USBPath,
		UKIPath:           req.UKIPath,
		DiskImagePath:     req.DiskImagePath,
		IPXEBootScript:    req.IPXEBootScript,
		KMSEndpoint:       req.KMSEndpoint,
		SelfExecutable:    req.SelfExecutable,
		StateDirectory:    req.StateDirectory,
		SiderolinkRequest: req.SiderolinkRequest,
	}

	for i := range req.Nodes {
		wn, err := nodeToWire(&req.Nodes[i])
		if err != nil {
			return nil, fmt.Errorf("node %d (%s): %w", i, req.Nodes[i].Name, err)
		}

		w.Nodes = append(w.Nodes, wn)
	}

	return json.Marshal(w)
}

// UnmarshalClusterRequest reverses MarshalClusterRequest.
func UnmarshalClusterRequest(b []byte) (provision.ClusterRequest, error) {
	var w wireClusterRequest

	if err := json.Unmarshal(b, &w); err != nil {
		return provision.ClusterRequest{}, fmt.Errorf("decode cluster request: %w", err)
	}

	out := provision.ClusterRequest{
		Name:              w.Name,
		Network:           w.Network,
		Image:             w.Image,
		KernelPath:        w.KernelPath,
		InitramfsPath:     w.InitramfsPath,
		ISOPath:           w.ISOPath,
		USBPath:           w.USBPath,
		UKIPath:           w.UKIPath,
		DiskImagePath:     w.DiskImagePath,
		IPXEBootScript:    w.IPXEBootScript,
		KMSEndpoint:       w.KMSEndpoint,
		SelfExecutable:    w.SelfExecutable,
		StateDirectory:    w.StateDirectory,
		SiderolinkRequest: w.SiderolinkRequest,
	}

	for i := range w.Nodes {
		n, err := nodeFromWire(&w.Nodes[i])
		if err != nil {
			return provision.ClusterRequest{}, fmt.Errorf("node %d (%s): %w", i, w.Nodes[i].Name, err)
		}

		out.Nodes = append(out.Nodes, n)
	}

	return out, nil
}

func nodeToWire(n *provision.NodeRequest) (wireNodeRequest, error) {
	w := wireNodeRequest{
		Name:                  n.Name,
		IPs:                   n.IPs,
		Type:                  n.Type,
		ConfigInjectionMethod: n.ConfigInjectionMethod,
		NanoCPUs:              n.NanoCPUs,
		Memory:                n.Memory,
		Disks:                 n.Disks,
		Ports:                 n.Ports,
		SkipInjectingConfig:   n.SkipInjectingConfig,
		DefaultBootOrder:      n.DefaultBootOrder,
		UUID:                  n.UUID,
		BadRTC:                n.BadRTC,
		PXEBooted:             n.PXEBooted,
		TFTPServer:            n.TFTPServer,
		IPXEBootFilename:      n.IPXEBootFilename,
	}

	if n.Config != nil {
		b, err := n.Config.Bytes()
		if err != nil {
			return wireNodeRequest{}, fmt.Errorf("serialize node config: %w", err)
		}

		w.Config = b
	}

	if n.ExtraKernelArgs != nil {
		w.ExtraKernelArgs = string(n.ExtraKernelArgs.Bytes())
	}

	if n.SDStubKernelArgs != nil {
		w.SDStubKernelArgs = string(n.SDStubKernelArgs.Bytes())
	}

	if v := n.Quirks.Version(); v != nil {
		w.TalosVersion = v.String()
	}

	return w, nil
}

func nodeFromWire(w *wireNodeRequest) (provision.NodeRequest, error) {
	n := provision.NodeRequest{
		Name:                  w.Name,
		IPs:                   w.IPs,
		Type:                  w.Type,
		Quirks:                quirks.New(w.TalosVersion),
		ConfigInjectionMethod: w.ConfigInjectionMethod,
		NanoCPUs:              w.NanoCPUs,
		Memory:                w.Memory,
		Disks:                 w.Disks,
		Ports:                 w.Ports,
		SkipInjectingConfig:   w.SkipInjectingConfig,
		DefaultBootOrder:      w.DefaultBootOrder,
		UUID:                  w.UUID,
		BadRTC:                w.BadRTC,
		PXEBooted:             w.PXEBooted,
		TFTPServer:            w.TFTPServer,
		IPXEBootFilename:      w.IPXEBootFilename,
	}

	if len(w.Config) > 0 {
		cfg, err := configloader.NewFromBytes(w.Config)
		if err != nil {
			return provision.NodeRequest{}, fmt.Errorf("deserialize node config: %w", err)
		}

		n.Config = cfg
	}

	if w.ExtraKernelArgs != "" {
		n.ExtraKernelArgs = procfs.NewCmdline(w.ExtraKernelArgs)
	}

	if w.SDStubKernelArgs != "" {
		n.SDStubKernelArgs = procfs.NewCmdline(w.SDStubKernelArgs)
	}

	return n, nil
}

// MarshalCluster serializes a provision.Cluster.
func MarshalCluster(c provision.Cluster) ([]byte, error) {
	statePath, err := c.StatePath()
	if err != nil {
		return nil, fmt.Errorf("resolve cluster state path: %w", err)
	}

	return json.Marshal(wireCluster{
		ProvisionerName: c.Provisioner(),
		StatePath:       statePath,
		Info:            c.Info(),
	})
}

// UnmarshalCluster reverses MarshalCluster, returning a *Cluster.
func UnmarshalCluster(b []byte) (*Cluster, error) {
	var w wireCluster

	if err := json.Unmarshal(b, &w); err != nil {
		return nil, fmt.Errorf("decode cluster: %w", err)
	}

	return &Cluster{wire: w}, nil
}

// wireOptions is the serializable subset of provision.Options. Non-portable
// fields are deliberately excluded: LogWriter / TalosConfig / TalosClient are
// process-local; TargetArch and ExtraUEFISearchPaths are server-decided
// (the server runs the VMs); Docker* applies only to the docker provisioner.
type wireOptions struct {
	KubernetesEndpoint        string `json:"kubernetes_endpoint"`
	BootloaderEnabled         bool   `json:"bootloader_enabled"`
	SkipInjectingExtraCmdline bool   `json:"skip_injecting_extra_cmdline"`
	UEFIEnabled               bool   `json:"uefi_enabled"`
	TPM1_2Enabled             bool   `json:"tpm1_2_enabled"`
	TPM2Enabled               bool   `json:"tpm2_enabled"`
	IOMMUEnabled              bool   `json:"iommu_enabled"`
	KMSEndpoint               string `json:"kms_endpoint"`
	JSONLogsEndpoint          string `json:"json_logs_endpoint"`
	SiderolinkEnabled         bool   `json:"siderolink_enabled"`
	DeleteStateOnErr          bool   `json:"delete_state_on_err"`

	// BGP test fabric peer (runs server-side, where the VMs and the host FIB live).
	BGPEnabled       bool   `json:"bgp_enabled"`
	BGPCLOS          bool   `json:"bgp_clos"`
	BGPListenAddress string `json:"bgp_listen_address"`
	BGPNeighborRange string `json:"bgp_neighbor_range"`
	BGPAdvertise     string `json:"bgp_advertise"`
	BGPLocalASN      uint32 `json:"bgp_local_asn"`
	BGPPeerASN       uint32 `json:"bgp_peer_asn"`
	BGPLoopbackCIDR  string `json:"bgp_loopback_cidr"`
}

// MarshalOptions resolves a provision.Option list into its serializable
// wire form (JSON). Used by the remote client to ship boot parameters
// (UEFI, TPM, bootloader, ...) the server would otherwise default to zero.
func MarshalOptions(opts []provision.Option) ([]byte, error) {
	o := provision.DefaultOptions()

	for _, opt := range opts {
		if err := opt(&o); err != nil {
			return nil, fmt.Errorf("apply option: %w", err)
		}
	}

	return json.Marshal(wireOptions{
		KubernetesEndpoint:        o.KubernetesEndpoint,
		BootloaderEnabled:         o.BootloaderEnabled,
		SkipInjectingExtraCmdline: o.SkipInjectingExtraCmdline,
		UEFIEnabled:               o.UEFIEnabled,
		TPM1_2Enabled:             o.TPM1_2Enabled,
		TPM2Enabled:               o.TPM2Enabled,
		IOMMUEnabled:              o.IOMMUEnabled,
		KMSEndpoint:               o.KMSEndpoint,
		JSONLogsEndpoint:          o.JSONLogsEndpoint,
		SiderolinkEnabled:         o.SiderolinkEnabled,
		DeleteStateOnErr:          o.DeleteStateOnErr,
		BGPEnabled:                o.BGPEnabled,
		BGPCLOS:                   o.BGPCLOS,
		BGPListenAddress:          o.BGPListenAddress,
		BGPNeighborRange:          o.BGPNeighborRange,
		BGPAdvertise:              o.BGPAdvertise,
		BGPLocalASN:               o.BGPLocalASN,
		BGPPeerASN:                o.BGPPeerASN,
		BGPLoopbackCIDR:           o.BGPLoopbackCIDR,
	})
}

// UnmarshalOptions reverses MarshalOptions into a provision.Option list.
// An empty payload yields no options (caller falls back to defaults).
func UnmarshalOptions(b []byte) ([]provision.Option, error) {
	if len(b) == 0 {
		return nil, nil
	}

	var w wireOptions

	if err := json.Unmarshal(b, &w); err != nil {
		return nil, fmt.Errorf("decode options: %w", err)
	}

	opts := []provision.Option{
		provision.WithKubernetesEndpoint(w.KubernetesEndpoint),
		provision.WithBootloader(w.BootloaderEnabled),
		provision.WithSkipInjectingExtraCmdline(w.SkipInjectingExtraCmdline),
		provision.WithUEFI(w.UEFIEnabled),
		provision.WithTPM1_2(w.TPM1_2Enabled),
		provision.WithTPM2(w.TPM2Enabled),
		provision.WithIOMMU(w.IOMMUEnabled),
		provision.WithKMS(w.KMSEndpoint),
		provision.WithJSONLogs(w.JSONLogsEndpoint),
		provision.WithSiderolinkAgent(w.SiderolinkEnabled),
		provision.WithDeleteOnErr(w.DeleteStateOnErr),
	}

	switch {
	case w.BGPCLOS:
		opts = append(opts, provision.WithBGPCLOS(w.BGPAdvertise, w.BGPLocalASN, w.BGPPeerASN, w.BGPLoopbackCIDR))
	case w.BGPEnabled:
		opts = append(opts, provision.WithBGP(w.BGPListenAddress, w.BGPNeighborRange, w.BGPAdvertise, w.BGPLocalASN, w.BGPPeerASN))
	}

	return opts, nil
}
