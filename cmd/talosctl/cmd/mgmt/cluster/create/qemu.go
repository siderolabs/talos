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
	"net"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/google/uuid"
	"github.com/hashicorp/go-getter/v2"
	"github.com/klauspost/compress/zstd"
	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/gen/maps"
	"github.com/siderolabs/go-blockdevice/v2/encryption"
	"github.com/siderolabs/go-pointer"
	"github.com/siderolabs/go-procfs/procfs"
	sideronet "github.com/siderolabs/net"
	"github.com/siderolabs/siderolink/pkg/wireguard"

	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create/clustermaker"
	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/internal/firewallpatch"
	clientconfig "github.com/siderolabs/talos/pkg/machinery/client/config"
	"github.com/siderolabs/talos/pkg/machinery/config/bundle"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/configpatcher"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/generate"
	"github.com/siderolabs/talos/pkg/machinery/config/types/security"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/provision"
)

// vipOffset is the offset from the network address of the CIDR to use for allocating the Virtual (shared) IP address, if enabled.
const vipOffset = 50

func getQemuClusterMaker(qOps qemuOps, cOps commonOps, provisioner provision.Provisioner) (clustermaker.ClusterMaker, error) {
	talosversion := getQemuTalosVersion(cOps, qOps)

	return clustermaker.New(clustermaker.Input{
		Ops:          cOps,
		Provisioner:  provisioner,
		TalosVersion: talosversion,
	})
}

func getQemuTalosVersion(cOps commonOps, qOps qemuOps) string {
	talosversion := cOps.TalosVersion
	if talosversion == "" {
		parts := strings.Split(qOps.nodeInstallImage, ":")
		talosversion = parts[len(parts)-1]
	}

	return talosversion
}

