// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package clustermaker

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"os"
	"slices"
	"strings"

	sideronet "github.com/siderolabs/net"

	clustercmd "github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster"
	"github.com/siderolabs/talos/cmd/talosctl/pkg/mgmt/helpers"
	"github.com/siderolabs/talos/pkg/cluster/check"
	clientconfig "github.com/siderolabs/talos/pkg/machinery/client/config"
	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/bundle"
	"github.com/siderolabs/talos/pkg/machinery/config/configpatcher"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/generate"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/provision"
	"github.com/siderolabs/talos/pkg/provision/access"
)

type clusterMaker struct {
	// input fields
	options      Options
	provisioner  provision.Provisioner
	talosVersion string

	// fields available post init
	request         provision.ClusterRequest
	cidr4           netip.Prefix
	provisionOpts   []provision.Option
	cfgBundleOpts   []bundle.Option
	genOpts         []generate.Option
	ips             [][]netip.Addr
	versionContract *config.VersionContract

	// fields available post finalization
	inClusterEndpoint string
	configBundle      *bundle.Bundle
	bundleTalosconfig *clientconfig.Config
	cluster           provision.Cluster
}

// Input is the input options for clusterMaker.
type Input struct {
	Ops          Options
	Provisioner  provision.Provisioner
	TalosVersion string
}

// New initializes a new ClusterMaker.
func New(input Input) (ClusterMaker, error) {
	cm, err := newClusterMaker(input)

	return &cm, err
}

// newClusterMaker is used in tests.
func newClusterMaker(input Input) (clusterMaker, error) {
	cm := clusterMaker{}
	err := cm.init(input)

	return cm, err
}

func (cm *clusterMaker) GetPartialClusterRequest() PartialClusterRequest {
	return PartialClusterRequest(cm.request)
}

func (cm *clusterMaker) AddGenOps(opts ...generate.Option) {
	cm.genOpts = append(cm.genOpts, opts...)
}

func (cm *clusterMaker) AddProvisionOps(opts ...provision.Option) {
	cm.provisionOpts = append(cm.provisionOpts, opts...)
}

func (cm *clusterMaker) AddCfgBundleOpts(opts ...bundle.Option) {
	cm.cfgBundleOpts = append(cm.cfgBundleOpts, opts...)
}

func (cm *clusterMaker) SetInClusterEndpoint(endpoint string) {
	cm.inClusterEndpoint = endpoint
}

func (cm *clusterMaker) CreateCluster(ctx context.Context, request PartialClusterRequest) error {
	cm.request = provision.ClusterRequest(request)

	err := cm.finalizeRequest()
	if err != nil {
		return err
	}

	cluster, err := cm.provisioner.Create(ctx, cm.request, cm.provisionOpts...)
	if err != nil {
		return err
	}

	cm.cluster = cluster

	return nil
}

func (cm *clusterMaker) PostCreate(ctx context.Context) error {
	// No talosconfig in the bundle - skip the operations below
	if cm.bundleTalosconfig == nil {
		return nil
	}

	clusterAccess := access.NewAdapter(cm.cluster, cm.provisionOpts...)
	defer clusterAccess.Close() //nolint:errcheck

	if err := cm.applyConfigs(ctx, clusterAccess); err != nil {
		return err
	}

	if err := cm.bootstrapCluster(ctx, clusterAccess); err != nil {
		return err
	}

	return clustercmd.ShowCluster(cm.cluster)
}

func (cm *clusterMaker) GetCIDR4() netip.Prefix {
	return cm.cidr4
}

func (cm *clusterMaker) GetVersionContract() *config.VersionContract {
	return cm.versionContract
}

func (cm *clusterMaker) bootstrapCluster(ctx context.Context, clusterAccess *access.Adapter) error {
	if !cm.options.WithInitNode {
		if err := clusterAccess.Bootstrap(ctx, os.Stdout); err != nil {
			return fmt.Errorf("bootstrap error: %w", err)
		}
	}

	if !cm.options.ClusterWait {
		return nil
	}

	// Run cluster readiness checks
	checkCtx, checkCtxCancel := context.WithTimeout(ctx, cm.options.ClusterWaitTimeout)
	defer checkCtxCancel()

	checks := check.DefaultClusterChecks()

	if cm.options.SkipK8sNodeReadinessCheck {
		checks = slices.Concat(check.PreBootSequenceChecks(), check.K8sComponentsReadinessChecks())
	}

	checks = append(checks, check.ExtraClusterChecks()...)

	if err := check.Wait(checkCtx, clusterAccess, checks, check.StderrReporter()); err != nil {
		return err
	}

	if !cm.options.SkipKubeconfig {
		if err := mergeKubeconfig(ctx, clusterAccess); err != nil {
			return err
		}
	}

	return nil
}

