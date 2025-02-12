// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

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
	stdruntime "runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/docker/cli/opts"
	"github.com/dustin/go-humanize"
	"github.com/google/uuid"
	"github.com/hashicorp/go-getter/v2"
	"github.com/klauspost/compress/zstd"
	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/gen/maps"
	"github.com/siderolabs/go-blockdevice/v2/encryption"
	"github.com/siderolabs/go-kubeconfig"
	"github.com/siderolabs/go-pointer"
	"github.com/siderolabs/go-procfs/procfs"
	sideronet "github.com/siderolabs/net"
	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/internal/firewallpatch"
	"github.com/siderolabs/talos/cmd/talosctl/pkg/mgmt/helpers"
	"github.com/siderolabs/talos/pkg/cli"
	"github.com/siderolabs/talos/pkg/cluster/check"
	"github.com/siderolabs/talos/pkg/images"
	clientconfig "github.com/siderolabs/talos/pkg/machinery/client/config"
	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/bundle"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/configpatcher"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/generate"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/types/security"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/version"
	"github.com/siderolabs/talos/pkg/provision"
	"github.com/siderolabs/talos/pkg/provision/access"
	"github.com/siderolabs/talos/pkg/provision/providers"
)

const (
	docker = "docker"

	// gatewayOffset is the offset from the network address of the IP address of the network gateway.
	gatewayOffset = 1

	// nodesOffset is the offset from the network address of the beginning of the IP addresses to be used for nodes.
	nodesOffset = 2

	// vipOffset is the offset from the network address of the CIDR to use for allocating the Virtual (shared) IP address, if enabled.
	vipOffset = 50

	inputDirFlag                 = "input-dir"
	networkIPv4Flag              = "ipv4"
	networkIPv6Flag              = "ipv6"
	networkMTUFlag               = "mtu"
	networkCIDRFlag              = "cidr"
	networkNoMasqueradeCIDRsFlag = "no-masquerade-cidrs"
	nameserversFlag              = "nameservers"
	clusterDiskPreallocateFlag   = "disk-preallocate"
	clusterDisksFlag             = "user-disk"
	clusterDiskSizeFlag          = "disk"
	useVIPFlag                   = "use-vip"
	bootloaderEnabledFlag        = "with-bootloader"
	controlPlanePortFlag         = "control-plane-port"
	firewallFlag                 = "with-firewall"
	tpm2EnabledFlag              = "with-tpm2"
	withDebugShellFlag           = "with-debug-shell"

	// The following flags are the gen options - the options that are only used in machine configuration (i.e., not during the qemu/docker provisioning).
	// They are not applicable when no machine configuration is generated, hence mutually exclusive with the --input-dir flag.

	nodeInstallImageFlag          = "install-image"
	configDebugFlag               = "with-debug"
	dnsDomainFlag                 = "dns-domain"
	withClusterDiscoveryFlag      = "with-cluster-discovery"
	registryMirrorFlag            = "registry-mirror"
	registryInsecureFlag          = "registry-insecure-skip-verify"
	customCNIUrlFlag              = "custom-cni-url"
	talosVersionFlag              = "talos-version"
	encryptStatePartitionFlag     = "encrypt-state"
	encryptEphemeralPartitionFlag = "encrypt-ephemeral"
	enableKubeSpanFlag            = "with-kubespan"
	forceEndpointFlag             = "endpoint"
	kubePrismFlag                 = "kubeprism-port"
	diskEncryptionKeyTypesFlag    = "disk-encryption-key-types"
)

var (
	talosconfig               string
	nodeImage                 string
	nodeInstallImage          string
	registryMirrors           []string
	registryInsecure          []string
	kubernetesVersion         string
	nodeVmlinuzPath           string
	nodeInitramfsPath         string
	nodeISOPath               string
	nodeUSBPath               string
	nodeDiskImagePath         string
	nodeIPXEBootScript        string
	applyConfigEnabled        bool
	bootloaderEnabled         bool
	uefiEnabled               bool
	tpm2Enabled               bool
	extraUEFISearchPaths      []string
	configDebug               bool
	networkCIDR               string
	networkNoMasqueradeCIDRs  []string
	networkMTU                int
	networkIPv4               bool
	networkIPv6               bool
	wireguardCIDR             string
	nameservers               []string
	dnsDomain                 string
	workers                   int
	controlplanes             int
	controlPlaneCpus          string
	workersCpus               string
	controlPlaneMemory        int
	workersMemory             int
	clusterDiskSize           int
	clusterDiskPreallocate    bool
	diskBlockSize             uint
	clusterDisks              []string
	extraDisks                int
	extraDiskSize             int
	extraDisksDrivers         []string
	targetArch                string
	clusterWait               bool
	clusterWaitTimeout        time.Duration
	forceInitNodeAsEndpoint   bool
	forceEndpoint             string
	inputDir                  string
	cniBinPath                []string
	cniConfDir                string
	cniCacheDir               string
	cniBundleURL              string
	ports                     string
	dockerHostIP              string
	withInitNode              bool
	customCNIUrl              string
	skipKubeconfig            bool
	skipInjectingConfig       bool
	talosVersion              string
	encryptStatePartition     bool
	encryptEphemeralPartition bool
	useVIP                    bool
	enableKubeSpan            bool
	enableClusterDiscovery    bool
	configPatch               []string
	configPatchControlPlane   []string
	configPatchWorker         []string
	badRTC                    bool
	extraBootKernelArgs       string
	dockerDisableIPv6         bool
	controlPlanePort          int
	kubePrismPort             int
	dhcpSkipHostname          bool
	skipK8sNodeReadinessCheck bool
	networkChaos              bool
	jitter                    time.Duration
	latency                   time.Duration
	packetLoss                float64
	packetReorder             float64
	packetCorrupt             float64
	bandwidth                 int
	diskEncryptionKeyTypes    []string
	withFirewall              string
	withUUIDHostnames         bool
	withSiderolinkAgent       agentFlag
	withJSONLogs              bool
	debugShellEnabled         bool
	configInjectionMethodFlag string
	mountOpts                 opts.MountOpt
)

