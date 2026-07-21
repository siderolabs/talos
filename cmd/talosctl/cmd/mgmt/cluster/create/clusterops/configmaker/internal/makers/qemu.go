// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package makers

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-blockdevice/v2/encryption"
	"github.com/siderolabs/go-procfs/procfs"
	sideronet "github.com/siderolabs/net"
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create/clusterops"
	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/internal/firewallpatch"
	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	"github.com/siderolabs/talos/pkg/machinery/config/bundle"
	configbase "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/configpatcher"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/generate"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/types/block"
	"github.com/siderolabs/talos/pkg/machinery/config/types/cri"
	k8scfg "github.com/siderolabs/talos/pkg/machinery/config/types/k8s"
	metacfg "github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	networkcfg "github.com/siderolabs/talos/pkg/machinery/config/types/network"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	blockres "github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/provision"
	"github.com/siderolabs/talos/pkg/provision/providers/vm"
	"github.com/siderolabs/talos/pkg/provision/siderolinkbuilder"
)

const (
	// vipOffset is the offset from the network address of the CIDR to use for allocating the Virtual (shared) IP address, if enabled.
	vipOffset = 50
)

var _ ConfigMaker = &(Qemu{})

// Qemu is the maker for qemu.
type Qemu struct {
	*Maker[clusterops.Qemu]

	VIP               netip.Addr
	SideroLinkBuilder *siderolinkbuilder.SiderolinkBuilder
}

// NewQemu returns a new qemu Maker.
func NewQemu(ops MakerOptions[clusterops.Qemu]) (Qemu, error) {
	maker, err := New(ops)
	if err != nil {
		return Qemu{}, err
	}

	m := Qemu{Maker: &maker}

	m.SetExtraOptionsProvider(&m)

	if err := m.Init(); err != nil {
		return Qemu{}, err
	}

	return m, nil
}

// InitExtra implements ExtraOptionsProvider.
func (m *Qemu) InitExtra() error {
	if m.EOps.UseVIP {
		vip, err := sideronet.NthIPInNetwork(m.Cidrs[0], vipOffset)
		if err != nil {
			return err
		}

		m.VIP = vip

		m.InClusterEndpoint = "https://" + nethelpers.JoinHostPort(vip.String(), m.Ops.ControlPlanePort)
	}

	if err := m.initDisks(); err != nil {
		return err
	}

	if m.EOps.WithSiderolinkAgent.IsEnabled() {
		slb, err := siderolinkbuilder.New(context.Background(), m.GatewayIPs[0].String(), m.EOps.WithSiderolinkAgent.IsTLS())
		if err != nil {
			return err
		}

		m.SideroLinkBuilder = slb
	}

	m.initEndpoints()

	if m.Ops.WithJSONLogs {
		m.initJSONLogs()
	}

	if m.EOps.WithBGP {
		m.initBGP()
	}

	if m.EOps.WithBGPCLOS {
		if err := m.initBGPCLOS(); err != nil {
			return err
		}
	}

	return nil
}

func (m *Qemu) initEndpoints() {
	switch {
	case m.Ops.ForceEndpoint != "":
		// using non-default endpoints, provision additional cert SANs and fix endpoint list
		m.Endpoints = []string{m.Ops.ForceEndpoint}
	case m.Ops.ForceInitNodeAsEndpoint:
		m.Endpoints = []string{m.IPs[0][0].String()}
	case m.Endpoints == nil:
		// use control plane nodes as endpoints, client-side load-balancing
		for i := range m.Ops.Controlplanes {
			m.Endpoints = slices.Concat(m.Endpoints, []string{m.IPs[0][i].String()})
		}
	}
}