func (cm *clusterMaker) applyConfigs(ctx context.Context, clusterAccess *access.Adapter) error {
	// Create and save the talosctl configuration file.
	if err := saveConfig(cm.bundleTalosconfig, cm.options); err != nil {
		return err
	}

	if cm.options.ApplyConfigEnabled {
		if err := clusterAccess.ApplyConfig(ctx, cm.request.Nodes, cm.request.SiderolinkRequest, os.Stdout); err != nil {
			return err
		}
	}

	return nil
}

func (cm *clusterMaker) init(input Input) error {
	cm.provisioner = input.Provisioner
	cm.options = input.Ops
	cm.talosVersion = input.TalosVersion

	if cm.options.TalosVersion != "latest" {
		versionContract, err := config.ParseContractFromVersion(cm.talosVersion)
		if err != nil {
			return fmt.Errorf("error parsing Talos version %q: %w", cm.options.TalosVersion, err)
		}

		cm.versionContract = versionContract
	}

	if err := cm.createPartialClusterRequest(); err != nil {
		return err
	}

	if err := cm.createNodeRequests(); err != nil {
		return err
	}

	if err := cm.initProvisionOpts(); err != nil {
		return err
	}

	if err := cm.initConfigBundleOpts(); err != nil {
		return err
	}

	if err := cm.initGenOps(); err != nil {
		return err
	}

	return nil
}

func (cm *clusterMaker) finalizeRequest() error {
	if cm.options.InputDir != "" {
		cm.AddCfgBundleOpts(bundle.WithExistingConfigs(cm.options.InputDir))
	} else {
		if cm.inClusterEndpoint == "" {
			cm.inClusterEndpoint = cm.provisioner.GetInClusterKubernetesControlPlaneEndpoint(cm.request.Network, cm.options.ControlPlanePort)
		}

		cm.AddCfgBundleOpts(bundle.WithInputOptions(
			&bundle.InputOptions{
				ClusterName: cm.options.RootOps.ClusterName,
				Endpoint:    cm.inClusterEndpoint,
				KubeVersion: strings.TrimPrefix(cm.options.KubernetesVersion, "v"),
				GenOptions:  cm.genOpts,
			}))
	}

	configBundle, bundleTalosconfig, err := cm.getConfigBundle()
	if err != nil {
		return err
	}

	cm.AddProvisionOps(provision.WithTalosConfig(configBundle.TalosConfig()))

	cm.bundleTalosconfig = bundleTalosconfig
	cm.configBundle = configBundle

	return cm.applyNodeCfgs()
}

func (cm *clusterMaker) getConfigBundle() (configBundle *bundle.Bundle, bundleTalosconfig *clientconfig.Config, err error) {
	configBundle, err = bundle.NewBundle(cm.cfgBundleOpts...)
	if err != nil {
		return nil, nil, err
	}

	bundleTalosconfig = configBundle.TalosConfig()
	if bundleTalosconfig == nil {
		if cm.options.ClusterWait {
			return nil, nil, errors.New("no talosconfig in the config bundle: cannot wait for cluster")
		}

		if cm.options.ApplyConfigEnabled {
			return nil, nil, errors.New("no talosconfig in the config bundle: cannot apply config")
		}
	}

	if cm.options.SkipInjectingConfig {
		types := []machine.Type{machine.TypeControlPlane, machine.TypeWorker}

		if cm.options.WithInitNode {
			types = slices.Insert(types, 0, machine.TypeInit)
		}

		if err = configBundle.Write(".", encoder.CommentsAll, types...); err != nil {
			return nil, nil, err
		}
	}

	return
}

func (cm *clusterMaker) initProvisionOpts() error {
	if cm.options.WithJSONLogs {
		cm.AddProvisionOps(provision.WithJSONLogs(nethelpers.JoinHostPort(cm.request.Network.GatewayAddrs[0].String(), jsonLogsPort)))
	}

	externalKubernetesEndpoint := cm.provisioner.GetExternalKubernetesControlPlaneEndpoint(cm.request.Network, cm.options.ControlPlanePort)
	cm.AddProvisionOps(provision.WithKubernetesEndpoint(externalKubernetesEndpoint))

	return nil
}

