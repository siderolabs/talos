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

	"github.com/ghodss/yaml"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-blockdevice/v2/encryption"
	"github.com/siderolabs/go-pointer"
	"github.com/siderolabs/go-procfs/procfs"
	sideronet "github.com/siderolabs/net"

	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create/clusterops"
	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create/clusterops/configmaker/internal/siderolinkbuilder"
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
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	blockres "github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/provision"
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

	if m.Ops.WithJSONLogs {
		m.initJSONLogs()
	}

	return nil
}

// AddExtraGenOps implements ExtraOptionsProvider.
func (m *Qemu) AddExtraGenOps() error {
	m.GenOps = slices.Concat(m.GenOps, []generate.Option{generate.WithInstallImage(m.EOps.NodeInstallImage)})

	if m.Ops.CustomCNIUrl != "" {
		m.GenOps = slices.Concat(m.GenOps, []generate.Option{generate.WithClusterCNIConfig(&v1alpha1.CNIConfig{
			CNIName: constants.CustomCNI,
			CNIUrls: []string{m.Ops.CustomCNIUrl},
		})})

		if m.EOps.UseVIP {
			m.GenOps = slices.Concat(m.GenOps,
				[]generate.Option{generate.WithNetworkOptions(
					v1alpha1.WithNetworkInterfaceVirtualIP(m.Provisioner.GetFirstInterface(), m.VIP.String()),
				)},
			)
		}

		if !m.EOps.BootloaderEnabled {
			// disable kexec, as this would effectively use the bootloader
			m.GenOps = slices.Concat(m.GenOps, []generate.Option{
				generate.WithSysctls(map[string]string{
					"kernel.kexec_load_disabled": "1",
				}),
			})
		}
	}

	switch {
	case m.Ops.ForceEndpoint != "":
		// using non-default endpoints, provision additional cert SANs and fix endpoint list
		m.Endpoints = []string{m.Ops.ForceEndpoint}
		m.GenOps = slices.Concat(m.GenOps, []generate.Option{generate.WithAdditionalSubjectAltNames(m.Endpoints)})
	case m.Ops.ForceInitNodeAsEndpoint:
		m.Endpoints = []string{m.IPs[0][0].String()}
	case m.Endpoints == nil:
		// use control plane nodes as endpoints, client-side load-balancing
		for i := range m.Ops.Controlplanes {
			m.Endpoints = slices.Concat(m.Endpoints, []string{m.IPs[0][i].String()})
		}
	}

	return nil
}