//nolint:gocyclo,cyclop
func _createQemuCluster(ctx context.Context, qOps qemuOps, cOps commonOps, provisioner provision.Provisioner, cm clustermaker.ClusterMaker) error {
	clusterReq := cm.GetPartialClusterRequest()

	disks, err := getDisks(qOps)
	if err != nil {
		return err
	}

	cm.AddProvisionOps(
		provision.WithBootlader(qOps.bootloaderEnabled),
		provision.WithUEFI(qOps.uefiEnabled),
		provision.WithTPM2(qOps.tpm2Enabled),
		provision.WithDebugShell(qOps.debugShellEnabled),
		provision.WithExtraUEFISearchPaths(qOps.extraUEFISearchPaths),
		provision.WithTargetArch(qOps.targetArch),
		provision.WithSiderolinkAgent(qOps.withSiderolinkAgent.IsEnabled()),
		provision.WithIOMMU(qOps.withIOMMU),
	)

	if qOps.withFirewall != "" {
		var defaultAction nethelpers.DefaultAction

		defaultAction, err = nethelpers.DefaultActionString(qOps.withFirewall)
		if err != nil {
			return err
		}

		var controlplaneIPs []netip.Addr

		for _, n := range clusterReq.Nodes.ControlPlaneNodes() {
			controlplaneIPs = append(controlplaneIPs, n.IPs...)
		}

		cm.AddCfgBundleOpts(
			bundle.WithPatchControlPlane([]configpatcher.Patch{firewallpatch.ControlPlane(defaultAction, clusterReq.Network.CIDRs, clusterReq.Network.GatewayAddrs, controlplaneIPs)}),
			bundle.WithPatchWorker([]configpatcher.Patch{firewallpatch.Worker(defaultAction, clusterReq.Network.CIDRs, clusterReq.Network.GatewayAddrs)}),
		)
	}

	var slb *siderolinkBuilder

	if qOps.withSiderolinkAgent.IsEnabled() {
		slb, err = newSiderolinkBuilder(clusterReq.Network.GatewayAddrs[0].String(), qOps.withSiderolinkAgent.IsTLS())
		if err != nil {
			return err
		}
	}

	if trustedRootsConfig := slb.TrustedRootsConfig(); trustedRootsConfig != nil {
		trustedRootsPatch, err := configloader.NewFromBytes(trustedRootsConfig)
		if err != nil {
			return fmt.Errorf("error loading trusted roots config: %w", err)
		}

		cm.AddCfgBundleOpts(bundle.WithPatch([]configpatcher.Patch{configpatcher.NewStrategicMergePatch(trustedRootsPatch)}))
	}

	// If pre-existing talos config is not provided:
	if cOps.InputDir == "" {
		cm.AddGenOps(generate.WithInstallImage(qOps.nodeInstallImage))

		if len(disks) > 1 {
			// convert provision disks to machine disks
			machineDisks := make([]*v1alpha1.MachineDisk, len(disks)-1)
			for i, disk := range disks[1:] {
				machineDisks[i] = &v1alpha1.MachineDisk{
					DeviceName:     provisioner.UserDiskName(i + 1),
					DiskPartitions: disk.Partitions,
				}
			}

			cm.AddGenOps(generate.WithUserDisks(machineDisks))
		}

		if qOps.encryptStatePartition || qOps.encryptEphemeralPartition {
			diskEncryptionConfig := &v1alpha1.SystemDiskEncryptionConfig{}

			var keys []*v1alpha1.EncryptionKey

			for i, key := range qOps.diskEncryptionKeyTypes {
				switch key {
				case "uuid":
					keys = append(keys, &v1alpha1.EncryptionKey{
						KeyNodeID: &v1alpha1.EncryptionKeyNodeID{},
						KeySlot:   i,
					})
				case "kms":
					var ip netip.Addr

					// get bridge IP
					ip, err = sideronet.NthIPInNetwork(cm.GetCIDR4(), 1)
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

					cm.AddProvisionOps(provision.WithKMS(nethelpers.JoinHostPort("0.0.0.0", port)))
				case "tpm":
					keyTPM := &v1alpha1.EncryptionKeyTPM{}

					if cm.GetVersionContract().SecureBootEnrollEnforcementSupported() {
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

			cm.AddGenOps(generate.WithSystemDiskEncryption(diskEncryptionConfig))
		}

		if qOps.useVIP {
			vip, err := sideronet.NthIPInNetwork(clusterReq.Network.CIDRs[0], vipOffset)
			if err != nil {
				return fmt.Errorf("failed to get virtual IP: %w", err)
			}

			cm.AddGenOps(generate.WithNetworkOptions(
				v1alpha1.WithNetworkInterfaceVirtualIP(provisioner.GetFirstInterface(), vip.String()),
			))

			externalKubernetesEndpoint := "https://" + nethelpers.JoinHostPort(vip.String(), cOps.ControlPlanePort)

			cm.SetInClusterEndpoint(externalKubernetesEndpoint)
			cm.AddProvisionOps(provision.WithKubernetesEndpoint(externalKubernetesEndpoint))
		}

		if !qOps.bootloaderEnabled {
			// disable kexec, as this would effectively use the bootloader
			cm.AddGenOps(generate.WithSysctls(map[string]string{"kernel.kexec_load_disabled": "1"}))
		}
	}

	fmt.Fprintln(os.Stderr, "validating CIDR and reserving IPs")

	if len(clusterReq.Network.CIDRs) == 0 {
		return errors.New("neither IPv4 nor IPv6 network was enabled")
	}

	// Validate network chaos flags
	if !qOps.networkChaos {
		if qOps.jitter != 0 || qOps.latency != 0 || qOps.packetLoss != 0 || qOps.packetReorder != 0 || qOps.packetCorrupt != 0 || qOps.bandwidth != 0 {
			return errors.New("network chaos flags can only be used with --with-network-chaos")
		}
	}

	err = downloadBootAssets(ctx, qOps)
	if err != nil {
		return err
	}

	networkRequest, err := getQemuNetworkRequest(clusterReq, qOps, cOps)
	if err != nil {
		return err
	}

	// Craft cluster and node requests
	clusterReq.Network = networkRequest
	clusterReq.KernelPath = qOps.nodeVmlinuzPath
	clusterReq.InitramfsPath = qOps.nodeInitramfsPath
	clusterReq.ISOPath = qOps.nodeISOPath
	clusterReq.IPXEBootScript = qOps.nodeIPXEBootScript
	clusterReq.DiskImagePath = qOps.nodeDiskImagePath
	clusterReq.USBPath = qOps.nodeUSBPath
	clusterReq.UKIPath = qOps.nodeUKIPath

	var extraKernelArgs *procfs.Cmdline

	if qOps.extraBootKernelArgs != "" || qOps.withSiderolinkAgent.IsEnabled() {
		extraKernelArgs = procfs.NewCmdline(qOps.extraBootKernelArgs)
	}

	err = slb.SetKernelArgs(extraKernelArgs, qOps.withSiderolinkAgent.IsTunnel())
	if err != nil {
		return err
	}

	var configInjectionMethod provision.ConfigInjectionMethod

	switch qOps.configInjectionMethodFlagVal {
	case "", "default", "http":
		configInjectionMethod = provision.ConfigInjectionMethodHTTP
	case "metal-iso":
		configInjectionMethod = provision.ConfigInjectionMethodMetalISO
	default:
		return fmt.Errorf("unknown config injection method %q", configInjectionMethod)
	}

	nodes := []provision.NodeRequest{}

	// Create the controlplane nodes.
	for i, n := range clusterReq.Nodes.ControlPlaneNodes() {
		nodeUUID := uuid.New()

		err = slb.DefineIPv6ForUUID(nodeUUID)
		if err != nil {
			return err
		}

		n.Name = getQemuNodeName(clusterReq.Name, "controlplane", i+1, nodeUUID, qOps)
		n.Disks = disks
		n.ConfigInjectionMethod = configInjectionMethod
		n.BadRTC = qOps.badRTC
		n.ExtraKernelArgs = extraKernelArgs
		n.UUID = pointer.To(nodeUUID)
		n.Quirks = quirks.New(getQemuTalosVersion(cOps, qOps))

		nodes = append(nodes, n)
	}

	// append extra worker disks
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
		})
	}

	for i, n := range clusterReq.Nodes.WorkerNodes() {
		nodeUUID := uuid.New()

		err = slb.DefineIPv6ForUUID(nodeUUID)
		if err != nil {
			return err
		}

		n.Name = getQemuNodeName(clusterReq.Name, "worker", i+1, nodeUUID, qOps)
		n.Disks = disks
		n.ConfigInjectionMethod = configInjectionMethod
		n.BadRTC = qOps.badRTC
		n.ExtraKernelArgs = extraKernelArgs
		n.UUID = pointer.To(nodeUUID)
		n.Quirks = quirks.New(getQemuTalosVersion(cOps, qOps))

		nodes = append(nodes, n)
	}

	clusterReq.Nodes = nodes

	clusterReq.SiderolinkRequest = slb.SiderolinkRequest()

	err = cm.CreateCluster(ctx, clusterReq)
	if err != nil {
		return err
	}

	if qOps.debugShellEnabled {
		fmt.Println("You can now connect to debug shell on any node using these commands:")

		for _, node := range nodes {
			talosDir, err := clientconfig.GetTalosDirectory()
			if err != nil {
				return err
			}

			fmt.Printf("socat - UNIX-CONNECT:%s\n", filepath.Join(talosDir, "clusters", clusterReq.Name, node.Name+".serial"))
		}

		return nil
	}

	return cm.PostCreate(ctx)
}