// AddExtraGenOps implements ExtraOptionsProvider.
func (m *Qemu) AddExtraGenOps() error {
	m.GenOps = slices.Concat(m.GenOps, []generate.Option{generate.WithInstallImage(m.EOps.NodeInstallImage)})

	if m.Ops.CustomCNIUrl != "" {
		m.GenOps = slices.Concat(
			m.GenOps,
			[]generate.Option{
				generate.WithCustomCNIUrl(m.Ops.CustomCNIUrl),
			},
		)
	}

	if m.EOps.UseVIP {
		if m.VersionContract.MultidocNetworkConfigSupported() {
			vipCfg := networkcfg.NewLayer2VIPConfigV1Alpha1(m.VIP.String())
			vipCfg.LinkName = m.Provisioner.GetFirstInterfaceName()

			ctr, err := container.New(vipCfg)
			if err != nil {
				return err
			}

			m.ConfigBundleOps = append(
				m.ConfigBundleOps,
				bundle.WithPatchControlPlane([]configpatcher.Patch{configpatcher.NewStrategicMergePatch(ctr)}),
			)
		} else {
			m.GenOps = slices.Concat(
				m.GenOps,
				[]generate.Option{generate.WithNetworkOptions(
					v1alpha1.WithNetworkInterfaceVirtualIP(m.Provisioner.GetFirstInterface(), m.VIP.String()),
				)},
			)
		}
	}

	// disable kexec, if bootloader is disabled, and
	// also disable kexec on arm64 due to https://github.com/siderolabs/talos/issues/12393
	if !m.EOps.BootloaderEnabled || m.EOps.TargetArch == "arm64" {
		m.GenOps = slices.Concat(m.GenOps, []generate.Option{
			generate.WithSysctls(map[string]string{
				"kernel.kexec_load_disabled": "1",
			}),
		})
	}

	if m.Ops.ForceEndpoint != "" {
		m.GenOps = slices.Concat(m.GenOps, []generate.Option{generate.WithAdditionalSubjectAltNames(m.Endpoints)})
	}

	for host, auth := range m.EOps.DownloadHTTPAuth {
		registryAuthConfig := cri.NewRegistryAuthConfigV1Alpha1(host)
		registryAuthConfig.RegistryUsername = auth.Username
		registryAuthConfig.RegistryPassword = auth.Password

		ctr, err := container.New(registryAuthConfig)
		if err != nil {
			return err
		}

		m.ConfigBundleOps = append(
			m.ConfigBundleOps,
			bundle.WithPatch([]configpatcher.Patch{configpatcher.NewStrategicMergePatch(ctr)}),
		)
	}

	return nil
}

// AddExtraProvisionOpts implements ExtraOptionsProvider.
func (m *Qemu) AddExtraProvisionOpts() error {
	m.ProvisionOps = slices.Concat(m.ProvisionOps, []provision.Option{
		provision.WithBootloader(m.EOps.BootloaderEnabled),
		provision.WithSkipInjectingExtraCmdline(m.EOps.SkipInjectingExtraCmdline),
		provision.WithUEFI(m.EOps.UefiEnabled),
		provision.WithTPM1_2(m.EOps.Tpm1_2Enabled),
		provision.WithTPM2(m.EOps.Tpm2Enabled),
		provision.WithIOMMU(m.EOps.WithIOMMU),
		provision.WithExtraUEFISearchPaths(m.EOps.ExtraUEFISearchPaths),
		provision.WithTargetArch(m.EOps.TargetArch),
		provision.WithSiderolinkAgent(m.EOps.WithSiderolinkAgent.IsEnabled()),
	})

	externalKubernetesEndpoint := m.Provisioner.GetExternalKubernetesControlPlaneEndpoint(m.ClusterRequest.Network, m.Ops.ControlPlanePort)

	// full-CLOS uses the BGP-advertised anycast VIP as the k8s endpoint (reachable via the host zebra
	// route), not the provisioner's host-side load balancer.
	if m.EOps.UseVIP || m.EOps.WithBGPCLOS {
		externalKubernetesEndpoint = "https://" + nethelpers.JoinHostPort(m.VIP.String(), m.Ops.ControlPlanePort)
	}

	m.ProvisionOps = slices.Concat(m.ProvisionOps, []provision.Option{provision.WithKubernetesEndpoint(externalKubernetesEndpoint)})

	return nil
}

// AddExtraConfigBundleOpts implements ExtraOptionsProvider.
func (m *Qemu) AddExtraConfigBundleOpts() error {
	if m.EOps.WithFirewall != "" {
		var defaultAction nethelpers.DefaultAction

		defaultAction, err := nethelpers.DefaultActionString(m.EOps.WithFirewall)
		if err != nil {
			return err
		}

		var controlplaneIPs []netip.Addr

		for i := range m.IPs {
			controlplaneIPs = slices.Concat(controlplaneIPs, m.IPs[i][:m.Ops.Controlplanes])
		}

		m.ConfigBundleOps = slices.Concat(m.ConfigBundleOps,
			[]bundle.Option{
				bundle.WithPatchControlPlane([]configpatcher.Patch{firewallpatch.ControlPlane(defaultAction, m.Cidrs, m.GatewayIPs, controlplaneIPs)}),
				bundle.WithPatchWorker([]configpatcher.Patch{firewallpatch.Worker(defaultAction, m.Cidrs, m.GatewayIPs)}),
			})
	}

	if err := m.addDiskEncryptionPatches(); err != nil {
		return err
	}

	m.ConfigBundleOps = slices.Concat(m.ConfigBundleOps, []bundle.Option{
		bundle.WithPatch(m.SideroLinkBuilder.ConfigPatches(m.EOps.WithSiderolinkAgent.IsTunnel())),
	})

	return nil
}

