// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package create

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/go-getter/v2"
	"github.com/klauspost/compress/zstd"
	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/gen/maps"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-blockdevice/v2/encryption"
	"github.com/siderolabs/go-kubeconfig"
	"github.com/siderolabs/go-pointer"
	"github.com/siderolabs/go-procfs/procfs"
	sideronet "github.com/siderolabs/net"
	"k8s.io/client-go/tools/clientcmd"

	clustercmd "github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster"
	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/internal/firewallpatch"
	"github.com/siderolabs/talos/cmd/talosctl/pkg/mgmt/helpers"
	"github.com/siderolabs/talos/pkg/cluster/check"
	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	clientconfig "github.com/siderolabs/talos/pkg/machinery/client/config"
	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/bundle"
	configbase "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/configpatcher"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/generate"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/types/block"
	"github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
	"github.com/siderolabs/talos/pkg/machinery/config/types/security"
	"github.com/siderolabs/talos/pkg/machinery/config/types/siderolink"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	blockres "github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/provision"
	"github.com/siderolabs/talos/pkg/provision/access"
	"github.com/siderolabs/talos/pkg/provision/providers"
)

const (
	// gatewayOffset is the offset from the network address of the IP address of the network gateway.
	gatewayOffset = 1

	// nodesOffset is the offset from the network address of the beginning of the IP addresses to be used for nodes.
	nodesOffset = 2

	// vipOffset is the offset from the network address of the CIDR to use for allocating the Virtual (shared) IP address, if enabled.
	vipOffset = 50
)

// downloadBootAssets downloads the boot assets in the given qemuOps if they are URLs, and replaces their URL paths with the downloaded paths on the filesystem.
//
// As it modifies the qemuOps struct, it needs to be passed by reference.
//
//nolint:gocyclo
func downloadBootAssets(ctx context.Context, qOps *qemuOps) error {
	// download & cache images if provides as URLs
	for _, downloadableImage := range []struct {
		path           *string
		disableArchive bool
	}{
		{
			path: &qOps.nodeVmlinuzPath,
		},
		{
			path:           &qOps.nodeInitramfsPath,
			disableArchive: true,
		},
		{
			path: &qOps.nodeISOPath,
		},
		{
			path: &qOps.nodeUSBPath,
		},
		{
			path: &qOps.nodeUKIPath,
		},
		{
			path: &qOps.nodeDiskImagePath,
			// we disable extracting the compressed image since we handle zstd for disk images
			disableArchive: true,
		},
	} {
		if *downloadableImage.path == "" {
			continue
		}

		u, err := url.Parse(*downloadableImage.path)
		if err != nil || !(u.Scheme == "http" || u.Scheme == "https") {
			// not a URL
			continue
		}

		defaultStateDir, err := clientconfig.GetTalosDirectory()
		if err != nil {
			return err
		}

		cacheDir := filepath.Join(defaultStateDir, "cache")

		if err = os.MkdirAll(cacheDir, 0o755); err != nil {
			return err
		}

		destPath := strings.ReplaceAll(
			strings.ReplaceAll(u.String(), "/", "-"),
			":", "-")

		_, err = os.Stat(filepath.Join(cacheDir, destPath))
		if err == nil {
			*downloadableImage.path = filepath.Join(cacheDir, destPath)

			// already cached
			continue
		}

		fmt.Fprintf(os.Stderr, "downloading asset from %q to %q\n", u.String(), filepath.Join(cacheDir, destPath))

		client := getter.Client{
			Getters: []getter.Getter{
				&getter.HttpGetter{
					HeadFirstTimeout: 30 * time.Minute,
					ReadTimeout:      30 * time.Minute,
				},
			},
		}

		if downloadableImage.disableArchive {
			q := u.Query()

			q.Set("archive", "false")

			u.RawQuery = q.Encode()
		}

		_, err = client.Get(ctx, &getter.Request{
			Src:     u.String(),
			Dst:     filepath.Join(cacheDir, destPath),
			GetMode: getter.ModeFile,
		})
		if err != nil {
			// clean up the destination on failure
			os.Remove(filepath.Join(cacheDir, destPath)) //nolint:errcheck

			return err
		}

		*downloadableImage.path = filepath.Join(cacheDir, destPath)
	}

	return nil
}