//nolint:gocyclo
func downloadBootAssets(ctx context.Context, qOps qemuOps) error {
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
			path: &qOps.nodeUKIPath,
		},
		{
			path: &qOps.nodeUSBPath,
		},
		{
			path: &qOps.nodeDiskImagePath,
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

func getDisks(qemuOps qemuOps) ([]*provision.Disk, error) {
	const GPTAlignment = 2 * 1024 * 1024 // 2 MB

	// should have at least a single primary disk
	disks := []*provision.Disk{
		{
			Size:            uint64(qemuOps.clusterDiskSize) * 1024 * 1024,
			SkipPreallocate: !qemuOps.clusterDiskPreallocate,
			Driver:          "virtio",
			BlockSize:       qemuOps.diskBlockSize,
		},
	}

	for _, disk := range qemuOps.clusterDisks {
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
			SkipPreallocate: !qemuOps.clusterDiskPreallocate,
			Driver:          "ide",
			BlockSize:       qemuOps.diskBlockSize,
		})
	}

	return disks, nil
}

func getQemuNodeName(clusterName, role string, index int, uuid uuid.UUID, qemuOps qemuOps) string {
	if qemuOps.withUUIDHostnames {
		return fmt.Sprintf("machine-%s", uuid)
	}

	return fmt.Sprintf("%s-%s-%d", clusterName, role, index)
}

func newSiderolinkBuilder(wgHost string, useTLS bool) (*siderolinkBuilder, error) {
	prefix := networkPrefix("")

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

func generateRandomNodeAddr(prefix netip.Prefix) (netip.Prefix, error) {
	return wireguard.GenerateRandomNodeAddr(prefix)
}

func networkPrefix(prefix string) netip.Prefix {
	return wireguard.NetworkPrefix(prefix)
}

func getQemuNetworkRequest(partial clustermaker.PartialClusterRequest, qOps qemuOps, cOps commonOps) (req provision.NetworkRequest, err error) {
	// Parse nameservers
	req = partial.Network
	nameserverIPs := make([]netip.Addr, len(qOps.nameservers))

	for i := range nameserverIPs {
		nameserverIPs[i], err = netip.ParseAddr(qOps.nameservers[i])
		if err != nil {
			return req, fmt.Errorf("failed parsing nameserver IP %q: %w", qOps.nameservers[i], err)
		}
	}

	noMasqueradeCIDRs := make([]netip.Prefix, 0, len(qOps.networkNoMasqueradeCIDRs))

	for _, cidr := range qOps.networkNoMasqueradeCIDRs {
		var parsedCIDR netip.Prefix

		parsedCIDR, err = netip.ParsePrefix(cidr)
		if err != nil {
			return req, fmt.Errorf("error parsing non-masquerade CIDR %q: %w", cidr, err)
		}

		noMasqueradeCIDRs = append(noMasqueradeCIDRs, parsedCIDR)
	}

	req.Nameservers = nameserverIPs
	req.CNI = provision.CNIConfig{
		BinPath:  qOps.cniBinPath,
		ConfDir:  qOps.cniConfDir,
		CacheDir: qOps.cniCacheDir,

		BundleURL: qOps.cniBundleURL,
	}
	req.LoadBalancerPorts = []int{cOps.ControlPlanePort}
	req.DHCPSkipHostname = qOps.dhcpSkipHostname
	req.NetworkChaos = qOps.networkChaos
	req.Jitter = qOps.jitter
	req.Latency = qOps.latency
	req.PacketLoss = qOps.packetLoss
	req.PacketReorder = qOps.packetReorder
	req.PacketCorrupt = qOps.packetCorrupt
	req.Bandwidth = qOps.bandwidth
	req.NoMasqueradeCIDRs = noMasqueradeCIDRs

	return req, nil
}