// ModifyClusterRequest implements ExtraOptionsProvider.
func (m *Qemu) ModifyClusterRequest() error {
	nameserverIPs, err := getNameserverIPs(m.EOps.Nameservers, m.GatewayIPs)
	if err != nil {
		return err
	}

	noMasqueradeCIDRs := make([]netip.Prefix, 0, len(m.EOps.NetworkNoMasqueradeCIDRs))

	for _, cidr := range m.EOps.NetworkNoMasqueradeCIDRs {
		var parsedCIDR netip.Prefix

		parsedCIDR, err = netip.ParsePrefix(cidr)
		if err != nil {
			return fmt.Errorf("error parsing non-masquerade CIDR %q: %w", cidr, err)
		}

		noMasqueradeCIDRs = append(noMasqueradeCIDRs, parsedCIDR)
	}

	err = m.validateNetworkChaosParams()
	if err != nil {
		return err
	}

	m.ClusterRequest.Network.CNI = provision.CNIConfig{
		BinPath:  m.EOps.CniBinPath,
		ConfDir:  m.EOps.CniConfDir,
		CacheDir: m.EOps.CniCacheDir,

		BundleURL: m.EOps.CniBundleURL,
	}
	m.ClusterRequest.Network.Nameservers = nameserverIPs
	m.ClusterRequest.Network.NoMasqueradeCIDRs = noMasqueradeCIDRs
	m.ClusterRequest.Network.DHCPSkipHostname = m.EOps.DHCPSkipHostname
	m.ClusterRequest.Network.NetworkChaos = m.EOps.NetworkChaos
	m.ClusterRequest.Network.Jitter = m.EOps.Jitter
	m.ClusterRequest.Network.Latency = m.EOps.Latency
	m.ClusterRequest.Network.PacketLoss = m.EOps.PacketLoss
	m.ClusterRequest.Network.PacketReorder = m.EOps.PacketReorder
	m.ClusterRequest.Network.PacketCorrupt = m.EOps.PacketCorrupt
	m.ClusterRequest.Network.Bandwidth = m.EOps.Bandwidth
	m.ClusterRequest.Network.Airgapped = m.EOps.Airgapped
	m.ClusterRequest.Network.ImageCachePath = m.EOps.ImageCachePath
	m.ClusterRequest.Network.ImageCacheTLSCertFile = m.EOps.ImageCacheTLSCertFile
	m.ClusterRequest.Network.ImageCacheTLSKeyFile = m.EOps.ImageCacheTLSKeyFile
	m.ClusterRequest.Network.ImageCachePort = m.EOps.ImageCachePort

	m.ClusterRequest.KernelPath = m.EOps.NodeVmlinuzPath
	m.ClusterRequest.InitramfsPath = m.EOps.NodeInitramfsPath
	m.ClusterRequest.ISOPath = m.EOps.NodeISOPath
	m.ClusterRequest.USBPath = m.EOps.NodeUSBPath
	m.ClusterRequest.UKIPath = m.EOps.NodeUKIPath
	m.ClusterRequest.IPXEBootScript = m.EOps.NodeIPXEBootScript
	m.ClusterRequest.DiskImagePath = m.EOps.NodeDiskImagePath

	return nil
}

func (m *Qemu) validateNetworkChaosParams() error {
	if !m.EOps.NetworkChaos {
		if m.EOps.Jitter != 0 || m.EOps.Latency != 0 || m.EOps.PacketLoss != 0 || m.EOps.PacketReorder != 0 || m.EOps.PacketCorrupt != 0 || m.EOps.Bandwidth != 0 {
			return errors.New("network chaos flags can only be used with network-chaos option enabled")
		}
	}

	return nil
}