func (cm *clusterMaker) initConfigBundleOpts() error {
	addConfigPatch := func(configPatches []string, configOpt func([]configpatcher.Patch) bundle.Option) error {
		var patches []configpatcher.Patch

		patches, err := configpatcher.LoadPatches(configPatches)
		if err != nil {
			return fmt.Errorf("error parsing config JSON patch: %w", err)
		}

		cm.AddCfgBundleOpts(configOpt(patches))

		return nil
	}

	if err := addConfigPatch(cm.options.ConfigPatch, bundle.WithPatch); err != nil {
		return err
	}

	if err := addConfigPatch(cm.options.ConfigPatchControlPlane, bundle.WithPatchControlPlane); err != nil {
		return err
	}

	if err := addConfigPatch(cm.options.ConfigPatchWorker, bundle.WithPatchWorker); err != nil {
		return err
	}

	if cm.options.WithJSONLogs {
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
										Host:   nethelpers.JoinHostPort(cm.request.Network.GatewayAddrs[0].String(), jsonLogsPort),
									},
								},
								LoggingFormat: "json_lines",
							},
						},
					},
				},
			})
		cm.AddCfgBundleOpts(bundle.WithPatch([]configpatcher.Patch{configpatcher.NewStrategicMergePatch(cfg)}))
	}

	return nil
}

func (cm *clusterMaker) initGenOps() error {
	cm.AddGenOps(
		generate.WithDebug(cm.options.ConfigDebug),
		generate.WithDNSDomain(cm.options.DNSDomain),
		generate.WithClusterDiscovery(cm.options.EnableClusterDiscovery),
	)
	cm.AddGenOps(cm.provisioner.GenOptions(cm.request.Network)...)

	for _, registryMirror := range cm.options.RegistryMirrors {
		left, right, ok := strings.Cut(registryMirror, "=")
		if !ok {
			return fmt.Errorf("invalid registry mirror spec: %q", registryMirror)
		}

		cm.AddGenOps(generate.WithRegistryMirror(left, right))
	}

	for _, registryHost := range cm.options.RegistryInsecure {
		cm.AddGenOps(generate.WithRegistryInsecureSkipVerify(registryHost))
	}

	if cm.versionContract != nil {
		cm.AddGenOps(generate.WithVersionContract(cm.versionContract))
	}

	if cm.options.CustomCNIUrl != "" {
		cm.AddGenOps(generate.WithClusterCNIConfig(&v1alpha1.CNIConfig{
			CNIName: constants.CustomCNI,
			CNIUrls: []string{cm.options.CustomCNIUrl},
		}))
	}

	if cm.options.KubePrismPort != constants.DefaultKubePrismPort {
		cm.AddGenOps(generate.WithKubePrismPort(cm.options.KubePrismPort))
	}

	if cm.options.ControlPlanePort != constants.DefaultControlPlanePort {
		cm.AddGenOps(generate.WithLocalAPIServerPort(cm.options.ControlPlanePort))
	}

	if cm.options.EnableKubeSpan {
		cm.AddGenOps(generate.WithNetworkOptions(v1alpha1.WithKubeSpan()))
	}

	return cm.addEnpointListGenOption()
}

func (cm *clusterMaker) addEnpointListGenOption() error {
	endpointList := cm.provisioner.GetTalosAPIEndpoints(cm.request.Network)

	switch {
	case cm.options.ForceEndpoint != "":
		// using non-default endpoints, provision additional cert SANs and fix endpoint list
		endpointList = []string{cm.options.ForceEndpoint}
		cm.AddGenOps(generate.WithAdditionalSubjectAltNames(endpointList))
	case cm.options.ForceInitNodeAsEndpoint:
		endpointList = []string{cm.ips[0][0].String()}
	case len(endpointList) > 0:
		for _, endpointHostPort := range endpointList {
			endpointHost, _, err := net.SplitHostPort(endpointHostPort)
			if err != nil {
				endpointHost = endpointHostPort
			}

			cm.AddGenOps(generate.WithAdditionalSubjectAltNames([]string{endpointHost}))
		}
	case endpointList == nil:
		// use control plane nodes as endpoints, client-side load-balancing
		for i := range cm.options.Controlplanes {
			endpointList = append(endpointList, cm.ips[0][i].String())
		}
	}

	cm.AddGenOps(generate.WithEndpointList(endpointList))

	return nil
}

func (cm *clusterMaker) createPartialClusterRequest() error {
	cm.request = provision.ClusterRequest{
		Name:           cm.options.RootOps.ClusterName,
		SelfExecutable: os.Args[0],
		StateDirectory: cm.options.RootOps.StateDir,
	}

	if err := cm.initNetworkParams(); err != nil {
		return err
	}

	return cm.initNetworkParams()
}