func getEncryptionKeys(cidr4 netip.Prefix, versionContract *config.VersionContract, provisionOptions *[]provision.Option, diskEncryptionKeyTypes []string) (
	[]*v1alpha1.EncryptionKey, error,
) {
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
			ip, err := sideronet.NthIPInNetwork(cidr4, 1)
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

			*provisionOptions = append(*provisionOptions, provision.WithKMS(nethelpers.JoinHostPort("0.0.0.0", port)))
		case "tpm":
			keyTPM := &v1alpha1.EncryptionKeyTPM{}

			if versionContract.SecureBootEnrollEnforcementSupported() {
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

//nolint:gocyclo,cyclop
func create(ctx context.Context, ops createOps) error {
	rootOps := ops.common.rootOps
	// common options
	cOps := ops.common
	// qemu options
	qOps := ops.qemu
	// docker options
	dockerOps := ops.docker

	if err := downloadBootAssets(ctx, &qOps); err != nil {
		return err
	}

	if cOps.controlplanes < 1 {
		return errors.New("number of controlplanes can't be less than 1")
	}

	controlPlaneNanoCPUs, err := parseCPUShare(cOps.controlPlaneCpus)
	if err != nil {
		return fmt.Errorf("error parsing --cpus: %s", err)
	}

	workerNanoCPUs, err := parseCPUShare(cOps.workersCpus)
	if err != nil {
		return fmt.Errorf("error parsing --cpus-workers: %s", err)
	}

	controlPlaneMemory := int64(cOps.controlPlaneMemory) * 1024 * 1024
	workerMemory := int64(cOps.workersMemory) * 1024 * 1024

	// Validate CIDR range and allocate IPs
	fmt.Fprintln(os.Stderr, "validating CIDR and reserving IPs")

	cidr4, err := netip.ParsePrefix(cOps.networkCIDR)
	if err != nil {
		return fmt.Errorf("error validating cidr block: %w", err)
	}

	if !cidr4.Addr().Is4() {
		return errors.New("--cidr is expected to be IPV4 CIDR")
	}

	// use ULA IPv6 network fd00::/8, add 'TAL' in hex to build /32 network, add IPv4 CIDR to build /64 unique network
	cidr6, err := netip.ParsePrefix(
		fmt.Sprintf(
			"fd74:616c:%02x%02x:%02x%02x::/64",
			cidr4.Addr().As4()[0], cidr4.Addr().As4()[1], cidr4.Addr().As4()[2], cidr4.Addr().As4()[3],
		),
	)
	if err != nil {
		return fmt.Errorf("error validating cidr IPv6 block: %w", err)
	}

	var cidrs []netip.Prefix

	if cOps.networkIPv4 {
		cidrs = append(cidrs, cidr4)
	}

	if cOps.networkIPv6 {
		cidrs = append(cidrs, cidr6)
	}

	if len(cidrs) == 0 {
		return errors.New("neither IPv4 nor IPv6 network was enabled")
	}

	// Gateway addr at 1st IP in range, ex. 192.168.0.1
	gatewayIPs := make([]netip.Addr, len(cidrs))

	for j := range gatewayIPs {
		gatewayIPs[j], err = sideronet.NthIPInNetwork(cidrs[j], gatewayOffset)
		if err != nil {
			return err
		}
	}

	// Set starting ip at 2nd ip in range, ex: 192.168.0.2
	ips := make([][]netip.Addr, len(cidrs))

	for j := range cidrs {
		ips[j] = make([]netip.Addr, cOps.controlplanes+cOps.workers)

		for i := range ips[j] {
			ips[j][i], err = sideronet.NthIPInNetwork(cidrs[j], nodesOffset+i)
			if err != nil {
				return err
			}
		}
	}

	noMasqueradeCIDRs := make([]netip.Prefix, 0, len(qOps.networkNoMasqueradeCIDRs))

	for _, cidr := range qOps.networkNoMasqueradeCIDRs {
		var parsedCIDR netip.Prefix

		parsedCIDR, err = netip.ParsePrefix(cidr)
		if err != nil {
			return fmt.Errorf("error parsing non-masquerade CIDR %q: %w", cidr, err)
		}

		noMasqueradeCIDRs = append(noMasqueradeCIDRs, parsedCIDR)
	}

	// Parse nameservers
	nameserverIPs := make([]netip.Addr, len(qOps.nameservers))

	for i := range nameserverIPs {
		nameserverIPs[i], err = netip.ParseAddr(qOps.nameservers[i])
		if err != nil {
			return fmt.Errorf("failed parsing nameserver IP %q: %w", qOps.nameservers[i], err)
		}
	}

	// Virtual (shared) IP at the vipOffset IP in range, ex. 192.168.0.50
	var vip netip.Addr

	if qOps.useVIP {
		vip, err = sideronet.NthIPInNetwork(cidrs[0], vipOffset)
		if err != nil {
			return err
		}
	}

	// Validate network chaos flags
	if !qOps.networkChaos {
		if qOps.jitter != 0 || qOps.latency != 0 || qOps.packetLoss != 0 || qOps.packetReorder != 0 || qOps.packetCorrupt != 0 || qOps.bandwidth != 0 {
			return errors.New("network chaos flags can only be used with --with-network-chaos")
		}
	}

	provisioner, err := providers.Factory(ctx, rootOps.ProvisionerName)
	if err != nil {
		return err
	}

	if rootOps.ProvisionerName == providers.DockerProviderName && !qOps.bootloaderEnabled {
		return errors.New("docker provisioner requires bootloader to be enabled")
	}

	defer provisioner.Close() //nolint:errcheck

	// Craft cluster and node requests
	request := provision.ClusterRequest{
		Name: rootOps.ClusterName,

		Network: provision.NetworkRequest{
			Name:              rootOps.ClusterName,
			CIDRs:             cidrs,
			NoMasqueradeCIDRs: noMasqueradeCIDRs,
			GatewayAddrs:      gatewayIPs,
			MTU:               cOps.networkMTU,
			Nameservers:       nameserverIPs,
			LoadBalancerPorts: []int{cOps.controlPlanePort},
			CNI: provision.CNIConfig{
				BinPath:  qOps.cniBinPath,
				ConfDir:  qOps.cniConfDir,
				CacheDir: qOps.cniCacheDir,

				BundleURL: qOps.cniBundleURL,
			},
			DHCPSkipHostname:  qOps.dhcpSkipHostname,
			DockerDisableIPv6: dockerOps.dockerDisableIPv6,
			NetworkChaos:      qOps.networkChaos,
			Jitter:            qOps.jitter,
			Latency:           qOps.latency,
			PacketLoss:        qOps.packetLoss,
			PacketReorder:     qOps.packetReorder,
			PacketCorrupt:     qOps.packetCorrupt,
			Bandwidth:         qOps.bandwidth,
		},

		Image:          dockerOps.nodeImage,
		KernelPath:     qOps.nodeVmlinuzPath,
		InitramfsPath:  qOps.nodeInitramfsPath,
		ISOPath:        qOps.nodeISOPath,
		USBPath:        qOps.nodeUSBPath,
		UKIPath:        qOps.nodeUKIPath,
		IPXEBootScript: qOps.nodeIPXEBootScript,
		DiskImagePath:  qOps.nodeDiskImagePath,

		SelfExecutable: os.Args[0],
		StateDirectory: rootOps.StateDir,
	}

	provisionOptions := []provision.Option{
		provision.WithDockerPortsHostIP(dockerOps.dockerHostIP),
		provision.WithBootlader(qOps.bootloaderEnabled),
		provision.WithUEFI(qOps.uefiEnabled),
		provision.WithTPM1_2(qOps.tpm1_2Enabled),
		provision.WithTPM2(qOps.tpm2Enabled),
		provision.WithDebugShell(qOps.debugShellEnabled),
		provision.WithIOMMU(qOps.withIOMMU),
		provision.WithExtraUEFISearchPaths(qOps.extraUEFISearchPaths),
		provision.WithTargetArch(qOps.targetArch),
		provision.WithSiderolinkAgent(qOps.withSiderolinkAgent.IsEnabled()),
	}

	var configBundleOpts []bundle.Option

	if qOps.debugShellEnabled {
		if rootOps.ProvisionerName != "qemu" {
			return errors.New("debug shell only supported with qemu provisioner")
		}
	}

	if dockerOps.ports != "" {
		if rootOps.ProvisionerName != providers.DockerProviderName {
			return errors.New("exposed-ports flag only supported with docker provisioner")
		}

		portList := strings.Split(dockerOps.ports, ",")
		provisionOptions = append(provisionOptions, provision.WithDockerPorts(portList))
	}

	// should have at least a single primary disk
	disks := []*provision.Disk{
		{
			Size:            uint64(qOps.clusterDiskSize) * 1024 * 1024,
			SkipPreallocate: !qOps.clusterDiskPreallocate,
			Driver:          "virtio",
			BlockSize:       qOps.diskBlockSize,
		},
	}

	if cOps.inputDir != "" {
		configBundleOpts = append(configBundleOpts, bundle.WithExistingConfigs(cOps.inputDir))
	} else {
		genOptions := []generate.Option{
			generate.WithInstallImage(qOps.nodeInstallImage),
			generate.WithDebug(cOps.configDebug),
			generate.WithDNSDomain(cOps.dnsDomain),
			generate.WithClusterDiscovery(cOps.enableClusterDiscovery),
		}

		for _, registryMirror := range cOps.registryMirrors {
			left, right, ok := strings.Cut(registryMirror, "=")
			if !ok {
				return fmt.Errorf("invalid registry mirror spec: %q", registryMirror)
			}

			genOptions = append(genOptions, generate.WithRegistryMirror(left, right))
		}

		for _, registryHost := range cOps.registryInsecure {
			genOptions = append(genOptions, generate.WithRegistryInsecureSkipVerify(registryHost))
		}

		genOptions = append(genOptions, provisioner.GenOptions(request.Network)...)

		if cOps.customCNIUrl != "" {
			genOptions = append(genOptions, generate.WithClusterCNIConfig(&v1alpha1.CNIConfig{
				CNIName: constants.CustomCNI,
				CNIUrls: []string{cOps.customCNIUrl},
			}))
		}

		if cOps.talosVersion == "" {
			if rootOps.ProvisionerName == providers.DockerProviderName {
				parts := strings.Split(dockerOps.nodeImage, ":")

				cOps.talosVersion = parts[len(parts)-1]
			} else {
				parts := strings.Split(qOps.nodeInstallImage, ":")

				cOps.talosVersion = parts[len(parts)-1]
			}
		}

		var versionContract *config.VersionContract

		if cOps.talosVersion != "latest" {
			versionContract, err = config.ParseContractFromVersion(cOps.talosVersion)
			if err != nil {
				return fmt.Errorf("error parsing Talos version %q: %w", cOps.talosVersion, err)
			}

			genOptions = append(genOptions, generate.WithVersionContract(versionContract))
		}

		extraDisks, userVolumePatches, err := getDisks(provisioner, cidr4, versionContract, &provisionOptions, qOps)
		if err != nil {
			return err
		}

		disks = slices.Concat(disks, extraDisks)

		if qOps.encryptStatePartition || qOps.encryptEphemeralPartition {
			diskEncryptionConfig := &v1alpha1.SystemDiskEncryptionConfig{}

			keys, err := getEncryptionKeys(cidr4, versionContract, &provisionOptions, qOps.diskEncryptionKeyTypes)
			if err != nil {
				return err
			}

			if qOps.encryptStatePartition {
				diskEncryptionConfig.StatePartition = &v1alpha1.EncryptionConfig{
					EncryptionProvider: encryption.LUKS2,
					EncryptionKeys:     keys,
				}
			}

			if qOps.encryptEphemeralPartition {
				diskEncryptionConfig.EphemeralPartition = &v1alpha1.EncryptionConfig{
					EncryptionProvider: encryption.LUKS2,
					EncryptionKeys:     keys,
				}
			}

			genOptions = append(genOptions, generate.WithSystemDiskEncryption(diskEncryptionConfig))
		}

		if qOps.useVIP {
			genOptions = append(genOptions,
				generate.WithNetworkOptions(
					v1alpha1.WithNetworkInterfaceVirtualIP(provisioner.GetFirstInterface(), vip.String()),
				),
			)
		}

		if cOps.enableKubeSpan {
			genOptions = append(genOptions,
				generate.WithNetworkOptions(
					v1alpha1.WithKubeSpan(),
				),
			)
		}

		if !qOps.bootloaderEnabled {
			// disable kexec, as this would effectively use the bootloader
			genOptions = append(genOptions,
				generate.WithSysctls(map[string]string{
					"kernel.kexec_load_disabled": "1",
				}),
			)
		}

		if cOps.controlPlanePort != constants.DefaultControlPlanePort {
			genOptions = append(genOptions,
				generate.WithLocalAPIServerPort(cOps.controlPlanePort),
			)
		}

		if cOps.kubePrismPort != constants.DefaultKubePrismPort {
			genOptions = append(genOptions,
				generate.WithKubePrismPort(cOps.kubePrismPort),
			)
		}

		externalKubernetesEndpoint := provisioner.GetExternalKubernetesControlPlaneEndpoint(request.Network, cOps.controlPlanePort)

		if qOps.useVIP {
			externalKubernetesEndpoint = "https://" + nethelpers.JoinHostPort(vip.String(), cOps.controlPlanePort)
		}

		provisionOptions = append(provisionOptions, provision.WithKubernetesEndpoint(externalKubernetesEndpoint))

		endpointList := provisioner.GetTalosAPIEndpoints(request.Network)

		switch {
		case cOps.forceEndpoint != "":
			// using non-default endpoints, provision additional cert SANs and fix endpoint list
			endpointList = []string{cOps.forceEndpoint}
			genOptions = append(genOptions, generate.WithAdditionalSubjectAltNames(endpointList))
		case cOps.forceInitNodeAsEndpoint:
			endpointList = []string{ips[0][0].String()}
		case len(endpointList) > 0:
			for _, endpointHostPort := range endpointList {
				endpointHost, _, err := net.SplitHostPort(endpointHostPort)
				if err != nil {
					endpointHost = endpointHostPort
				}

				genOptions = append(genOptions, generate.WithAdditionalSubjectAltNames([]string{endpointHost}))
			}
		case endpointList == nil:
			// use control plane nodes as endpoints, client-side load-balancing
			for i := range cOps.controlplanes {
				endpointList = append(endpointList, ips[0][i].String())
			}
		}

		inClusterEndpoint := provisioner.GetInClusterKubernetesControlPlaneEndpoint(request.Network, cOps.controlPlanePort)

		if qOps.useVIP {
			inClusterEndpoint = "https://" + nethelpers.JoinHostPort(vip.String(), cOps.controlPlanePort)
		}

		genOptions = append(genOptions, generate.WithEndpointList(endpointList))
		configBundleOpts = append(configBundleOpts,
			bundle.WithInputOptions(
				&bundle.InputOptions{
					ClusterName: rootOps.ClusterName,
					Endpoint:    inClusterEndpoint,
					KubeVersion: strings.TrimPrefix(cOps.kubernetesVersion, "v"),
					GenOptions:  genOptions,
				}),
			bundle.WithPatch(userVolumePatches),
		)
	}

	addConfigPatch := func(configPatches []string, configOpt func([]configpatcher.Patch) bundle.Option) error {
		var patches []configpatcher.Patch

		patches, err = configpatcher.LoadPatches(configPatches)
		if err != nil {
			return fmt.Errorf("error parsing config JSON patch: %w", err)
		}

		configBundleOpts = append(configBundleOpts, configOpt(patches))

		return nil
	}

	if err = addConfigPatch(cOps.configPatch, bundle.WithPatch); err != nil {
		return err
	}

	if err = addConfigPatch(cOps.configPatchControlPlane, bundle.WithPatchControlPlane); err != nil {
		return err
	}

	if err = addConfigPatch(cOps.configPatchWorker, bundle.WithPatchWorker); err != nil {
		return err
	}

	if qOps.withFirewall != "" {
		var defaultAction nethelpers.DefaultAction

		defaultAction, err = nethelpers.DefaultActionString(qOps.withFirewall)
		if err != nil {
			return err
		}

		var controlplaneIPs []netip.Addr

		for i := range ips {
			controlplaneIPs = append(controlplaneIPs, ips[i][:cOps.controlplanes]...)
		}

		configBundleOpts = append(configBundleOpts,
			bundle.WithPatchControlPlane([]configpatcher.Patch{firewallpatch.ControlPlane(defaultAction, cidrs, gatewayIPs, controlplaneIPs)}),
			bundle.WithPatchWorker([]configpatcher.Patch{firewallpatch.Worker(defaultAction, cidrs, gatewayIPs)}),
		)
	}

	var slb *siderolinkBuilder

	if qOps.withSiderolinkAgent.IsEnabled() {
		slb, err = newSiderolinkBuilder(gatewayIPs[0].String(), qOps.withSiderolinkAgent.IsTLS())
		if err != nil {
			return err
		}
	}

	configBundleOpts = append(configBundleOpts, bundle.WithPatch(slb.ConfigPatches(qOps.withSiderolinkAgent.IsTunnel())))

	if cOps.withJSONLogs {
		const port = 4003

		provisionOptions = append(provisionOptions, provision.WithJSONLogs(nethelpers.JoinHostPort(gatewayIPs[0].String(), port)))

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
										Host:   nethelpers.JoinHostPort(gatewayIPs[0].String(), port),
									},
								},
								LoggingFormat: "json_lines",
							},
						},
					},
				},
			})
		configBundleOpts = append(configBundleOpts, bundle.WithPatch([]configpatcher.Patch{configpatcher.NewStrategicMergePatch(cfg)}))
	}

	configBundle, err := bundle.NewBundle(configBundleOpts...)
	if err != nil {
		return err
	}

	bundleTalosconfig := configBundle.TalosConfig()
	if bundleTalosconfig == nil {
		if cOps.clusterWait {
			return errors.New("no talosconfig in the config bundle: cannot wait for cluster")
		}

		if cOps.applyConfigEnabled {
			return errors.New("no talosconfig in the config bundle: cannot apply config")
		}
	}

	if cOps.skipInjectingConfig {
		types := []machine.Type{machine.TypeControlPlane, machine.TypeWorker}

		if cOps.withInitNode {
			types = slices.Insert(types, 0, machine.TypeInit)
		}

		if err = configBundle.Write(".", encoder.CommentsAll, types...); err != nil {
			return err
		}
	}

	// Wireguard configuration.
	var wireguardConfigBundle *helpers.WireguardConfigBundle
	if cOps.wireguardCIDR != "" {
		wireguardConfigBundle, err = helpers.NewWireguardConfigBundle(ips[0], cOps.wireguardCIDR, 51111, cOps.controlplanes)
		if err != nil {
			return err
		}
	}

	var extraKernelArgs *procfs.Cmdline

	if qOps.extraBootKernelArgs != "" || qOps.withSiderolinkAgent.IsEnabled() {
		extraKernelArgs = procfs.NewCmdline(qOps.extraBootKernelArgs)
	}

	err = slb.SetKernelArgs(extraKernelArgs, qOps.withSiderolinkAgent.IsTunnel())
	if err != nil {
		return err
	}

	// Add talosconfig to provision options, so we'll have it to parse there
	provisionOptions = append(provisionOptions, provision.WithTalosConfig(configBundle.TalosConfig()))

	var configInjectionMethod provision.ConfigInjectionMethod

	switch qOps.configInjectionMethodFlagVal {
	case "", "default", "http":
		configInjectionMethod = provision.ConfigInjectionMethodHTTP
	case "metal-iso":
		configInjectionMethod = provision.ConfigInjectionMethodMetalISO
	default:
		return fmt.Errorf("unknown config injection method %q", configInjectionMethod)
	}

	// Create the controlplane nodes.
	for i := range cOps.controlplanes {
		var cfg config.Provider

		nodeIPs := make([]netip.Addr, len(cidrs))
		for j := range nodeIPs {
			nodeIPs[j] = ips[j][i]
		}

		nodeUUID := uuid.New()

		err = slb.DefineIPv6ForUUID(nodeUUID)
		if err != nil {
			return err
		}

		nodeReq := provision.NodeRequest{
			Name:                  nodeName(rootOps.ClusterName, "controlplane", i+1, nodeUUID, qOps),
			Type:                  machine.TypeControlPlane,
			Quirks:                quirks.New(cOps.talosVersion),
			IPs:                   nodeIPs,
			Memory:                controlPlaneMemory,
			NanoCPUs:              controlPlaneNanoCPUs,
			Disks:                 disks,
			Mounts:                dockerOps.mountOpts.Value(),
			SkipInjectingConfig:   cOps.skipInjectingConfig,
			ConfigInjectionMethod: configInjectionMethod,
			BadRTC:                qOps.badRTC,
			ExtraKernelArgs:       extraKernelArgs,
			UUID:                  pointer.To(nodeUUID),
		}

		if cOps.withInitNode && i == 0 {
			cfg = configBundle.Init()
			nodeReq.Type = machine.TypeInit
		} else {
			cfg = configBundle.ControlPlane()
		}

		if wireguardConfigBundle != nil {
			cfg, err = wireguardConfigBundle.PatchConfig(nodeIPs[0], cfg)
			if err != nil {
				return err
			}
		}

		nodeReq.Config = cfg

		request.Nodes = append(request.Nodes, nodeReq)
	}

	// append extra disks
	for i := range qOps.extraDisks {
		driver := "ide"

		// ide driver is not supported on arm64
		if qOps.targetArch == "arm64" {
			driver = "virtio"
		}

		if i < len(qOps.extraDisksDrivers) {
			driver = qOps.extraDisksDrivers[i]
		}

		disks = append(disks, &provision.Disk{
			Size:            uint64(qOps.extraDiskSize) * 1024 * 1024,
			SkipPreallocate: !qOps.clusterDiskPreallocate,
			Driver:          driver,
			BlockSize:       qOps.diskBlockSize,
		})
	}

	for i := 1; i <= cOps.workers; i++ {
		cfg := configBundle.Worker()

		nodeIPs := make([]netip.Addr, len(cidrs))
		for j := range nodeIPs {
			nodeIPs[j] = ips[j][cOps.controlplanes+i-1]
		}

		if wireguardConfigBundle != nil {
			cfg, err = wireguardConfigBundle.PatchConfig(nodeIPs[0], cfg)
			if err != nil {
				return err
			}
		}

		nodeUUID := uuid.New()

		err = slb.DefineIPv6ForUUID(nodeUUID)
		if err != nil {
			return err
		}

		request.Nodes = append(request.Nodes,
			provision.NodeRequest{
				Name:                  nodeName(rootOps.ClusterName, "worker", i, nodeUUID, qOps),
				Type:                  machine.TypeWorker,
				IPs:                   nodeIPs,
				Quirks:                quirks.New(cOps.talosVersion),
				Memory:                workerMemory,
				NanoCPUs:              workerNanoCPUs,
				Disks:                 disks,
				Mounts:                dockerOps.mountOpts.Value(),
				Config:                cfg,
				ConfigInjectionMethod: configInjectionMethod,
				SkipInjectingConfig:   cOps.skipInjectingConfig,
				BadRTC:                qOps.badRTC,
				ExtraKernelArgs:       extraKernelArgs,
				UUID:                  pointer.To(nodeUUID),
			})
	}

	request.SiderolinkRequest = slb.SiderolinkRequest()

	cluster, err := provisioner.Create(ctx, request, provisionOptions...)
	if err != nil {
		return err
	}

	if qOps.debugShellEnabled {
		fmt.Println("You can now connect to debug shell on any node using these commands:")

		for _, node := range request.Nodes {
			talosDir, err := clientconfig.GetTalosDirectory()
			if err != nil {
				return nil
			}

			fmt.Printf("socat - UNIX-CONNECT:%s\n", filepath.Join(talosDir, "clusters", rootOps.ClusterName, node.Name+".serial"))
		}

		return nil
	}

	// No talosconfig in the bundle - skip the operations below
	if bundleTalosconfig == nil {
		return nil
	}

	// Create and save the talosctl configuration file.
	if err = saveConfig(bundleTalosconfig, cOps.talosconfig); err != nil {
		return err
	}

	clusterAccess := access.NewAdapter(cluster, provisionOptions...)
	defer clusterAccess.Close() //nolint:errcheck

	if cOps.applyConfigEnabled {
		err = clusterAccess.ApplyConfig(ctx, request.Nodes, request.SiderolinkRequest, os.Stdout)
		if err != nil {
			return err
		}
	}

	if err = postCreate(ctx, clusterAccess, cOps); err != nil {
		return err
	}

	return clustercmd.ShowCluster(cluster)
}