// ModifyNodes implements ExtraOptionsProvider.
func (m *Qemu) ModifyNodes() error {
	var configInjectionMethod provision.ConfigInjectionMethod

	switch m.EOps.ConfigInjectionMethod {
	case "", "default", "http":
		configInjectionMethod = provision.ConfigInjectionMethodHTTP
	case "metal-iso":
		configInjectionMethod = provision.ConfigInjectionMethodMetalISO
	default:
		return fmt.Errorf("unknown config injection method %d", configInjectionMethod)
	}

	// full-CLOS nodes have no routable address before BGP comes up, but metal Configuration() gates the
	// HTTP config download on network readiness (a non-link-local address) — a deadlock, since the address
	// only arrives with the config. Deliver config via a local config volume (metal-iso) instead, which is
	// read with no network wait (and matches a real CLOS edge: config is out-of-band, not over the fabric).
	if m.EOps.WithBGPCLOS {
		configInjectionMethod = provision.ConfigInjectionMethodMetalISO
	}

	var extraKernelArgs *procfs.Cmdline

	if m.EOps.ExtraBootKernelArgs != "" || m.EOps.WithSiderolinkAgent.IsEnabled() {
		extraKernelArgs = procfs.NewCmdline(m.EOps.ExtraBootKernelArgs)
	}

	err := m.SideroLinkBuilder.SetKernelArgs(extraKernelArgs, m.EOps.WithSiderolinkAgent.IsTunnel())
	if err != nil {
		return err
	}

	for i := range m.ClusterRequest.Nodes {
		node := &m.ClusterRequest.Nodes[i]

		err := m.SideroLinkBuilder.DefineIPv6ForUUID(*node.UUID)
		if err != nil {
			return err
		}

		node.ConfigInjectionMethod = configInjectionMethod
		node.Quirks = quirks.New(m.Ops.TalosVersion)
		node.SkipInjectingConfig = m.Ops.SkipInjectingConfig
		node.BadRTC = m.EOps.BadRTC
		node.ExtraKernelArgs = extraKernelArgs
	}

	m.ClusterRequest.SiderolinkRequest = m.SideroLinkBuilder.SiderolinkRequest()

	return nil
}

func (m *Qemu) addDiskEncryptionPatches() error {
	var diskEncryptionPatches []configpatcher.Patch

	if m.EOps.EncryptStatePartition || m.EOps.EncryptEphemeralPartition {
		keys, err := m.getEncryptionKeys(m.EOps.DiskEncryptionKeyTypes)
		if err != nil {
			return err
		}

		if !m.VersionContract.VolumeConfigEncryptionSupported() {
			// legacy v1alpha1 flow to support booting old Talos versions
			patch, err := m.getLegacyDiskEncryptionPatch(keys)
			if err != nil {
				return err
			}

			diskEncryptionPatches = append(diskEncryptionPatches, patch)
		} else {
			for _, spec := range []struct {
				label   string
				enabled bool
			}{
				{label: constants.StatePartitionLabel, enabled: m.EOps.EncryptStatePartition},
				{label: constants.EphemeralPartitionLabel, enabled: m.EOps.EncryptEphemeralPartition},
			} {
				if !spec.enabled {
					continue
				}

				patch, err := m.getDiskEncryptionPatch(spec, keys)
				if err != nil {
					return err
				}

				diskEncryptionPatches = append(diskEncryptionPatches, patch)
			}
		}
	}

	m.ConfigBundleOps = slices.Concat(
		m.ConfigBundleOps,
		[]bundle.Option{bundle.WithPatch(diskEncryptionPatches)},
	)

	return nil
}

func (*Qemu) getDiskEncryptionPatch(spec struct {
	label   string
	enabled bool
}, keys []*v1alpha1.EncryptionKey,
) (configpatcher.StrategicMergePatch, error) {
	blockCfg := block.NewVolumeConfigV1Alpha1()
	blockCfg.MetaName = spec.label
	blockCfg.EncryptionSpec = block.EncryptionSpec{
		EncryptionProvider: blockres.EncryptionProviderLUKS2,
		EncryptionKeys:     convertEncryptionKeys(keys),
	}

	if spec.label != constants.StatePartitionLabel {
		for idx := range blockCfg.EncryptionSpec.EncryptionKeys {
			blockCfg.EncryptionSpec.EncryptionKeys[idx].KeyLockToSTATE = new(true)
		}
	}

	ctr, err := container.New(blockCfg)
	if err != nil {
		return nil, fmt.Errorf("error creating container for %q volume: %w", spec.label, err)
	}

	patch := configpatcher.NewStrategicMergePatch(ctr)

	return patch, nil
}

func (m *Qemu) getLegacyDiskEncryptionPatch(keys []*v1alpha1.EncryptionKey) (configpatcher.Patch, error) {
	diskEncryptionConfig := &v1alpha1.SystemDiskEncryptionConfig{}

	if m.EOps.EncryptStatePartition {
		diskEncryptionConfig.StatePartition = &v1alpha1.EncryptionConfig{
			EncryptionProvider: encryption.LUKS2,
			EncryptionKeys:     keys,
		}
	}

	if m.EOps.EncryptEphemeralPartition {
		diskEncryptionConfig.EphemeralPartition = &v1alpha1.EncryptionConfig{
			EncryptionProvider: encryption.LUKS2,
			EncryptionKeys:     keys,
		}
	}

	patchRaw := map[string]any{
		"machine": map[string]any{
			"systemDiskEncryption": diskEncryptionConfig,
		},
	}

	patchData, err := yaml.Marshal(patchRaw)
	if err != nil {
		return nil, fmt.Errorf("error marshaling patch: %w", err)
	}

	patch, err := configpatcher.LoadPatch(patchData)
	if err != nil {
		return nil, fmt.Errorf("error loading patch: %w", err)
	}

	return patch, nil
}