// AddExtraProvisionOpts implements ExtraOptionsProvider.
func (m *Qemu) AddExtraProvisionOpts() error {
	m.ProvisionOps = slices.Concat(m.ProvisionOps, []provision.Option{
		provision.WithBootlader(m.EOps.BootloaderEnabled),
		provision.WithUEFI(m.EOps.UefiEnabled),
		provision.WithTPM1_2(m.EOps.Tpm1_2Enabled),
		provision.WithTPM2(m.EOps.Tpm2Enabled),
		provision.WithDebugShell(m.EOps.DebugShellEnabled),
		provision.WithIOMMU(m.EOps.WithIOMMU),
		provision.WithExtraUEFISearchPaths(m.EOps.ExtraUEFISearchPaths),
		provision.WithTargetArch(m.EOps.TargetArch),
		provision.WithSiderolinkAgent(m.EOps.WithSiderolinkAgent.IsEnabled()),
	})

	externalKubernetesEndpoint := m.Provisioner.GetExternalKubernetesControlPlaneEndpoint(m.ClusterRequest.Network, m.Ops.ControlPlanePort)

	if m.EOps.UseVIP {
		externalKubernetesEndpoint = "https://" + nethelpers.JoinHostPort(m.VIP.String(), m.Ops.Controlplanes)
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

	if err := m.addDiskEncriptionPatches(); err != nil {
		return err
	}

	m.ConfigBundleOps = slices.Concat(m.ConfigBundleOps, []bundle.Option{
		bundle.WithPatch(m.SideroLinkBuilder.ConfigPatches(m.EOps.WithSiderolinkAgent.IsTunnel())),
	})

	return nil
}

// ModifyClusterRequest implements ExtraOptionsProvider.
func (m *Qemu) ModifyClusterRequest() error {
	nameserverIPs, err := getNameserverIPs(m.EOps.Nameservers)
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
	m.ClusterRequest.Network.Jitter = m.EOps.Jjitter
	m.ClusterRequest.Network.Latency = m.EOps.Latency
	m.ClusterRequest.Network.PacketLoss = m.EOps.PacketLoss
	m.ClusterRequest.Network.PacketReorder = m.EOps.PacketReorder
	m.ClusterRequest.Network.PacketCorrupt = m.EOps.PacketCorrupt
	m.ClusterRequest.Network.Bandwidth = m.EOps.Bandwidth

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
		if m.EOps.Jjitter != 0 || m.EOps.Latency != 0 || m.EOps.PacketLoss != 0 || m.EOps.PacketReorder != 0 || m.EOps.PacketCorrupt != 0 || m.EOps.Bandwidth != 0 {
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
		return fmt.Errorf("unknown config injection method %q", configInjectionMethod)
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

func (m *Qemu) addDiskEncriptionPatches() error {
	var diskEncryptionPatches []configpatcher.Patch

	if m.EOps.EncryptStatePartition || m.EOps.EncryptEphemeralPartition {
		keys, err := m.getEncryptionKeys(m.EOps.DiskEncryptionKeyTypes)
		if err != nil {
			return err
		}

		if !m.VersionContract.VolumeConfigEncryptionSupported() {
			// legacy v1alpha1 flow to support booting old Talos versions
			patch, err := m.getLegacyDiskEncriptionPatch(keys)
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

				patch, err := m.getDiskEncriptionPatch(spec, keys)
				if err != nil {
					return err
				}

				diskEncryptionPatches = append(diskEncryptionPatches, patch)
			}
		}
	}

	m.ConfigBundleOps = slices.Concat(m.ConfigBundleOps,
		[]bundle.Option{bundle.WithPatch(diskEncryptionPatches)},
	)

	return nil
}

func (*Qemu) getDiskEncriptionPatch(spec struct {
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
			blockCfg.EncryptionSpec.EncryptionKeys[idx].KeyLockToSTATE = pointer.To(true)
		}
	}

	ctr, err := container.New(blockCfg)
	if err != nil {
		return nil, fmt.Errorf("error creating container for %q volume: %w", spec.label, err)
	}

	patch := configpatcher.NewStrategicMergePatch(ctr)

	return patch, nil
}

func (m *Qemu) getLegacyDiskEncriptionPatch(keys []*v1alpha1.EncryptionKey) (configpatcher.Patch, error) {
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
	workerExtraDisks := []*provision.Disk{}
	primaryDisks := []*provision.Disk{
		{
			Size:            m.EOps.Disks.Requests()[0].Size.Bytes(),
			SkipPreallocate: !m.EOps.PreallocateDisks,
			Driver:          m.EOps.Disks.Requests()[0].Driver,
			BlockSize:       m.EOps.DiskBlockSize,
		},
	}
	// get worker extra disks
	for _, d := range m.EOps.Disks.Requests()[1:] {
		workerExtraDisks = append(workerExtraDisks, &provision.Disk{
			Size:            d.Size.Bytes(),
			SkipPreallocate: !m.EOps.PreallocateDisks,
			Driver:          d.Driver,
			BlockSize:       m.EOps.DiskBlockSize,
		})
	}

	m.ForEachNode(func(i int, node *provision.NodeRequest) {
		node.Disks = slices.Concat(node.Disks, primaryDisks)
		if node.Type == machine.TypeWorker {
			node.Disks = slices.Concat(node.Disks, workerExtraDisks)
		}
	})

	return m.initExtraDisks()
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
				ProvisioningMaxSize: block.MustByteSize(volumeSize),
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
				keyTPM.TPMCheckSecurebootStatusOnEnroll = pointer.To(true)
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
			r.KeyKMS = pointer.To(block.EncryptionKeyKMS(*k.KeyKMS))
		}

		if k.KeyTPM != nil {
			encryptionKeyTPM := block.EncryptionKeyTPM{
				TPMCheckSecurebootStatusOnEnroll: k.KeyTPM.TPMCheckSecurebootStatusOnEnroll,
			}

			r.KeyTPM = pointer.To(encryptionKeyTPM)
		}

		if k.KeyNodeID != nil {
			r.KeyNodeID = pointer.To(block.EncryptionKeyNodeID(*k.KeyNodeID))
		}

		if k.KeyStatic != nil {
			r.KeyStatic = pointer.To(block.EncryptionKeyStatic(*k.KeyStatic))
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
		})

	m.ConfigBundleOps = slices.Concat(m.ConfigBundleOps, []bundle.Option{bundle.WithPatch([]configpatcher.Patch{configpatcher.NewStrategicMergePatch(cfg)})})
}

func getNameserverIPs(nameservers []string) ([]netip.Addr, error) {
	nameserverIPs := make([]netip.Addr, len(nameservers))

	for i := range nameserverIPs {
		ip, err := netip.ParseAddr(nameservers[i])
		if err != nil {
			return nil, fmt.Errorf("failed parsing nameserver IP %q: %w", nameservers[i], err)
		}

		nameserverIPs[i] = ip
	}

	return nameserverIPs, nil
}