func nodeName(clusterName, role string, index int, uuid uuid.UUID, qOps qemuOps) string {
	if qOps.withUUIDHostnames {
		return fmt.Sprintf("machine-%s", uuid)
	}

	return fmt.Sprintf("%s-%s-%d", clusterName, role, index)
}

func postCreate(ctx context.Context, clusterAccess *access.Adapter, cOps commonOps) error {
	if !cOps.withInitNode {
		if err := clusterAccess.Bootstrap(ctx, os.Stdout); err != nil {
			return fmt.Errorf("bootstrap error: %w", err)
		}
	}

	if !cOps.clusterWait {
		return nil
	}

	// Run cluster readiness checks
	checkCtx, checkCtxCancel := context.WithTimeout(ctx, cOps.clusterWaitTimeout)
	defer checkCtxCancel()

	checks := check.DefaultClusterChecks()

	if cOps.skipK8sNodeReadinessCheck {
		checks = slices.Concat(check.PreBootSequenceChecks(), check.K8sComponentsReadinessChecks())
	}

	checks = append(checks, check.ExtraClusterChecks()...)

	if err := check.Wait(checkCtx, clusterAccess, checks, check.StderrReporter()); err != nil {
		return err
	}

	if !cOps.skipKubeconfig {
		if err := mergeKubeconfig(ctx, clusterAccess); err != nil {
			return err
		}
	}

	return nil
}