func (m *Qemu) initDisks() error {
	workerExtraDisks := make([]*provision.Disk, 0, len(m.EOps.Disks.Requests())-1)

	// Every node gets PrimaryDisks identical primary disks (cloned from the
	// first disk request). More than one lets a node build an MD array across
	// its primaries (e.g. a RAID1 boot drive).
	if m.EOps.PrimaryDisks < 1 {
		return fmt.Errorf("number of primary disks must be >= 1, got %d", m.EOps.PrimaryDisks)
	}

	primaryCount := m.EOps.PrimaryDisks

	primaryDisks := make([]*provision.Disk, 0, primaryCount)
	for range primaryCount {
		primaryDisks = append(primaryDisks, &provision.Disk{
			Size:            m.EOps.Disks.Requests()[0].Size.Bytes(),
			SkipPreallocate: !m.EOps.PreallocateDisks,
			Driver:          m.EOps.Disks.Requests()[0].Driver,
			BlockSize:       m.EOps.DiskBlockSize,
			Serial:          m.EOps.Disks.Requests()[0].Serial,
		})
	}

	// get worker extra disks
	for _, d := range m.EOps.Disks.Requests()[1:] {
		workerExtraDisks = append(workerExtraDisks, &provision.Disk{
			Size:            d.Size.Bytes(),
			SkipPreallocate: !m.EOps.PreallocateDisks,
			Driver:          d.Driver,
			BlockSize:       m.EOps.DiskBlockSize,
			Tag:             d.Tag,
			Serial:          d.Serial,
		})
	}

	m.ForEachNode(func(i int, node *provision.NodeRequest) {
		node.Disks = slices.Concat(node.Disks, primaryDisks)
	})

	if err := m.initExtraDisks(); err != nil {
		return err
	}

	m.ForEachNode(func(i int, node *provision.NodeRequest) {
		if node.Type == machine.TypeWorker {
			node.Disks = slices.Concat(node.Disks, workerExtraDisks)
		}
	})

	return nil
}

//nolint:gocyclo
func (m *Qemu) initExtraDisks() error {
	const GPTAlignment = 2 * 1024 * 1024 // 2 MB

	var (
		userVolumes    []*block.UserVolumeConfigV1Alpha1
		encryptionSpec block.EncryptionSpec
	)

	if m.EOps.EncryptUserVolumes {
		encryptionSpec.EncryptionProvider = blockres.EncryptionProviderLUKS2

		keys, err := m.getEncryptionKeys(m.EOps.DiskEncryptionKeyTypes)
		if err != nil {
			return err
		}

		encryptionSpec.EncryptionKeys = convertEncryptionKeys(keys)
	}

	disks := make([]*provision.Disk, 0, len(m.EOps.ClusterUserVolumes))

	for diskID, disk := range m.EOps.ClusterUserVolumes {
		var (
			volumes  = strings.Split(disk, ":")
			diskSize uint64
		)

		if len(volumes)%2 != 0 {
			return errors.New("failed to parse malformed volume definitions")
		}

		for j := 0; j < len(volumes); j += 2 {
			volumeName := volumes[j]
			volumeSize := volumes[j+1]

			userVolume := block.NewUserVolumeConfigV1Alpha1()
			userVolume.MetaName = volumeName
			userVolume.ProvisioningSpec = block.ProvisioningSpec{
				DiskSelectorSpec: block.DiskSelector{
					Match: cel.MustExpression(cel.ParseBooleanExpression(fmt.Sprintf("'%s' in disk.symlinks", m.Provisioner.UserDiskName(diskID+1)), celenv.DiskLocator())),
				},
				ProvisioningMinSize: block.MustByteSize(volumeSize),
				ProvisioningMaxSize: block.MustSize(volumeSize),
			}
			userVolume.EncryptionSpec = encryptionSpec

			userVolumes = append(userVolumes, userVolume)
			diskSize += userVolume.ProvisioningSpec.ProvisioningMaxSize.Value()
		}

		disks = append(disks, &provision.Disk{
			// add 2 MB per partition to make extra room for GPT and alignment
			Size:            diskSize + GPTAlignment*uint64(len(volumes)/2+1),
			SkipPreallocate: !m.EOps.PreallocateDisks,
			Driver:          "ide",
			BlockSize:       m.EOps.DiskBlockSize,
		})
	}

	if len(userVolumes) > 0 {
		ctr, err := container.New(xslices.Map(userVolumes, func(u *block.UserVolumeConfigV1Alpha1) configbase.Document { return u })...)
		if err != nil {
			return fmt.Errorf("failed to create user volumes container: %w", err)
		}

		userVolumePatches := []configpatcher.Patch{configpatcher.NewStrategicMergePatch(ctr)}
		m.ConfigBundleOps = slices.Concat(m.ConfigBundleOps, []bundle.Option{bundle.WithPatch(userVolumePatches)})
	}

	m.ForEachNode(func(i int, node *provision.NodeRequest) {
		node.Disks = slices.Concat(node.Disks, disks)
	})

	return nil
}

