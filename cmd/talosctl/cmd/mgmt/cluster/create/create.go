// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package create

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/go-getter/v2"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-pointer"
	sideronet "github.com/siderolabs/net"

	"github.com/siderolabs/talos/pkg/bytesize"
	"github.com/siderolabs/talos/pkg/cluster/check"
	clientconfig "github.com/siderolabs/talos/pkg/machinery/client/config"
	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/bundle"
	"github.com/siderolabs/talos/pkg/machinery/config/configpatcher"
	"github.com/siderolabs/talos/pkg/machinery/config/generate"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/provision"
	"github.com/siderolabs/talos/pkg/provision/access"
)

type nodeResources struct {
	cpu    string
	memory bytesize.ByteSize
}

type parsedNodeResources struct {
	nanoCPUs int64
	memory   bytesize.ByteSize
}

func parseResources(res nodeResources) (parsedNodeResources, error) {
	nanoCPUs, err := parseCPUShare(res.cpu)
	if err != nil {
		return parsedNodeResources{}, err
	}

	return parsedNodeResources{
		nanoCPUs: nanoCPUs,
		memory:   res.memory,
	}, nil
}

func getCidr4(cOps commonOps) (netip.Prefix, error) {
	cidr4, err := netip.ParsePrefix(cOps.networkCIDR)
	if err != nil {
		return netip.Prefix{}, fmt.Errorf("error validating cidr block: %w", err)
	}

	if !cidr4.Addr().Is4() {
		return netip.Prefix{}, errors.New("IPV4 CIDR expected, got IPV6 CIDR")
	}

	return cidr4, nil
}

func createNodeRequests(cOps commonOps, controlplaneRes, workerRes parsedNodeResources, nodeIPs [][]netip.Addr) (
	controlplanes, workers []provision.NodeRequest, err error,
) {
	if cOps.controlplanes < 1 {
		return nil, nil, errors.New("number of controlplanes can't be less than 1")
	}

	for i := range cOps.controlplanes {
		nodeUUID := uuid.New()
		machineName := fmt.Sprintf("%s-%s-%d", cOps.rootOps.ClusterName, "controlplane", i+1)

		if cOps.withUUIDHostnames {
			machineName = fmt.Sprintf("%s-%s", "machine", nodeUUID)
		}

		machineType := machine.TypeControlPlane
		ips := getNodeIPs(nodeIPs, i)
		controlplanes = append(controlplanes, provision.NodeRequest{
			Name:     machineName,
			IPs:      ips,
			Type:     machineType,
			Memory:   int64(controlplaneRes.memory.Bytes()),
			NanoCPUs: controlplaneRes.nanoCPUs,
			UUID:     pointer.To(nodeUUID),
		})
	}

	for workerIndex := range cOps.workers {
		nodeUUID := uuid.New()
		machineName := fmt.Sprintf("%s-%s-%d", cOps.rootOps.ClusterName, "worker", workerIndex+1)

		if cOps.withUUIDHostnames {
			machineName = fmt.Sprintf("%s-%s", "machine", nodeUUID)
		}

		nodeIndex := cOps.controlplanes + workerIndex
		ips := getNodeIPs(nodeIPs, nodeIndex)
		workers = append(workers, provision.NodeRequest{
			Name:     machineName,
			IPs:      ips,
			Type:     machine.TypeWorker,
			Memory:   int64(workerRes.memory.Bytes()),
			NanoCPUs: workerRes.nanoCPUs,
			UUID:     pointer.To(nodeUUID),
		})
	}

	return controlplanes, workers, nil
}

func getNodeIPs(ips [][]netip.Addr, nodeIndex int) []netip.Addr {
	return xslices.Map(ips, func(ips []netip.Addr) netip.Addr {
		return ips[nodeIndex]
	})
}

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