func saveConfig(talosConfigObj *clientconfig.Config, talosconfigPath string) (err error) {
	c, err := clientconfig.Open(talosconfigPath)
	if err != nil {
		return fmt.Errorf("error opening talos config: %w", err)
	}

	renames := c.Merge(talosConfigObj)
	for _, rename := range renames {
		fmt.Fprintf(os.Stderr, "renamed talosconfig context %s\n", rename.String())
	}

	return c.Save(talosconfigPath)
}

func mergeKubeconfig(ctx context.Context, clusterAccess *access.Adapter) error {
	kubeconfigPath, err := kubeconfig.SinglePath()
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "\nmerging kubeconfig into %q\n", kubeconfigPath)

	k8sconfig, err := clusterAccess.Kubeconfig(ctx)
	if err != nil {
		return fmt.Errorf("error fetching kubeconfig: %w", err)
	}

	kubeConfig, err := clientcmd.Load(k8sconfig)
	if err != nil {
		return fmt.Errorf("error parsing kubeconfig: %w", err)
	}

	if clusterAccess.ForceEndpoint != "" {
		for name := range kubeConfig.Clusters {
			kubeConfig.Clusters[name].Server = clusterAccess.ForceEndpoint
		}
	}

	_, err = os.Stat(kubeconfigPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}

		return clientcmd.WriteToFile(*kubeConfig, kubeconfigPath)
	}

	merger, err := kubeconfig.Load(kubeconfigPath)
	if err != nil {
		return fmt.Errorf("error loading existing kubeconfig: %w", err)
	}

	err = merger.Merge(kubeConfig, kubeconfig.MergeOptions{
		ActivateContext: true,
		OutputWriter:    os.Stdout,
		ConflictHandler: func(component kubeconfig.ConfigComponent, name string) (kubeconfig.ConflictDecision, error) {
			return kubeconfig.RenameDecision, nil
		},
	})
	if err != nil {
		return fmt.Errorf("error merging kubeconfig: %w", err)
	}

	return merger.Write(kubeconfigPath)
}