func (m *Qemu) getEncryptionKeys(diskEncryptionKeyTypes []string) ([]*v1alpha1.EncryptionKey, error) {
	var keys []*v1alpha1.EncryptionKey

	for i, key := range diskEncryptionKeyTypes {
		switch key {
		case "uuid":
			keys = append(keys, &v1alpha1.EncryptionKey{
				KeyNodeID: &v1alpha1.EncryptionKeyNodeID{},
				KeySlot:   i,
			})
		case "kms":
			var ip netip.Addr

			// get bridge IP
			ip, err := sideronet.NthIPInNetwork(m.Cidrs[0], 1)
			if err != nil {
				return nil, err
			}

			const port = 4050

			keys = append(keys, &v1alpha1.EncryptionKey{
				KeyKMS: &v1alpha1.EncryptionKeyKMS{
					KMSEndpoint: "grpc://" + nethelpers.JoinHostPort(ip.String(), port),
				},
				KeySlot: i,
			})

			m.ProvisionOps = slices.Concat(m.ProvisionOps, []provision.Option{provision.WithKMS(nethelpers.JoinHostPort("0.0.0.0", port))})
		case "tpm":
			keyTPM := &v1alpha1.EncryptionKeyTPM{}

			if m.VersionContract.SecureBootEnrollEnforcementSupported() {
				keyTPM.TPMCheckSecurebootStatusOnEnroll = new(true)
			}

			keys = append(keys, &v1alpha1.EncryptionKey{
				KeyTPM:  keyTPM,
				KeySlot: i,
			})
		default:
			return nil, fmt.Errorf("unknown key type %q", key)
		}
	}

	if len(keys) == 0 {
		return nil, errors.New("no disk encryption key types enabled")
	}

	return keys, nil
}

func convertEncryptionKeys(keys []*v1alpha1.EncryptionKey) []block.EncryptionKey {
	return xslices.Map(keys, func(k *v1alpha1.EncryptionKey) block.EncryptionKey {
		r := block.EncryptionKey{
			KeySlot: k.KeySlot,
		}

		if k.KeyKMS != nil {
			r.KeyKMS = new(block.EncryptionKeyKMS(*k.KeyKMS))
		}

		if k.KeyTPM != nil {
			encryptionKeyTPM := block.EncryptionKeyTPM{
				TPMCheckSecurebootStatusOnEnroll: k.KeyTPM.TPMCheckSecurebootStatusOnEnroll,
			}

			r.KeyTPM = new(encryptionKeyTPM)
		}

		if k.KeyNodeID != nil {
			r.KeyNodeID = new(block.EncryptionKeyNodeID(*k.KeyNodeID))
		}

		if k.KeyStatic != nil {
			r.KeyStatic = new(block.EncryptionKeyStatic(*k.KeyStatic))
		}

		return r
	})
}

func (m *Qemu) initJSONLogs() {
	const port = 4003

	m.ProvisionOps = slices.Concat(m.ProvisionOps, []provision.Option{provision.WithJSONLogs(nethelpers.JoinHostPort(m.GatewayIPs[0].String(), port))})

	cfg := container.NewV1Alpha1(
		&v1alpha1.Config{
			ConfigVersion: "v1alpha1",
			MachineConfig: &v1alpha1.MachineConfig{
				MachineLogging: &v1alpha1.LoggingConfig{
					LoggingDestinations: []v1alpha1.LoggingDestination{
						{
							LoggingEndpoint: &v1alpha1.Endpoint{
								URL: &url.URL{
									Scheme: "tcp",
									Host:   nethelpers.JoinHostPort(m.GatewayIPs[0].String(), port),
								},
							},
							LoggingFormat: "json_lines",
						},
					},
				},
			},
		},
	)

	m.ConfigBundleOps = slices.Concat(m.ConfigBundleOps, []bundle.Option{bundle.WithPatch([]configpatcher.Patch{configpatcher.NewStrategicMergePatch(cfg)})})
}