func (cm *clusterMaker) createNodeRequests() error {
	if cm.options.Controlplanes < 1 {
		return errors.New("number of controlplanes can't be less than 1")
	}

	controlPlaneNanoCPUs, err := parseCPUShare(cm.options.ControlPlaneCpus)
	if err != nil {
		return fmt.Errorf("error parsing --cpus: %s", err)
	}

	workerNanoCPUs, err := parseCPUShare(cm.options.WorkersCpus)
	if err != nil {
		return fmt.Errorf("error parsing --cpus-workers: %s", err)
	}

	controlPlaneMemory := int64(cm.options.ControlPlaneMemory) * 1024 * 1024
	workerMemory := int64(cm.options.WorkersMemory) * 1024 * 1024

	for i := range cm.options.Controlplanes {
		machineType := machine.TypeControlPlane
		nodeIPs := getNodeIP(cm.request.Network.CIDRs, cm.ips, i)
		cm.request.Nodes = append(cm.request.Nodes, provision.NodeRequest{
			Name:                fmt.Sprintf("%s-%s-%d", cm.options.RootOps.ClusterName, "controlplane", i+1),
			IPs:                 nodeIPs,
			Type:                machineType,
			Memory:              controlPlaneMemory,
			NanoCPUs:            controlPlaneNanoCPUs,
			SkipInjectingConfig: cm.options.SkipInjectingConfig,
		})
	}

	for workerIndex := range cm.options.Workers {
		nodeIndex := cm.options.Controlplanes + workerIndex
		nodeIPs := getNodeIP(cm.request.Network.CIDRs, cm.ips, nodeIndex)
		cm.request.Nodes = append(cm.request.Nodes, provision.NodeRequest{
			Name:                fmt.Sprintf("%s-%s-%d", cm.options.RootOps.ClusterName, "worker", workerIndex+1),
			IPs:                 nodeIPs,
			Type:                machine.TypeWorker,
			Memory:              workerMemory,
			NanoCPUs:            workerNanoCPUs,
			SkipInjectingConfig: cm.options.SkipInjectingConfig,
		})
	}

	return nil
}

func (cm *clusterMaker) applyNodeCfgs() (err error) {
	var wireguardConfigBundle *helpers.WireguardConfigBundle
	if cm.options.WireguardCIDR != "" {
		wireguardConfigBundle, err = helpers.NewWireguardConfigBundle(cm.ips[0], cm.options.WireguardCIDR, 51111, cm.options.Controlplanes)
		if err != nil {
			return err
		}
	}

	for i, n := range cm.request.Nodes {
		cfg := cm.configBundle.ControlPlane()
		if n.Type == machine.TypeInit {
			cfg = cm.configBundle.Init()
		} else if n.Type == machine.TypeWorker {
			cfg = cm.configBundle.Worker()
		}

		cfg, err := patchWireguard(wireguardConfigBundle, cfg, n.IPs)
		if err != nil {
			return err
		}

		cm.request.Nodes[i].Config = cfg
	}

	return nil
}

func (cm *clusterMaker) initNetworkParams() error {
	cidr4, err := netip.ParsePrefix(cm.options.NetworkCIDR)
	if err != nil {
		return fmt.Errorf("error validating cidr block: %w", err)
	}

	if !cidr4.Addr().Is4() {
		return errors.New("--cidr is expected to be IPV4 CIDR")
	}

	cm.cidr4 = cidr4

	var cidrs []netip.Prefix
	if cm.options.NetworkIPv4 {
		cidrs = append(cidrs, cidr4)
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

	if cm.options.NetworkIPv6 {
		cidrs = append(cidrs, cidr6)
	}

	// Gateway addr at 1st IP in range, ex. 192.168.0.1
	gatewayIPs := make([]netip.Addr, len(cidrs))

	for j := range gatewayIPs {
		gatewayIPs[j], err = sideronet.NthIPInNetwork(cidrs[j], gatewayOffset)
		if err != nil {
			return err
		}
	}

	cm.request.Network = provision.NetworkRequest{
		Name:         cm.options.RootOps.ClusterName,
		CIDRs:        cidrs,
		GatewayAddrs: gatewayIPs,
		MTU:          cm.options.NetworkMTU,
	}

	return cm.initIps()
}

func (cm *clusterMaker) initIps() error {
	cidrs := cm.request.Network.CIDRs
	ips := make([][]netip.Addr, len(cidrs))

	for j := range cidrs {
		ips[j] = make([]netip.Addr, cm.options.Controlplanes+cm.options.Workers)

		for i := range ips[j] {
			ip, err := sideronet.NthIPInNetwork(cidrs[j], nodesOffset+i)
			if err != nil {
				return err
			}

			ips[j][i] = ip
		}
	}

	cm.ips = ips

	return nil
}