func parseCPUShare(cpus string) (int64, error) {
	cpu, ok := new(big.Rat).SetString(cpus)
	if !ok {
		return 0, fmt.Errorf("failed to parsing as a rational number: %s", cpus)
	}

	nano := cpu.Mul(cpu, big.NewRat(1e9, 1))
	if !nano.IsInt() {
		return 0, errors.New("value is too precise")
	}

	return nano.Num().Int64(), nil
}

//nolint:gocyclo
func getDisks(
	provisioner provision.Provisioner,
	cidr4 netip.Prefix,
	versionContract *config.VersionContract,
	provisionOptions *[]provision.Option,
	qOps qemuOps,
) ([]*provision.Disk, []configpatcher.Patch, error) {
	const GPTAlignment = 2 * 1024 * 1024 // 2 MB

	var (
		userVolumes    []*block.UserVolumeConfigV1Alpha1
		encryptionSpec block.EncryptionSpec
	)

	if qOps.encryptUserVolumes {
		encryptionSpec.EncryptionProvider = blockres.EncryptionProviderLUKS2

		keys, err := getEncryptionKeys(
			cidr4,
			versionContract,
			provisionOptions,
			qOps.diskEncryptionKeyTypes,
		)
		if err != nil {
			return nil, nil, err
		}

		encryptionSpec.EncryptionKeys = xslices.Map(keys, func(k *v1alpha1.EncryptionKey) block.EncryptionKey {
			r := block.EncryptionKey{
				KeySlot: k.KeySlot,
			}

			if k.KeyKMS != nil {
				r.KeyKMS = pointer.To(block.EncryptionKeyKMS(*k.KeyKMS))
			}

			if k.KeyTPM != nil {
				r.KeyTPM = pointer.To(block.EncryptionKeyTPM(*k.KeyTPM))
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

	disks := make([]*provision.Disk, 0, len(qOps.clusterUserVolumes))

	for diskID, disk := range qOps.clusterUserVolumes {
		var (
			volumes  = strings.Split(disk, ":")
			diskSize uint64
		)

		if len(volumes)%2 != 0 {
			return nil, nil, errors.New("failed to parse malformed volume definitions")
		}

		for j := 0; j < len(volumes); j += 2 {
			volumeName := volumes[j]
			volumeSize := volumes[j+1]

			userVolume := block.NewUserVolumeConfigV1Alpha1()
			userVolume.MetaName = volumeName
			userVolume.ProvisioningSpec = block.ProvisioningSpec{
				DiskSelectorSpec: block.DiskSelector{
					Match: cel.MustExpression(cel.ParseBooleanExpression(fmt.Sprintf("'%s' in disk.symlinks", provisioner.UserDiskName(diskID+1)), celenv.DiskLocator())),
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
			SkipPreallocate: !qOps.clusterDiskPreallocate,
			Driver:          "ide",
			BlockSize:       qOps.diskBlockSize,
		})
	}

	if len(userVolumes) > 0 {
		ctr, err := container.New(xslices.Map(userVolumes, func(u *block.UserVolumeConfigV1Alpha1) configbase.Document { return u })...)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create user volumes container: %w", err)
		}

		return disks, []configpatcher.Patch{configpatcher.NewStrategicMergePatch(ctr)}, err
	}

	return disks, nil, nil
}

func newSiderolinkBuilder(wgHost string, useTLS bool) (*siderolinkBuilder, error) {
	prefix, err := networkPrefix("")
	if err != nil {
		return nil, err
	}

	result := &siderolinkBuilder{
		wgHost:       wgHost,
		binds:        map[uuid.UUID]netip.Addr{},
		prefix:       prefix,
		nodeIPv6Addr: prefix.Addr().Next().String(),
	}

	if useTLS {
		ca, err := x509.NewSelfSignedCertificateAuthority(x509.ECDSA(true), x509.IPAddresses([]net.IP{net.ParseIP(wgHost)}))
		if err != nil {
			return nil, err
		}

		result.apiCert = ca.CrtPEM
		result.apiKey = ca.KeyPEM
	}

	var resultErr error

	for range 10 {
		for _, d := range []struct {
			field *int
			net   string
			what  string
		}{
			{&result.wgPort, "udp", "WireGuard"},
			{&result.apiPort, "tcp", "gRPC API"},
			{&result.sinkPort, "tcp", "Event Sink"},
			{&result.logPort, "tcp", "Log Receiver"},
		} {
			var err error

			*d.field, err = getDynamicPort(d.net)
			if err != nil {
				return nil, fmt.Errorf("failed to get dynamic port for %s: %w", d.what, err)
			}
		}

		resultErr = checkPortsDontOverlap(result.wgPort, result.apiPort, result.sinkPort, result.logPort)
		if resultErr == nil {
			break
		}
	}

	if resultErr != nil {
		return nil, fmt.Errorf("failed to get non-overlapping dynamic ports in 10 attempts: %w", resultErr)
	}

	return result, nil
}

type siderolinkBuilder struct {
	wgHost string

	binds        map[uuid.UUID]netip.Addr
	prefix       netip.Prefix
	nodeIPv6Addr string
	wgPort       int
	apiPort      int
	sinkPort     int
	logPort      int

	apiCert []byte
	apiKey  []byte
}

// DefineIPv6ForUUID defines an IPv6 address for a given UUID. It is safe to call this method on a nil pointer.
func (slb *siderolinkBuilder) DefineIPv6ForUUID(id uuid.UUID) error {
	if slb == nil {
		return nil
	}

	result, err := generateRandomNodeAddr(slb.prefix)
	if err != nil {
		return err
	}

	slb.binds[id] = result.Addr()

	return nil
}

// SiderolinkRequest returns a SiderolinkRequest based on the current state of the builder.
// It is safe to call this method on a nil pointer.
func (slb *siderolinkBuilder) SiderolinkRequest() provision.SiderolinkRequest {
	if slb == nil {
		return provision.SiderolinkRequest{}
	}

	return provision.SiderolinkRequest{
		WireguardEndpoint: net.JoinHostPort(slb.wgHost, strconv.Itoa(slb.wgPort)),
		APIEndpoint:       ":" + strconv.Itoa(slb.apiPort),
		APICertificate:    slb.apiCert,
		APIKey:            slb.apiKey,
		SinkEndpoint:      ":" + strconv.Itoa(slb.sinkPort),
		LogEndpoint:       ":" + strconv.Itoa(slb.logPort),
		SiderolinkBind: maps.ToSlice(slb.binds, func(k uuid.UUID, v netip.Addr) provision.SiderolinkBind {
			return provision.SiderolinkBind{
				UUID: k,
				Addr: v,
			}
		}),
	}
}

// ConfigPatches returns the config patches for the current builder.
func (slb *siderolinkBuilder) ConfigPatches(tunnel bool) []configpatcher.Patch {
	cfg := slb.ConfigDocument(tunnel)
	if cfg == nil {
		return nil
	}

	return []configpatcher.Patch{configpatcher.NewStrategicMergePatch(cfg)}
}

// ConfigDocument returns the config document for the current builder.
func (slb *siderolinkBuilder) ConfigDocument(tunnel bool) config.Provider {
	if slb == nil {
		return nil
	}

	scheme := "grpc://"

	if slb.apiCert != nil {
		scheme = "https://"
	}

	apiLink := scheme + net.JoinHostPort(slb.wgHost, strconv.Itoa(slb.apiPort)) + "?jointoken=foo"

	if tunnel {
		apiLink += "&grpc_tunnel=true"
	}

	apiURL, err := url.Parse(apiLink)
	if err != nil {
		panic(fmt.Sprintf("failed to parse API URL: %s", err))
	}

	sdlConfig := siderolink.NewConfigV1Alpha1()
	sdlConfig.APIUrlConfig.URL = apiURL

	eventsConfig := runtime.NewEventSinkV1Alpha1()
	eventsConfig.Endpoint = net.JoinHostPort(slb.nodeIPv6Addr, strconv.Itoa(slb.sinkPort))

	logURL, err := url.Parse("tcp://" + net.JoinHostPort(slb.nodeIPv6Addr, strconv.Itoa(slb.logPort)))
	if err != nil {
		panic(fmt.Sprintf("failed to parse log URL: %s", err))
	}

	logConfig := runtime.NewKmsgLogV1Alpha1()
	logConfig.MetaName = "siderolink"
	logConfig.KmsgLogURL.URL = logURL

	documents := []configbase.Document{
		sdlConfig,
		eventsConfig,
		logConfig,
	}

	if slb.apiCert != nil {
		trustedRootsConfig := security.NewTrustedRootsConfigV1Alpha1()
		trustedRootsConfig.MetaName = "siderolink-ca"
		trustedRootsConfig.Certificates = string(slb.apiCert)

		documents = append(documents, trustedRootsConfig)
	}

	ctr, err := container.New(documents...)
	if err != nil {
		panic(fmt.Sprintf("failed to create container for Siderolink config: %s", err))
	}

	return ctr
}

// SetKernelArgs sets the kernel arguments for the current builder. It is safe to call this method on a nil pointer.
func (slb *siderolinkBuilder) SetKernelArgs(extraKernelArgs *procfs.Cmdline, tunnel bool) error {
	switch {
	case slb == nil:
		return nil
	case extraKernelArgs.Get("siderolink.api") != nil,
		extraKernelArgs.Get("talos.events.sink") != nil,
		extraKernelArgs.Get("talos.logging.kernel") != nil:
		return errors.New("siderolink kernel arguments are already set, cannot run with --with-siderolink")
	default:
		marshaled, err := slb.ConfigDocument(tunnel).EncodeBytes(encoder.WithComments(encoder.CommentsDisabled))
		if err != nil {
			panic(fmt.Sprintf("failed to marshal trusted roots config: %s", err))
		}

		var buf bytes.Buffer

		zencoder, err := zstd.NewWriter(&buf)
		if err != nil {
			return fmt.Errorf("failed to create zstd encoder: %w", err)
		}

		_, err = zencoder.Write(marshaled)
		if err != nil {
			return fmt.Errorf("failed to write zstd data: %w", err)
		}

		if err = zencoder.Close(); err != nil {
			return fmt.Errorf("failed to close zstd encoder: %w", err)
		}

		extraKernelArgs.Append(constants.KernelParamConfigInline, base64.StdEncoding.EncodeToString(buf.Bytes()))

		return nil
	}
}

func getDynamicPort(network string) (int, error) {
	var (
		closeFn func() error
		addrFn  func() net.Addr
	)

	switch network {
	case "tcp", "tcp4", "tcp6":
		l, err := net.Listen(network, "127.0.0.1:0")
		if err != nil {
			return 0, err
		}

		addrFn, closeFn = l.Addr, l.Close
	case "udp", "udp4", "udp6":
		l, err := net.ListenPacket(network, "127.0.0.1:0")
		if err != nil {
			return 0, err
		}

		addrFn, closeFn = l.LocalAddr, l.Close
	default:
		return 0, fmt.Errorf("unsupported network: %s", network)
	}

	_, portStr, err := net.SplitHostPort(addrFn().String())
	if err != nil {
		return 0, handleCloseErr(err, closeFn())
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 0, err
	}

	return port, handleCloseErr(nil, closeFn())
}

func handleCloseErr(err error, closeErr error) error {
	switch {
	case err != nil && closeErr != nil:
		return fmt.Errorf("error: %w, close error: %w", err, closeErr)
	case err == nil && closeErr != nil:
		return closeErr
	case err != nil && closeErr == nil:
		return err
	default:
		return nil
	}
}

func checkPortsDontOverlap(ports ...int) error {
	slices.Sort(ports)

	if len(ports) != len(slices.Compact(ports)) {
		return errors.New("generated ports overlap")
	}

	return nil
}

type agentFlag uint8

func (a *agentFlag) String() string {
	switch *a {
	case 1:
		return "wireguard"
	case 2:
		return "grpc-tunnel"
	case 3:
		return "wireguard+tls"
	case 4:
		return "grpc-tunnel+tls"
	default:
		return "none"
	}
}

func (a *agentFlag) Set(s string) error {
	switch s {
	case "true", "wireguard":
		*a = 1
	case "tunnel":
		*a = 2
	case "wireguard+tls":
		*a = 3
	case "grpc-tunnel+tls":
		*a = 4
	default:
		return fmt.Errorf("unknown type: %s, possible values: 'true', 'wireguard' for the usual WG; 'tunnel' for WG over GRPC, add '+tls' to enable TLS for API", s)
	}

	return nil
}

func (a *agentFlag) Type() string    { return "agent" }
func (a *agentFlag) IsEnabled() bool { return *a != 0 }
func (a *agentFlag) IsTunnel() bool  { return *a == 2 || *a == 4 }
func (a *agentFlag) IsTLS() bool     { return *a == 3 || *a == 4 }