// initBGP starts an embedded gobgp fabric peer on the bridge gateway. Node-side BGPInstanceConfig is supplied
// separately via config patches (each node needs a unique loopback, which a shared patch cannot express).
func (m *Qemu) initBGP() {
	const (
		fabricASN = 65000
		nodeASN   = 65001
		advertise = "10.200.0.0/24"
	)

	m.ProvisionOps = slices.Concat(m.ProvisionOps, []provision.Option{
		provision.WithBGP(m.GatewayIPs[0].String(), m.Cidrs[0].String(), advertise, fabricASN, nodeASN),
	})
}

// initBGPCLOS configures the authentic full-CLOS BGP test: nodes have NO management net0 (only virtio
// fabric uplink(s) to a host fabric peer + a loopback identity), reachable only via BGP. Each node's
// config (a unique loopback on lo + an unnumbered BGPInstanceConfig peering over the fabric interfaces) is baked
// here per-node, because a no-net0 node is unreachable until BGP is up and so cannot be patched live.
func (m *Qemu) initBGPCLOS() error {
	const (
		fabricASN = 65000
		nodeASN   = 65001
		// advertise a default route: a no-net0 node has no other path off its loopback, so it relies on
		// the fabric peer for everything (host services, image pulls, internet — the host NATs it out).
		advertise = "0.0.0.0/0"

		// two dedicated fabric uplinks per node so the test exercises ECMP (and BFD failover).
		uplinks = 2
	)

	// The node loopback identities reuse the already-allocated bridge CIDR (the normal --cidr) node IPs:
	// the nodes are not on the bridge L2 (no net0), so the host's BGP /32s are always more specific than
	// its connected /24 and reachability is exclusively via BGP.
	natCIDR := m.Cidrs[0].String()

	m.ClusterRequest.Network.CLOSNoNet0 = true
	m.ClusterRequest.Network.FabricUplinks = uplinks

	// shared anycast k8s-API VIP: every control-plane node advertises this /32 over BGP, so the fabric
	// learns it from all CPs and ECMPs across them — the control-plane endpoint is HA "by design" (BGP
	// replaces the L2/ARP VIP). The cluster's k8s endpoint targets it (set below), reachable via the host
	// zebra route; no host-side load balancer is involved.
	vip, err := sideronet.NthIPInNetwork(m.Cidrs[0], vipOffset)
	if err != nil {
		return err
	}

	m.VIP = vip
	m.InClusterEndpoint = "https://" + nethelpers.JoinHostPort(vip.String(), m.Ops.ControlPlanePort)

	// the fabric NICs are pinned to deterministic PCI slots so their guest kernel names are known at
	// provision time (used both as the BGP neighbor interface and the talos.config link-local zone).
	ifaces := make([]string, uplinks)
	for u := range ifaces {
		ifaces[u] = vm.CLOSFabricIfaceName(u)
	}

	if m.PerNodePatches == nil {
		m.PerNodePatches = map[int][]configpatcher.Patch{}
	}

	for i := range m.ClusterRequest.Nodes {
		loopback := firstIPv4(m.ClusterRequest.Nodes[i].IPs)

		// only control-plane nodes carry/advertise the shared k8s-API VIP.
		nodeVIP := netip.Addr{}
		if t := m.ClusterRequest.Nodes[i].Type; t == machine.TypeControlPlane {
			nodeVIP = vip
		}

		// distinct per-node ASNs (eBGP) so the fabric peer can re-advertise one node's routes to another
		// without the AS_PATH loop check rejecting them.
		ctr, err := m.closNodeConfig(loopback, nodeVIP, uint32(nodeASN+i), ifaces)
		if err != nil {
			return err
		}

		m.PerNodePatches[i] = []configpatcher.Patch{configpatcher.NewStrategicMergePatch(ctr)}
	}

	// flannel auto-detects its VXLAN egress interface from the default route, but the full-CLOS default is
	// an ECMP route over the fabric uplinks (no single top-level interface) with an IPv6-link-local
	// next-hop — which flannel cannot resolve ("could not determine interface"). Pin it to the interface
	// that reaches the host gateway. The generated config already carries a KubeFlannelCNIConfig (multidoc)
	// which this patch merges into.
	if m.VersionContract.MultidocKubernetesConfigSupported() {
		flannel := k8scfg.NewKubeFlannelCNIConfigV1Alpha1()
		flannel.FlannelBackendType = constants.FlannelDefaultBackend
		flannel.FlannelExtraArgs = []string{"--iface-can-reach=" + m.GatewayIPs[0].String()}

		flannelCtr, err := container.New(flannel)
		if err != nil {
			return err
		}

		m.ConfigBundleOps = append(
			m.ConfigBundleOps,
			bundle.WithPatchControlPlane([]configpatcher.Patch{configpatcher.NewStrategicMergePatch(flannelCtr)}),
		)
	}

	m.ProvisionOps = slices.Concat(m.ProvisionOps, []provision.Option{
		provision.WithBGPCLOS(advertise, fabricASN, nodeASN, natCIDR),
	})

	return nil
}