func parseDisksFlag(disks []string) ([]diskRequest, error) {
	result := []diskRequest{}

	if len(disks) == 0 {
		return nil, errors.New("at least one disk has to be specified")
	}

	for _, d := range disks {
		parts := strings.SplitN(d, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid disk format: %q", d)
		}

		size := bytesize.WithDefaultUnit("MiB")

		err := size.Set(parts[1])
		if err != nil {
			return nil, fmt.Errorf("invalid size in disk spec: %q", d)
		}

		result = append(result, diskRequest{
			Driver:    parts[0],
			SizeBytes: size.Bytes(),
		})
	}

	return result, nil
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

func getRegistryMirrorGenOps(cOps commonOps) ([]generate.Option, error) {
	ops := make([]generate.Option, 0, len(cOps.registryMirrors))

	for _, registryMirror := range cOps.registryMirrors {
		left, right, ok := strings.Cut(registryMirror, "=")
		if !ok {
			return nil, fmt.Errorf("invalid registry mirror spec: %q", registryMirror)
		}

		ops = append(ops, generate.WithRegistryMirror(left, right))
	}

	return ops, nil
}

func getBaseClusterRequest(cOps commonOps, cidrs []netip.Prefix, gatewayIPs []netip.Addr) provision.ClusterRequest {
	return provision.ClusterRequest{
		Name:           cOps.rootOps.ClusterName,
		SelfExecutable: os.Args[0],
		StateDirectory: cOps.rootOps.StateDir,

		Network: provision.NetworkRequest{
			Name:              cOps.rootOps.ClusterName,
			CIDRs:             cidrs,
			GatewayAddrs:      gatewayIPs,
			MTU:               cOps.networkMTU,
			LoadBalancerPorts: []int{cOps.controlPlanePort},
		},
	}
}

func getVersionContractGenOps(cOps commonOps) ([]generate.Option, *config.VersionContract, error) {
	if cOps.talosVersion == "latest" {
		return nil, nil, nil
	}

	versionContract, err := config.ParseContractFromVersion(cOps.talosVersion)
	if err != nil {
		return nil, nil, fmt.Errorf("error parsing Talos version %q: %w", cOps.talosVersion, err)
	}

	return []generate.Option{generate.WithVersionContract(versionContract)}, versionContract, nil
}

func getWithAdditionalSubjectAltNamesGenOps(endpointList []string) []generate.Option {
	return xslices.Map(endpointList, func(endpointHostPort string) generate.Option {
		endpointHost, _, err := net.SplitHostPort(endpointHostPort)
		if err != nil {
			endpointHost = endpointHostPort
		}

		return generate.WithAdditionalSubjectAltNames([]string{endpointHost})
	})
}

func getIps(cidr netip.Prefix, cOps commonOps) ([]netip.Addr, error) {
	nodes := cOps.controlplanes + cOps.workers
	ips := make([]netip.Addr, nodes)

	for i := range nodes {
		ip, err := sideronet.NthIPInNetwork(cidr, nodesOffset+i)
		if err != nil {
			return nil, err
		}

		ips[i] = ip
	}

	return ips, nil
}

type diskRequest struct {
	Driver    string
	SizeBytes uint64
}

func getDisks(qOps qemuOps) (primaryDisks, workerExtraDisks []*provision.Disk, err error) {
	diskRequests, err := parseDisksFlag(qOps.disks)
	if err != nil {
		return
	}

	primaryDisks = []*provision.Disk{
		{
			Size:            diskRequests[0].SizeBytes,
			SkipPreallocate: !qOps.preallocateDisks,
			Driver:          diskRequests[0].Driver,
			BlockSize:       qOps.diskBlockSize,
		},
	}
	// get worker extra disks
	for _, d := range diskRequests[1:] {
		workerExtraDisks = append(workerExtraDisks, &provision.Disk{
			Size:            d.SizeBytes,
			SkipPreallocate: !qOps.preallocateDisks,
			Driver:          d.Driver,
			BlockSize:       qOps.diskBlockSize,
		})
	}

	return primaryDisks, workerExtraDisks, nil
}

func getConfigPatchBundleOps(cOps commonOps) ([]bundle.Option, error) {
	configBundleOpts := []bundle.Option{}

	addConfigPatch := func(configPatches []string, configOpt func([]configpatcher.Patch) bundle.Option) error {
		var patches []configpatcher.Patch

		patches, err := configpatcher.LoadPatches(configPatches)
		if err != nil {
			return fmt.Errorf("error parsing config patch: %w", err)
		}

		configBundleOpts = append(configBundleOpts, configOpt(patches))

		return nil
	}
	if err := addConfigPatch(cOps.configPatch, bundle.WithPatch); err != nil {
		return nil, err
	}

	if err := addConfigPatch(cOps.configPatchControlPlane, bundle.WithPatchControlPlane); err != nil {
		return nil, err
	}

	if err := addConfigPatch(cOps.configPatchWorker, bundle.WithPatchWorker); err != nil {
		return nil, err
	}

	return configBundleOpts, nil
}

func postCreate(
	ctx context.Context,
	cOps commonOps,
	bundleTalosconfig *clientconfig.Config,
	cluster provision.Cluster,
	provisionOptions []provision.Option,
	request provision.ClusterRequest,
) error {
	if err := saveConfig(bundleTalosconfig, cOps.talosconfigDestination); err != nil {
		return err
	}

	clusterAccess := access.NewAdapter(cluster, provisionOptions...)
	defer clusterAccess.Close() //nolint:errcheck

	if cOps.applyConfigEnabled {
		err := clusterAccess.ApplyConfig(ctx, request.Nodes, request.SiderolinkRequest, os.Stdout)
		if err != nil {
			return err
		}
	}

	return bootstrapCluster(ctx, clusterAccess, cOps)
}

func bootstrapCluster(ctx context.Context, clusterAccess *access.Adapter, cOps commonOps) error {
	if cOps.skipInjectingConfig && !cOps.applyConfigEnabled {
		return nil
	}

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

	checks = slices.Concat(checks, check.ExtraClusterChecks())

	if err := check.Wait(checkCtx, clusterAccess, checks, check.StderrReporter()); err != nil {
		return err
	}

	if cOps.skipKubeconfig {
		return nil
	}

	return mergeKubeconfig(ctx, clusterAccess)
}

type createClusterRequestOps struct {
	commonOps              commonOps
	provisioner            provision.Provisioner
	withExtraGenOpts       func(provision.ClusterRequest) []generate.Option
	withExtraProvisionOpts func(provision.ClusterRequest) []provision.Option
	modifyClusterRequest   func(provision.ClusterRequest) (provision.ClusterRequest, error)
	modifyNodes            func(cr provision.ClusterRequest, cp, w []provision.NodeRequest) (controlplanes, workers []provision.NodeRequest, err error)
}

//nolint:gocyclo
func createClusterRequest(ops createClusterRequestOps) (clusterCreateRequestData, error) {
	cOps := ops.commonOps
	provisioner := ops.provisioner

	controlplaneResources, err := parseResources(cOps.controlplaneResources)
	if err != nil {
		return clusterCreateRequestData{}, fmt.Errorf("error parsing controlplane resources: %s", err)
	}

	workerResources, err := parseResources(cOps.workerResources)
	if err != nil {
		return clusterCreateRequestData{}, fmt.Errorf("error parsing worker resources: %s", err)
	}

	cidr, err := getCidr4(cOps)
	if err != nil {
		return clusterCreateRequestData{}, err
	}

	nodeIPs, err := getIps(cidr, cOps)
	if err != nil {
		return clusterCreateRequestData{}, err
	}

	gatewayIP, err := sideronet.NthIPInNetwork(cidr, gatewayOffset)
	if err != nil {
		return clusterCreateRequestData{}, err
	}

	clusterRequest := getBaseClusterRequest(cOps, []netip.Prefix{cidr}, []netip.Addr{gatewayIP})

	controlplanes, workers, err := createNodeRequests(cOps, controlplaneResources, workerResources, [][]netip.Addr{nodeIPs})
	if err != nil {
		return clusterCreateRequestData{}, err
	}

	clusterRequest.Nodes = append(clusterRequest.Nodes, controlplanes...)
	clusterRequest.Nodes = append(clusterRequest.Nodes, workers...)

	clusterRequest, err = ops.modifyClusterRequest(clusterRequest)
	if err != nil {
		return clusterCreateRequestData{}, err
	}

	genOptions := []generate.Option{}

	registryMirrorOps, err := getRegistryMirrorGenOps(cOps)
	if err != nil {
		return clusterCreateRequestData{}, err
	}

	genOptions = append(genOptions, registryMirrorOps...)

	versionContractGenOps, _, err := getVersionContractGenOps(cOps)
	if err != nil {
		return clusterCreateRequestData{}, err
	}

	genOptions = append(genOptions, versionContractGenOps...)
	genOptions = append(genOptions, provisioner.GenOptions(clusterRequest.Network)...)
	genOptions = append(genOptions, ops.withExtraGenOpts(clusterRequest)...)

	configBundleOpts := []bundle.Option{}

	configBundleOpts = append(configBundleOpts,
		bundle.WithInputOptions(
			&bundle.InputOptions{
				ClusterName: cOps.rootOps.ClusterName,
				Endpoint:    provisioner.GetInClusterKubernetesControlPlaneEndpoint(clusterRequest.Network, cOps.controlPlanePort),
				KubeVersion: strings.TrimPrefix(cOps.kubernetesVersion, "v"),
				GenOptions:  genOptions,
			}),
	)

	provisionOptions := []provision.Option{
		provision.WithKubernetesEndpoint(provisioner.GetExternalKubernetesControlPlaneEndpoint(clusterRequest.Network, cOps.controlPlanePort)),
	}

	configPatchBundleOps, err := getConfigPatchBundleOps(cOps)
	if err != nil {
		return clusterCreateRequestData{}, err
	}

	configBundleOpts = append(configBundleOpts, configPatchBundleOps...)

	configBundle, err := bundle.NewBundle(configBundleOpts...)
	if err != nil {
		return clusterCreateRequestData{}, err
	}

	bundleTalosconfig := configBundle.TalosConfig()

	provisionOptions = append(provisionOptions, provision.WithTalosConfig(configBundle.TalosConfig()))
	provisionOptions = append(provisionOptions, ops.withExtraProvisionOpts(clusterRequest)...)

	for i := range controlplanes {
		controlplanes[i].Config = configBundle.ControlPlane()
		controlplanes[i].SkipInjectingConfig = cOps.skipInjectingConfig
	}

	for i := range workers {
		workers[i].Config = configBundle.Worker()
		workers[i].SkipInjectingConfig = cOps.skipInjectingConfig
	}

	controlplanes, workers, err = ops.modifyNodes(clusterRequest, controlplanes, workers)
	if err != nil {
		return clusterCreateRequestData{}, err
	}

	clusterRequest.Nodes = provision.NodeRequests{}
	clusterRequest.Nodes = append(clusterRequest.Nodes, controlplanes...)
	clusterRequest.Nodes = append(clusterRequest.Nodes, workers...)

	return clusterCreateRequestData{
		clusterRequest:   clusterRequest,
		provisionOptions: provisionOptions,
		talosconfig:      bundleTalosconfig,
	}, nil
}