// createCmd represents the cluster up command.
var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Creates a local docker-based or QEMU-based kubernetes cluster",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cli.WithContext(context.Background(), create)
	},
}

//nolint:gocyclo
func downloadBootAssets(ctx context.Context) error {
	// download & cache images if provides as URLs
	for _, downloadableImage := range []struct {
		path           *string
		disableArchive bool
	}{
		{
			path: &nodeVmlinuzPath,
		},
		{
			path:           &nodeInitramfsPath,
			disableArchive: true,
		},
		{
			path: &nodeISOPath,
		},
		{
			path: &nodeUSBPath,
		},
		{
			path: &nodeDiskImagePath,
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

		if os.MkdirAll(cacheDir, 0o755) != nil {
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

//nolint:gocyclo,cyclop
func create(ctx context.Context) error {
	if err := downloadBootAssets(ctx); err != nil {
		return err
	}

	if controlplanes < 1 {
		return errors.New("number of controlplanes can't be less than 1")
	}

	controlPlaneNanoCPUs, err := parseCPUShare(controlPlaneCpus)
	if err != nil {
		return fmt.Errorf("error parsing --cpus: %s", err)
	}

	workerNanoCPUs, err := parseCPUShare(workersCpus)
	if err != nil {
		return fmt.Errorf("error parsing --cpus-workers: %s", err)
	}

	controlPlaneMemory := int64(controlPlaneMemory) * 1024 * 1024
	workerMemory := int64(workersMemory) * 1024 * 1024

	// Validate CIDR range and allocate IPs
	fmt.Fprintln(os.Stderr, "validating CIDR and reserving IPs")

	cidr4, err := netip.ParsePrefix(networkCIDR)
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

	if networkIPv4 {
		cidrs = append(cidrs, cidr4)
	}

	if networkIPv6 {
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
		ips[j] = make([]netip.Addr, controlplanes+workers)

		for i := range ips[j] {
			ips[j][i], err = sideronet.NthIPInNetwork(cidrs[j], nodesOffset+i)
			if err != nil {
				return err
			}
		}
	}

	noMasqueradeCIDRs := make([]netip.Prefix, 0, len(networkNoMasqueradeCIDRs))

	for _, cidr := range networkNoMasqueradeCIDRs {
		var parsedCIDR netip.Prefix

		parsedCIDR, err = netip.ParsePrefix(cidr)
		if err != nil {
			return fmt.Errorf("error parsing non-masquerade CIDR %q: %w", cidr, err)
		}

		noMasqueradeCIDRs = append(noMasqueradeCIDRs, parsedCIDR)
	}

	// Parse nameservers
	nameserverIPs := make([]netip.Addr, len(nameservers))

	for i := range nameserverIPs {
		nameserverIPs[i], err = netip.ParseAddr(nameservers[i])
		if err != nil {
			return fmt.Errorf("failed parsing nameserver IP %q: %w", nameservers[i], err)
		}
	}

	// Virtual (shared) IP at the vipOffset IP in range, ex. 192.168.0.50
	var vip netip.Addr

	if useVIP {
		vip, err = sideronet.NthIPInNetwork(cidrs[0], vipOffset)
		if err != nil {
			return err
		}
	}

	// Validate network chaos flags
	if !networkChaos {
		if jitter != 0 || latency != 0 || packetLoss != 0 || packetReorder != 0 || packetCorrupt != 0 || bandwidth != 0 {
			return errors.New("network chaos flags can only be used with --with-network-chaos")
		}
	}

	provisioner, err := providers.Factory(ctx, provisionerName)
	if err != nil {
		return err
	}

	defer provisioner.Close() //nolint:errcheck

	// Craft cluster and node requests
	request := provision.ClusterRequest{
		Name: clusterName,

		Network: provision.NetworkRequest{
			Name:              clusterName,
			CIDRs:             cidrs,
			NoMasqueradeCIDRs: noMasqueradeCIDRs,
			GatewayAddrs:      gatewayIPs,
			MTU:               networkMTU,
			Nameservers:       nameserverIPs,
			LoadBalancerPorts: []int{controlPlanePort},
			CNI: provision.CNIConfig{
				BinPath:  cniBinPath,
				ConfDir:  cniConfDir,
				CacheDir: cniCacheDir,

				BundleURL: cniBundleURL,
			},
			DHCPSkipHostname:  dhcpSkipHostname,
			DockerDisableIPv6: dockerDisableIPv6,
			NetworkChaos:      networkChaos,
			Jitter:            jitter,
			Latency:           latency,
			PacketLoss:        packetLoss,
			PacketReorder:     packetReorder,
			PacketCorrupt:     packetCorrupt,
			Bandwidth:         bandwidth,
		},

		Image:          nodeImage,
		KernelPath:     nodeVmlinuzPath,
		InitramfsPath:  nodeInitramfsPath,
		ISOPath:        nodeISOPath,
		USBPath:        nodeUSBPath,
		IPXEBootScript: nodeIPXEBootScript,
		DiskImagePath:  nodeDiskImagePath,

		SelfExecutable: os.Args[0],
		StateDirectory: stateDir,
	}

	provisionOptions := []provision.Option{
		provision.WithDockerPortsHostIP(dockerHostIP),
		provision.WithBootlader(bootloaderEnabled),
		provision.WithUEFI(uefiEnabled),
		provision.WithTPM2(tpm2Enabled),
		provision.WithDebugShell(debugShellEnabled),
		provision.WithExtraUEFISearchPaths(extraUEFISearchPaths),
		provision.WithTargetArch(targetArch),
		provision.WithSiderolinkAgent(withSiderolinkAgent.IsEnabled()),
	}

	var configBundleOpts []bundle.Option

	if debugShellEnabled {
		if provisionerName != "qemu" {
			return errors.New("debug shell only supported with qemu provisioner")
		}
	}

	if ports != "" {
		if provisionerName != docker {
			return errors.New("exposed-ports flag only supported with docker provisioner")
		}

		portList := strings.Split(ports, ",")
		provisionOptions = append(provisionOptions, provision.WithDockerPorts(portList))
	}

	disks, err := getDisks()
	if err != nil {
		return err
	}

	if inputDir != "" {
		configBundleOpts = append(configBundleOpts, bundle.WithExistingConfigs(inputDir))
	} else {
		genOptions := []generate.Option{
			generate.WithInstallImage(nodeInstallImage),
			generate.WithDebug(configDebug),
			generate.WithDNSDomain(dnsDomain),
			generate.WithClusterDiscovery(enableClusterDiscovery),
		}

		for _, registryMirror := range registryMirrors {
			left, right, ok := strings.Cut(registryMirror, "=")
			if !ok {
				return fmt.Errorf("invalid registry mirror spec: %q", registryMirror)
			}

			genOptions = append(genOptions, generate.WithRegistryMirror(left, right))
		}

		for _, registryHost := range registryInsecure {
			genOptions = append(genOptions, generate.WithRegistryInsecureSkipVerify(registryHost))
		}

		genOptions = append(genOptions, provisioner.GenOptions(request.Network)...)

		if customCNIUrl != "" {
			genOptions = append(genOptions, generate.WithClusterCNIConfig(&v1alpha1.CNIConfig{
				CNIName: constants.CustomCNI,
				CNIUrls: []string{customCNIUrl},
			}))
		}

		if len(disks) > 1 {
			// convert provision disks to machine disks
			machineDisks := make([]*v1alpha1.MachineDisk, len(disks)-1)
			for i, disk := range disks[1:] {
				machineDisks[i] = &v1alpha1.MachineDisk{
					DeviceName:     provisioner.UserDiskName(i + 1),
					DiskPartitions: disk.Partitions,
				}
			}

			genOptions = append(genOptions, generate.WithUserDisks(machineDisks))
		}

		if talosVersion == "" {
			if provisionerName == docker {
				parts := strings.Split(nodeImage, ":")

				talosVersion = parts[len(parts)-1]
			} else {
				parts := strings.Split(nodeInstallImage, ":")

				talosVersion = parts[len(parts)-1]
			}
		}

		var versionContract *config.VersionContract

		if talosVersion != "latest" {
			versionContract, err = config.ParseContractFromVersion(talosVersion)
			if err != nil {
				return fmt.Errorf("error parsing Talos version %q: %w", talosVersion, err)
			}

			genOptions = append(genOptions, generate.WithVersionContract(versionContract))
		}

		if encryptStatePartition || encryptEphemeralPartition {
			diskEncryptionConfig := &v1alpha1.SystemDiskEncryptionConfig{}

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
					ip, err = sideronet.NthIPInNetwork(cidr4, 1)
					if err != nil {
						return err
					}

					const port = 4050

					keys = append(keys, &v1alpha1.EncryptionKey{
						KeyKMS: &v1alpha1.EncryptionKeyKMS{
							KMSEndpoint: "grpc://" + nethelpers.JoinHostPort(ip.String(), port),
						},
						KeySlot: i,
					})

					provisionOptions = append(provisionOptions, provision.WithKMS(nethelpers.JoinHostPort("0.0.0.0", port)))
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
					return fmt.Errorf("unknown key type %q", key)
				}
			}

			if len(keys) == 0 {
				return errors.New("no disk encryption key types enabled")
			}

			if encryptStatePartition {
				diskEncryptionConfig.StatePartition = &v1alpha1.EncryptionConfig{
					EncryptionProvider: encryption.LUKS2,
					EncryptionKeys:     keys,
				}
			}

			if encryptEphemeralPartition {
				diskEncryptionConfig.EphemeralPartition = &v1alpha1.EncryptionConfig{
					EncryptionProvider: encryption.LUKS2,
					EncryptionKeys:     keys,
				}
			}

			genOptions = append(genOptions, generate.WithSystemDiskEncryption(diskEncryptionConfig))
		}

		if useVIP {
			genOptions = append(genOptions,
				generate.WithNetworkOptions(
					v1alpha1.WithNetworkInterfaceVirtualIP(provisioner.GetFirstInterface(), vip.String()),
				),
			)
		}

		if enableKubeSpan {
			genOptions = append(genOptions,
				generate.WithNetworkOptions(
					v1alpha1.WithKubeSpan(),
				),
			)
		}

		if !bootloaderEnabled {
			// disable kexec, as this would effectively use the bootloader
			genOptions = append(genOptions,
				generate.WithSysctls(map[string]string{
					"kernel.kexec_load_disabled": "1",
				}),
			)
		}

		if controlPlanePort != constants.DefaultControlPlanePort {
			genOptions = append(genOptions,
				generate.WithLocalAPIServerPort(controlPlanePort),
			)
		}

		if kubePrismPort != constants.DefaultKubePrismPort {
			genOptions = append(genOptions,
				generate.WithKubePrismPort(kubePrismPort),
			)
		}

		externalKubernetesEndpoint := provisioner.GetExternalKubernetesControlPlaneEndpoint(request.Network, controlPlanePort)

		if useVIP {
			externalKubernetesEndpoint = "https://" + nethelpers.JoinHostPort(vip.String(), controlPlanePort)
		}

		provisionOptions = append(provisionOptions, provision.WithKubernetesEndpoint(externalKubernetesEndpoint))

		endpointList := provisioner.GetTalosAPIEndpoints(request.Network)

		switch {
		case forceEndpoint != "":
			// using non-default endpoints, provision additional cert SANs and fix endpoint list
			endpointList = []string{forceEndpoint}
			genOptions = append(genOptions, generate.WithAdditionalSubjectAltNames(endpointList))
		case forceInitNodeAsEndpoint:
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
			for i := range controlplanes {
				endpointList = append(endpointList, ips[0][i].String())
			}
		}

		inClusterEndpoint := provisioner.GetInClusterKubernetesControlPlaneEndpoint(request.Network, controlPlanePort)

		if useVIP {
			inClusterEndpoint = "https://" + nethelpers.JoinHostPort(vip.String(), controlPlanePort)
		}

		genOptions = append(genOptions, generate.WithEndpointList(endpointList))
		configBundleOpts = append(configBundleOpts,
			bundle.WithInputOptions(
				&bundle.InputOptions{
					ClusterName: clusterName,
					Endpoint:    inClusterEndpoint,
					KubeVersion: strings.TrimPrefix(kubernetesVersion, "v"),
					GenOptions:  genOptions,
				}),
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

	if err = addConfigPatch(configPatch, bundle.WithPatch); err != nil {
		return err
	}

	if err = addConfigPatch(configPatchControlPlane, bundle.WithPatchControlPlane); err != nil {
		return err
	}

	if err = addConfigPatch(configPatchWorker, bundle.WithPatchWorker); err != nil {
		return err
	}

	if withFirewall != "" {
		var defaultAction nethelpers.DefaultAction

		defaultAction, err = nethelpers.DefaultActionString(withFirewall)
		if err != nil {
			return err
		}

		var controlplaneIPs []netip.Addr

		for i := range ips {
			controlplaneIPs = append(controlplaneIPs, ips[i][:controlplanes]...)
		}

		configBundleOpts = append(configBundleOpts,
			bundle.WithPatchControlPlane([]configpatcher.Patch{firewallpatch.ControlPlane(defaultAction, cidrs, gatewayIPs, controlplaneIPs)}),
			bundle.WithPatchWorker([]configpatcher.Patch{firewallpatch.Worker(defaultAction, cidrs, gatewayIPs)}),
		)
	}

	var slb *siderolinkBuilder

	if withSiderolinkAgent.IsEnabled() {
		slb, err = newSiderolinkBuilder(gatewayIPs[0].String(), withSiderolinkAgent.IsTLS())
		if err != nil {
			return err
		}
	}

	if trustedRootsConfig := slb.TrustedRootsConfig(); trustedRootsConfig != nil {
		trustedRootsPatch, err := configloader.NewFromBytes(trustedRootsConfig)
		if err != nil {
			return fmt.Errorf("error loading trusted roots config: %w", err)
		}

		configBundleOpts = append(configBundleOpts, bundle.WithPatch([]configpatcher.Patch{configpatcher.NewStrategicMergePatch(trustedRootsPatch)}))
	}

	if withJSONLogs {
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
		if clusterWait {
			return errors.New("no talosconfig in the config bundle: cannot wait for cluster")
		}

		if applyConfigEnabled {
			return errors.New("no talosconfig in the config bundle: cannot apply config")
		}
	}

	if skipInjectingConfig {
		types := []machine.Type{machine.TypeControlPlane, machine.TypeWorker}

		if withInitNode {
			types = slices.Insert(types, 0, machine.TypeInit)
		}

		if err = configBundle.Write(".", encoder.CommentsAll, types...); err != nil {
			return err
		}
	}

	// Wireguard configuration.
	var wireguardConfigBundle *helpers.WireguardConfigBundle
	if wireguardCIDR != "" {
		wireguardConfigBundle, err = helpers.NewWireguardConfigBundle(ips[0], wireguardCIDR, 51111, controlplanes)
		if err != nil {
			return err
		}
	}

	var extraKernelArgs *procfs.Cmdline

	if extraBootKernelArgs != "" || withSiderolinkAgent.IsEnabled() {
		extraKernelArgs = procfs.NewCmdline(extraBootKernelArgs)
	}

	err = slb.SetKernelArgs(extraKernelArgs, withSiderolinkAgent.IsTunnel())
	if err != nil {
		return err
	}

	// Add talosconfig to provision options, so we'll have it to parse there
	provisionOptions = append(provisionOptions, provision.WithTalosConfig(configBundle.TalosConfig()))

	var configInjectionMethod provision.ConfigInjectionMethod

	switch configInjectionMethodFlag {
	case "", "default", "http":
		configInjectionMethod = provision.ConfigInjectionMethodHTTP
	case "metal-iso":
		configInjectionMethod = provision.ConfigInjectionMethodMetalISO
	default:
		return fmt.Errorf("unknown config injection method %q", configInjectionMethod)
	}

	// Create the controlplane nodes.
	for i := range controlplanes {
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
			Name:                  nodeName(clusterName, "controlplane", i+1, nodeUUID),
			Type:                  machine.TypeControlPlane,
			IPs:                   nodeIPs,
			Memory:                controlPlaneMemory,
			NanoCPUs:              controlPlaneNanoCPUs,
			Disks:                 disks,
			Mounts:                mountOpts.Value(),
			SkipInjectingConfig:   skipInjectingConfig,
			ConfigInjectionMethod: configInjectionMethod,
			BadRTC:                badRTC,
			ExtraKernelArgs:       extraKernelArgs,
			UUID:                  pointer.To(nodeUUID),
		}

		if withInitNode && i == 0 {
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
	for i := range extraDisks {
		driver := "ide"

		// ide driver is not supported on arm64
		if targetArch == "arm64" {
			driver = "virtio"
		}

		if i < len(extraDisksDrivers) {
			driver = extraDisksDrivers[i]
		}

		disks = append(disks, &provision.Disk{
			Size:            uint64(extraDiskSize) * 1024 * 1024,
			SkipPreallocate: !clusterDiskPreallocate,
			Driver:          driver,
			BlockSize:       diskBlockSize,
		})
	}

	for i := 1; i <= workers; i++ {
		cfg := configBundle.Worker()

		nodeIPs := make([]netip.Addr, len(cidrs))
		for j := range nodeIPs {
			nodeIPs[j] = ips[j][controlplanes+i-1]
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
				Name:                  nodeName(clusterName, "worker", i, nodeUUID),
				Type:                  machine.TypeWorker,
				IPs:                   nodeIPs,
				Memory:                workerMemory,
				NanoCPUs:              workerNanoCPUs,
				Disks:                 disks,
				Mounts:                mountOpts.Value(),
				Config:                cfg,
				ConfigInjectionMethod: configInjectionMethod,
				SkipInjectingConfig:   skipInjectingConfig,
				BadRTC:                badRTC,
				ExtraKernelArgs:       extraKernelArgs,
				UUID:                  pointer.To(nodeUUID),
			})
	}

	request.SiderolinkRequest = slb.SiderolinkRequest()

	cluster, err := provisioner.Create(ctx, request, provisionOptions...)
	if err != nil {
		return err
	}

	if debugShellEnabled {
		fmt.Println("You can now connect to debug shell on any node using these commands:")

		for _, node := range request.Nodes {
			talosDir, err := clientconfig.GetTalosDirectory()
			if err != nil {
				return nil
			}

			fmt.Printf("socat - UNIX-CONNECT:%s\n", filepath.Join(talosDir, "clusters", clusterName, node.Name+".serial"))
		}

		return nil
	}

	// No talosconfig in the bundle - skip the operations below
	if bundleTalosconfig == nil {
		return nil
	}

	// Create and save the talosctl configuration file.
	if err = saveConfig(bundleTalosconfig); err != nil {
		return err
	}

	clusterAccess := access.NewAdapter(cluster, provisionOptions...)
	defer clusterAccess.Close() //nolint:errcheck

	if applyConfigEnabled {
		err = clusterAccess.ApplyConfig(ctx, request.Nodes, request.SiderolinkRequest, os.Stdout)
		if err != nil {
			return err
		}
	}

	if err = postCreate(ctx, clusterAccess); err != nil {
		return err
	}

	return showCluster(cluster)
}

func nodeName(clusterName, role string, index int, uuid uuid.UUID) string {
	if withUUIDHostnames {
		return fmt.Sprintf("machine-%s", uuid)
	}

	return fmt.Sprintf("%s-%s-%d", clusterName, role, index)
}

func postCreate(ctx context.Context, clusterAccess *access.Adapter) error {
	if !withInitNode {
		if err := clusterAccess.Bootstrap(ctx, os.Stdout); err != nil {
			return fmt.Errorf("bootstrap error: %w", err)
		}
	}

	if !clusterWait {
		return nil
	}

	// Run cluster readiness checks
	checkCtx, checkCtxCancel := context.WithTimeout(ctx, clusterWaitTimeout)
	defer checkCtxCancel()

	checks := check.DefaultClusterChecks()

	if skipK8sNodeReadinessCheck {
		checks = slices.Concat(check.PreBootSequenceChecks(), check.K8sComponentsReadinessChecks())
	}

	checks = append(checks, check.ExtraClusterChecks()...)

	if err := check.Wait(checkCtx, clusterAccess, checks, check.StderrReporter()); err != nil {
		return err
	}

	if !skipKubeconfig {
		if err := mergeKubeconfig(ctx, clusterAccess); err != nil {
			return err
		}
	}

	return nil
}

func saveConfig(talosConfigObj *clientconfig.Config) (err error) {
	c, err := clientconfig.Open(talosconfig)
	if err != nil {
		return fmt.Errorf("error opening talos config: %w", err)
	}

	renames := c.Merge(talosConfigObj)
	for _, rename := range renames {
		fmt.Fprintf(os.Stderr, "renamed talosconfig context %s\n", rename.String())
	}

	return c.Save(talosconfig)
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

func getDisks() ([]*provision.Disk, error) {
	const GPTAlignment = 2 * 1024 * 1024 // 2 MB

	// should have at least a single primary disk
	disks := []*provision.Disk{
		{
			Size:            uint64(clusterDiskSize) * 1024 * 1024,
			SkipPreallocate: !clusterDiskPreallocate,
			Driver:          "virtio",
			BlockSize:       diskBlockSize,
		},
	}

	for _, disk := range clusterDisks {
		var (
			partitions     = strings.Split(disk, ":")
			diskPartitions = make([]*v1alpha1.DiskPartition, len(partitions)/2)
			diskSize       uint64
		)

		if len(partitions)%2 != 0 {
			return nil, errors.New("failed to parse malformed partition definitions")
		}

		partitionIndex := 0

		for j := 0; j < len(partitions); j += 2 {
			partitionPath := partitions[j]

			if !strings.HasPrefix(partitionPath, "/var") {
				return nil, errors.New("user disk partitions can only be mounted into /var folder")
			}

			value, e := strconv.ParseInt(partitions[j+1], 10, 0)
			partitionSize := uint64(value)

			if e != nil {
				partitionSize, e = humanize.ParseBytes(partitions[j+1])

				if e != nil {
					return nil, errors.New("failed to parse partition size")
				}
			}

			diskPartitions[partitionIndex] = &v1alpha1.DiskPartition{
				DiskSize:       v1alpha1.DiskSize(partitionSize),
				DiskMountPoint: partitionPath,
			}
			diskSize += partitionSize
			partitionIndex++
		}

		disks = append(disks, &provision.Disk{
			// add 2 MB per partition to make extra room for GPT and alignment
			Size:            diskSize + GPTAlignment*uint64(len(diskPartitions)+1),
			Partitions:      diskPartitions,
			SkipPreallocate: !clusterDiskPreallocate,
			Driver:          "ide",
			BlockSize:       diskBlockSize,
		})
	}

	return disks, nil
}

func init() {
	createCmd.Flags().StringVar(
		&talosconfig,
		"talosconfig",
		"",
		fmt.Sprintf("The path to the Talos configuration file. Defaults to '%s' env variable if set, otherwise '%s' and '%s' in order.",
			constants.TalosConfigEnvVar,
			filepath.Join("$HOME", constants.TalosDir, constants.TalosconfigFilename),
			filepath.Join(constants.ServiceAccountMountPath, constants.TalosconfigFilename),
		),
	)
	createCmd.Flags().StringVar(&nodeImage, "image", helpers.DefaultImage(images.DefaultTalosImageRepository), "the image to use")
	createCmd.Flags().StringVar(&nodeInstallImage, nodeInstallImageFlag, helpers.DefaultImage(images.DefaultInstallerImageRepository), "the installer image to use")
	createCmd.Flags().StringVar(&nodeVmlinuzPath, "vmlinuz-path", helpers.ArtifactPath(constants.KernelAssetWithArch), "the compressed kernel image to use")
	createCmd.Flags().StringVar(&nodeISOPath, "iso-path", "", "the ISO path to use for the initial boot (VM only)")
	createCmd.Flags().StringVar(&nodeUSBPath, "usb-path", "", "the USB stick image path to use for the initial boot (VM only)")
	createCmd.Flags().StringVar(&nodeInitramfsPath, "initrd-path", helpers.ArtifactPath(constants.InitramfsAssetWithArch), "initramfs image to use")
	createCmd.Flags().StringVar(&nodeDiskImagePath, "disk-image-path", "", "disk image to use")
	createCmd.Flags().StringVar(&nodeIPXEBootScript, "ipxe-boot-script", "", "iPXE boot script (URL) to use")
	createCmd.Flags().BoolVar(&applyConfigEnabled, "with-apply-config", false, "enable apply config when the VM is starting in maintenance mode")
	createCmd.Flags().BoolVar(&bootloaderEnabled, bootloaderEnabledFlag, true, "enable bootloader to load kernel and initramfs from disk image after install")
	createCmd.Flags().BoolVar(&uefiEnabled, "with-uefi", true, "enable UEFI on x86_64 architecture")
	createCmd.Flags().BoolVar(&tpm2Enabled, tpm2EnabledFlag, false, "enable TPM2 emulation support using swtpm")
	createCmd.Flags().BoolVar(&debugShellEnabled, withDebugShellFlag, false, "drop talos into a maintenance shell on boot, this is for advanced debugging for developers only")
	createCmd.Flags().MarkHidden("with-debug-shell") //nolint:errcheck
	createCmd.Flags().StringSliceVar(&extraUEFISearchPaths, "extra-uefi-search-paths", []string{}, "additional search paths for UEFI firmware (only applies when UEFI is enabled)")
	createCmd.Flags().StringSliceVar(&registryMirrors, registryMirrorFlag, []string{}, "list of registry mirrors to use in format: <registry host>=<mirror URL>")
	createCmd.Flags().StringSliceVar(&registryInsecure, registryInsecureFlag, []string{}, "list of registry hostnames to skip TLS verification for")
	createCmd.Flags().BoolVar(&configDebug, configDebugFlag, false, "enable debug in Talos config to send service logs to the console")
	createCmd.Flags().IntVar(&networkMTU, networkMTUFlag, 1500, "MTU of the cluster network")
	createCmd.Flags().StringVar(&networkCIDR, networkCIDRFlag, "10.5.0.0/24", "CIDR of the cluster network (IPv4, ULA network for IPv6 is derived in automated way)")
	createCmd.Flags().StringSliceVar(&networkNoMasqueradeCIDRs, networkNoMasqueradeCIDRsFlag, []string{}, "list of CIDRs to exclude from NAT (QEMU provisioner only)")
	createCmd.Flags().BoolVar(&networkIPv4, networkIPv4Flag, true, "enable IPv4 network in the cluster")
	createCmd.Flags().BoolVar(&networkIPv6, networkIPv6Flag, false, "enable IPv6 network in the cluster (QEMU provisioner only)")
	createCmd.Flags().StringVar(&wireguardCIDR, "wireguard-cidr", "", "CIDR of the wireguard network")
	createCmd.Flags().StringSliceVar(&nameservers, nameserversFlag, []string{"8.8.8.8", "1.1.1.1", "2001:4860:4860::8888", "2606:4700:4700::1111"}, "list of nameservers to use")
	createCmd.Flags().IntVar(&workers, "workers", 1, "the number of workers to create")
	createCmd.Flags().IntVar(&controlplanes, "masters", 1, "the number of masters to create")
	createCmd.Flags().MarkDeprecated("masters", "use --controlplanes instead") //nolint:errcheck
	createCmd.Flags().IntVar(&controlplanes, "controlplanes", 1, "the number of controlplanes to create")
	createCmd.Flags().StringVar(&controlPlaneCpus, "cpus", "2.0", "the share of CPUs as fraction (each control plane/VM)")
	createCmd.Flags().StringVar(&workersCpus, "cpus-workers", "2.0", "the share of CPUs as fraction (each worker/VM)")
	createCmd.Flags().IntVar(&controlPlaneMemory, "memory", 2048, "the limit on memory usage in MB (each control plane/VM)")
	createCmd.Flags().IntVar(&workersMemory, "memory-workers", 2048, "the limit on memory usage in MB (each worker/VM)")
	createCmd.Flags().IntVar(&clusterDiskSize, clusterDiskSizeFlag, 6*1024, "default limit on disk size in MB (each VM)")
	createCmd.Flags().UintVar(&diskBlockSize, "disk-block-size", 512, "disk block size (VM only)")
	createCmd.Flags().BoolVar(&clusterDiskPreallocate, clusterDiskPreallocateFlag, true, "whether disk space should be preallocated")
	createCmd.Flags().StringSliceVar(&clusterDisks, clusterDisksFlag, []string{}, "list of disks to create for each VM in format: <mount_point1>:<size1>:<mount_point2>:<size2>")
	createCmd.Flags().IntVar(&extraDisks, "extra-disks", 0, "number of extra disks to create for each worker VM")
	createCmd.Flags().StringSliceVar(&extraDisksDrivers, "extra-disks-drivers", nil, "driver for each extra disk (virtio, ide, ahci, scsi, nvme, megaraid)")
	createCmd.Flags().IntVar(&extraDiskSize, "extra-disks-size", 5*1024, "default limit on disk size in MB (each VM)")
	createCmd.Flags().StringVar(&targetArch, "arch", stdruntime.GOARCH, "cluster architecture")
	createCmd.Flags().BoolVar(&clusterWait, "wait", true, "wait for the cluster to be ready before returning")
	createCmd.Flags().DurationVar(&clusterWaitTimeout, "wait-timeout", 20*time.Minute, "timeout to wait for the cluster to be ready")
	createCmd.Flags().BoolVar(&forceInitNodeAsEndpoint, "init-node-as-endpoint", false, "use init node as endpoint instead of any load balancer endpoint")
	createCmd.Flags().StringVar(&forceEndpoint, forceEndpointFlag, "", "use endpoint instead of provider defaults")
	createCmd.Flags().StringVar(&kubernetesVersion, "kubernetes-version", constants.DefaultKubernetesVersion, "desired kubernetes version to run")
	createCmd.Flags().StringVarP(&inputDir, inputDirFlag, "i", "", "location of pre-generated config files")
	createCmd.Flags().StringSliceVar(&cniBinPath, "cni-bin-path", []string{filepath.Join(defaultCNIDir, "bin")}, "search path for CNI binaries (VM only)")
	createCmd.Flags().StringVar(&cniConfDir, "cni-conf-dir", filepath.Join(defaultCNIDir, "conf.d"), "CNI config directory path (VM only)")
	createCmd.Flags().StringVar(&cniCacheDir, "cni-cache-dir", filepath.Join(defaultCNIDir, "cache"), "CNI cache directory path (VM only)")
	createCmd.Flags().StringVar(&cniBundleURL, "cni-bundle-url", fmt.Sprintf("https://github.com/%s/talos/releases/download/%s/talosctl-cni-bundle-%s.tar.gz",
		images.Username, version.Trim(version.Tag), constants.ArchVariable), "URL to download CNI bundle from (VM only)")
	createCmd.Flags().StringVarP(&ports,
		"exposed-ports",
		"p",
		"",
		"Comma-separated list of ports/protocols to expose on init node. Ex -p <hostPort>:<containerPort>/<protocol (tcp or udp)> (Docker provisioner only)",
	)
	createCmd.Flags().StringVar(&dockerHostIP, "docker-host-ip", "0.0.0.0", "Host IP to forward exposed ports to (Docker provisioner only)")
	createCmd.Flags().BoolVar(&withInitNode, "with-init-node", false, "create the cluster with an init node")
	createCmd.Flags().StringVar(&customCNIUrl, customCNIUrlFlag, "", "install custom CNI from the URL (Talos cluster)")
	createCmd.Flags().StringVar(&dnsDomain, dnsDomainFlag, "cluster.local", "the dns domain to use for cluster")
	createCmd.Flags().BoolVar(&skipKubeconfig, "skip-kubeconfig", false, "skip merging kubeconfig from the created cluster")
	createCmd.Flags().BoolVar(&skipInjectingConfig, "skip-injecting-config", false, "skip injecting config from embedded metadata server, write config files to current directory")
	createCmd.Flags().BoolVar(&encryptStatePartition, encryptStatePartitionFlag, false, "enable state partition encryption")
	createCmd.Flags().BoolVar(&encryptEphemeralPartition, encryptEphemeralPartitionFlag, false, "enable ephemeral partition encryption")
	createCmd.Flags().StringArrayVar(&diskEncryptionKeyTypes, diskEncryptionKeyTypesFlag, []string{"uuid"}, "encryption key types to use for disk encryption (uuid, kms)")
	createCmd.Flags().StringVar(&talosVersion, talosVersionFlag, "", "the desired Talos version to generate config for (if not set, defaults to image version)")
	createCmd.Flags().BoolVar(&useVIP, useVIPFlag, false, "use a virtual IP for the controlplane endpoint instead of the loadbalancer")
	createCmd.Flags().BoolVar(&enableClusterDiscovery, withClusterDiscoveryFlag, true, "enable cluster discovery")
	createCmd.Flags().BoolVar(&enableKubeSpan, enableKubeSpanFlag, false, "enable KubeSpan system")
	createCmd.Flags().StringArrayVar(&configPatch, "config-patch", nil, "patch generated machineconfigs (applied to all node types), use @file to read a patch from file")
	createCmd.Flags().StringArrayVar(&configPatchControlPlane, "config-patch-control-plane", nil, "patch generated machineconfigs (applied to 'init' and 'controlplane' types)")
	createCmd.Flags().StringArrayVar(&configPatchWorker, "config-patch-worker", nil, "patch generated machineconfigs (applied to 'worker' type)")
	createCmd.Flags().BoolVar(&badRTC, "bad-rtc", false, "launch VM with bad RTC state (QEMU only)")
	createCmd.Flags().StringVar(&extraBootKernelArgs, "extra-boot-kernel-args", "", "add extra kernel args to the initial boot from vmlinuz and initramfs (QEMU only)")
	createCmd.Flags().BoolVar(&dockerDisableIPv6, "docker-disable-ipv6", false, "skip enabling IPv6 in containers (Docker only)")
	createCmd.Flags().IntVar(&controlPlanePort, controlPlanePortFlag, constants.DefaultControlPlanePort, "control plane port (load balancer and local API port, QEMU only)")
	createCmd.Flags().IntVar(&kubePrismPort, kubePrismFlag, constants.DefaultKubePrismPort, "KubePrism port (set to 0 to disable)")
	createCmd.Flags().BoolVar(&dhcpSkipHostname, "disable-dhcp-hostname", false, "skip announcing hostname via DHCP (QEMU only)")
	createCmd.Flags().BoolVar(&skipK8sNodeReadinessCheck, "skip-k8s-node-readiness-check", false, "skip k8s node readiness checks")
	createCmd.Flags().BoolVar(&networkChaos, "with-network-chaos", false, "enable to use network chaos parameters when creating a qemu cluster")
	createCmd.Flags().DurationVar(&jitter, "with-network-jitter", 0, "specify jitter on the bridge interface when creating a qemu cluster")
	createCmd.Flags().DurationVar(&latency, "with-network-latency", 0, "specify latency on the bridge interface when creating a qemu cluster")
	createCmd.Flags().Float64Var(&packetLoss, "with-network-packet-loss", 0.0, "specify percent of packet loss on the bridge interface when creating a qemu cluster. e.g. 50% = 0.50 (default: 0.0)")
	createCmd.Flags().Float64Var(&packetReorder, "with-network-packet-reorder", 0.0,
		"specify percent of reordered packets on the bridge interface when creating a qemu cluster. e.g. 50% = 0.50 (default: 0.0)")
	createCmd.Flags().Float64Var(&packetCorrupt, "with-network-packet-corrupt", 0.0,
		"specify percent of corrupt packets on the bridge interface when creating a qemu cluster. e.g. 50% = 0.50 (default: 0.0)")
	createCmd.Flags().IntVar(&bandwidth, "with-network-bandwidth", 0, "specify bandwidth restriction (in kbps) on the bridge interface when creating a qemu cluster")
	createCmd.Flags().StringVar(&withFirewall, firewallFlag, "", "inject firewall rules into the cluster, value is default policy - accept/block (QEMU only)")
	createCmd.Flags().BoolVar(&withUUIDHostnames, "with-uuid-hostnames", false, "use machine UUIDs as default hostnames (QEMU only)")
	createCmd.Flags().Var(&withSiderolinkAgent, "with-siderolink", "enables the use of siderolink agent as configuration apply mechanism. `true` or `wireguard` enables the agent, `tunnel` enables the agent with grpc tunneling") //nolint:lll
	createCmd.Flags().BoolVar(&withJSONLogs, "with-json-logs", false, "enable JSON logs receiver and configure Talos to send logs there")
	createCmd.Flags().StringVar(&configInjectionMethodFlag, "config-injection-method", "", "a method to inject machine config: default is HTTP server, 'metal-iso' to mount an ISO (QEMU only)")
	createCmd.Flags().Var(&mountOpts, "mount", "attach a mount to the container (Docker only)")

	createCmd.MarkFlagsMutuallyExclusive(inputDirFlag, nodeInstallImageFlag)
	createCmd.MarkFlagsMutuallyExclusive(inputDirFlag, configDebugFlag)
	createCmd.MarkFlagsMutuallyExclusive(inputDirFlag, dnsDomainFlag)
	createCmd.MarkFlagsMutuallyExclusive(inputDirFlag, withClusterDiscoveryFlag)
	createCmd.MarkFlagsMutuallyExclusive(inputDirFlag, registryMirrorFlag)
	createCmd.MarkFlagsMutuallyExclusive(inputDirFlag, registryInsecureFlag)
	createCmd.MarkFlagsMutuallyExclusive(inputDirFlag, customCNIUrlFlag)
	createCmd.MarkFlagsMutuallyExclusive(inputDirFlag, talosVersionFlag)
	createCmd.MarkFlagsMutuallyExclusive(inputDirFlag, encryptStatePartitionFlag)
	createCmd.MarkFlagsMutuallyExclusive(inputDirFlag, encryptEphemeralPartitionFlag)
	createCmd.MarkFlagsMutuallyExclusive(inputDirFlag, enableKubeSpanFlag)
	createCmd.MarkFlagsMutuallyExclusive(inputDirFlag, forceEndpointFlag)
	createCmd.MarkFlagsMutuallyExclusive(inputDirFlag, kubePrismFlag)
	createCmd.MarkFlagsMutuallyExclusive(inputDirFlag, diskEncryptionKeyTypesFlag)

	Cmd.AddCommand(createCmd)
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

// TrustedRootsConfig returns the trusted roots config for the current builder.
func (slb *siderolinkBuilder) TrustedRootsConfig() []byte {
	if slb == nil || slb.apiCert == nil {
		return nil
	}

	trustedRootsConfig := security.NewTrustedRootsConfigV1Alpha1()
	trustedRootsConfig.MetaName = "siderolink-ca"
	trustedRootsConfig.Certificates = string(slb.apiCert)

	marshaled, err := encoder.NewEncoder(trustedRootsConfig, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	if err != nil {
		panic(fmt.Sprintf("failed to marshal trusted roots config: %s", err))
	}

	return marshaled
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
		scheme := "grpc://"

		if slb.apiCert != nil {
			scheme = "https://"
		}

		apiLink := scheme + net.JoinHostPort(slb.wgHost, strconv.Itoa(slb.apiPort)) + "?jointoken=foo"

		if tunnel {
			apiLink += "&grpc_tunnel=true"
		}

		extraKernelArgs.Append("siderolink.api", apiLink)
		extraKernelArgs.Append("talos.events.sink", net.JoinHostPort(slb.nodeIPv6Addr, strconv.Itoa(slb.sinkPort)))
		extraKernelArgs.Append("talos.logging.kernel", "tcp://"+net.JoinHostPort(slb.nodeIPv6Addr, strconv.Itoa(slb.logPort)))

		if trustedRootsConfig := slb.TrustedRootsConfig(); trustedRootsConfig != nil {
			var buf bytes.Buffer

			zencoder, err := zstd.NewWriter(&buf)
			if err != nil {
				return fmt.Errorf("failed to create zstd encoder: %w", err)
			}

			_, err = zencoder.Write(trustedRootsConfig)
			if err != nil {
				return fmt.Errorf("failed to write zstd data: %w", err)
			}

			if err = zencoder.Close(); err != nil {
				return fmt.Errorf("failed to close zstd encoder: %w", err)
			}

			extraKernelArgs.Append(constants.KernelParamConfigInline, base64.StdEncoding.EncodeToString(buf.Bytes()))
		}

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