// firstIPv4 returns the first IPv4 address in the list (the node's loopback identity is IPv4), falling
// back to the first address.
func firstIPv4(addrs []netip.Addr) netip.Addr {
	for _, a := range addrs {
		if a.Is4() {
			return a
		}
	}

	if len(addrs) > 0 {
		return addrs[0]
	}

	return netip.Addr{}
}

// closNodeConfig builds a full-CLOS node's baked config: a loopback /32 on lo (its identity, advertised by
// BGP) and an unnumbered BGPInstanceConfig peering with the host fabric peer over each fabric interface
// (multipath/ECMP when there is more than one, with BFD). On control-plane nodes a shared anycast k8s-API
// VIP /32 is also carried on lo (advertised by every CP, so the fabric ECMPs across them = CP-HA).
func (m *Qemu) closNodeConfig(loopback, vip netip.Addr, asn uint32, ifaces []string) (*container.Container, error) {
	// carry the loopback /32 on the always-present lo interface (the controller advertises it and filters
	// the 127/8 + ::1 loopback addresses).
	lo := networkcfg.NewLinkConfigV1Alpha1("lo")
	lo.LinkUp = new(true)
	lo.LinkAddresses = []networkcfg.AddressConfig{
		{AddressAddress: netip.PrefixFrom(loopback, loopback.BitLen())},
	}

	// control-plane nodes also carry the shared anycast k8s-API VIP /32; every CP advertises it, so the
	// fabric learns it from all CPs and ECMPs across them — the control-plane IP is HA "by design".
	if vip.IsValid() {
		lo.LinkAddresses = append(lo.LinkAddresses, networkcfg.AddressConfig{
			AddressAddress: netip.PrefixFrom(vip, vip.BitLen()),
		})
	}

	docs := make([]configbase.Document, 0, len(ifaces)*2+1)
	docs = append(docs, lo)

	// explicitly configure each fabric NIC (link up, no addresses): this marks them configured so Talos
	// does not start the default DHCP4 operator on them (there is no DHCP on the unnumbered fabric); they
	// stay IPv6-link-local only for unnumbered BGP.
	for _, iface := range ifaces {
		link := networkcfg.NewLinkConfigV1Alpha1(iface)
		link.LinkUp = new(true)

		docs = append(docs, link)
	}

	bgp := networkcfg.NewBGPInstanceConfigV1Alpha1("fabric")
	bgp.BGPLocalASN = asn
	bgp.BGPRouterID = metacfg.Addr{Addr: loopback}
	// source BGP-routed traffic from the loopback identity: the fabric uplinks have no address of their
	// own, so without this the kernel's source selection for the (cross-family, unnumbered) routes is
	// non-deterministic.
	bgp.BGPRouteSource = metacfg.Addr{Addr: loopback}
	bgp.BGPAdvertise = []string{"lo"}
	bgp.BGPMultipath = len(ifaces) > 1
	bgp.BGPNeighborConfigs = make([]networkcfg.BGPNeighborConfig, 0, len(ifaces))

	for _, iface := range ifaces {
		bgp.BGPNeighborConfigs = append(bgp.BGPNeighborConfigs, networkcfg.BGPNeighborConfig{
			NeighborLinkConfig: iface,
			NeighborBFDConfig: &networkcfg.BGPBFDConfig{
				BFDTransmitInterval: 300 * time.Millisecond,
				BFDReceiveInterval:  300 * time.Millisecond,
				BFDDetectMultiplier: 3,
			},
		})
	}

	docs = append(docs, bgp)

	// with no DHCP the node never learns a resolver; point it at the bridge gateway (the provisioner's
	// DNS), reachable via the BGP-learned default route, so image pulls by name resolve.
	resolver := networkcfg.NewResolverConfigV1Alpha1()
	resolver.ResolverNameservers = []networkcfg.NameserverConfig{
		{Address: metacfg.Addr{Addr: m.GatewayIPs[0]}},
	}

	docs = append(docs, resolver)

	return container.New(docs...)
}

func getNameserverIPs(nameservers []string, gatewayIPs []netip.Addr) ([]netip.Addr, error) {
	nameserverIPs := make([]netip.Addr, len(nameservers))

	if len(nameservers) == 0 {
		return gatewayIPs, nil
	}

	for i := range nameserverIPs {
		ip, err := netip.ParseAddr(nameservers[i])
		if err != nil {
			return nil, fmt.Errorf("failed parsing nameserver IP %q: %w", nameservers[i], err)
		}

		nameserverIPs[i] = ip
	}

	return nameserverIPs, nil
}
