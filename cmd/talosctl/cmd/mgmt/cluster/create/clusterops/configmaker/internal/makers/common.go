// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package makers

import (
	"errors"
	"fmt"
	"math/big"
	"net/netip"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-pointer"
	sideronet "github.com/siderolabs/net"

	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create/clusterops"
	"github.com/siderolabs/talos/cmd/talosctl/pkg/mgmt/helpers"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/bundle"
	"github.com/siderolabs/talos/pkg/machinery/config/configpatcher"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/generate"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/types/siderolink"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/provision"
)

const (
	// gatewayOffset is the offset from the network address of the IP address of the network gateway.
	gatewayOffset = 1 + iota

	// nodesOffset is the offset from the network address of the beginning of the IP addresses to be used for nodes.
	nodesOffset
)

// MakerOptions are the options needed to initialize a maker.
type MakerOptions[ExtraOps any] = struct {
	ExtraOps    ExtraOps
	CommonOps   clusterops.Common
	Provisioner provision.Provisioner
}

// New creates a new maker.
func New[ExtraOps any](options MakerOptions[ExtraOps]) (Maker[ExtraOps], error) {
	m := Maker[ExtraOps]{}
	m.Ops = options.CommonOps
	m.EOps = options.ExtraOps
	m.Provisioner = options.Provisioner

	return m, nil
}

// Maker contains config making logic shared between provisioners.
type Maker[ExtraOps any] struct {
	Ops clusterops.Common

	ClusterRequest    provision.ClusterRequest
	Provisioner       provision.Provisioner
	IPs               [][]netip.Addr
	VersionContract   *config.VersionContract
	GatewayIPs        []netip.Addr
	Cidrs             []netip.Prefix
	InClusterEndpoint string
	Endpoints         []string
	WithOmni          bool

	ProvisionOps    []provision.Option
	GenOps          []generate.Option
	ConfigBundleOps []bundle.Option

	EOps ExtraOps

	extraOptionsProvider ExtraOptionsProvider
}

// SetExtraOptionsProvider sets extra options provider containing the provider specific logic.
func (m *Maker[T]) SetExtraOptionsProvider(hooks ExtraOptionsProvider) {
	m.extraOptionsProvider = hooks
}

// Init initializes the common struct fields.
func (m *Maker[T]) Init() error {
	if err := m.InitCommon(); err != nil {
		return err
	}

	if err := m.InitExtra(); err != nil {
		return err
	}

	return nil
}

// InitExtra calls the init functions set by the individual implementation of the maker.
func (m *Maker[T]) InitExtra() error {
	if err := m.extraOptionsProvider.InitExtra(); err != nil {
		return err
	}

	// skip generating machine config if nodes are to be used with omni
	if !m.WithOmni {
		if err := m.extraOptionsProvider.AddExtraGenOps(); err != nil {
			return err
		}

		if err := m.extraOptionsProvider.AddExtraConfigBundleOpts(); err != nil {
			return err
		}
	}

	if err := m.extraOptionsProvider.AddExtraProvisionOpts(); err != nil {
		return err
	}

	if err := m.extraOptionsProvider.ModifyClusterRequest(); err != nil {
		return err
	}

	if err := m.extraOptionsProvider.ModifyNodes(); err != nil {
		return err
	}

	return nil
}

// InitCommon initializes the common fields.
//
//nolint:gocyclo
func (m *Maker[T]) InitCommon() error {
	if m.Ops.OmniAPIEndpoint != "" {
		m.WithOmni = true
	}

	if err := m.initVersionContract(); err != nil {
		return err
	}

	if err := m.initCIDRs(); err != nil {
		return err
	}

	if err := m.initIPs(); err != nil {
		return err
	}

	if err := m.initGatewayIPs(); err != nil {
		return err
	}

	m.initClusterRequest()

	if err := m.initEndpoints(); err != nil {
		return err
	}

	if err := m.initNodeRequests(); err != nil {
		return err
	}

	// skip generating machine config if nodes are to be used with omni
	if !m.WithOmni {
		if err := m.initGenOps(); err != nil {
			return err
		}

		if err := m.initConfigBundleOps(); err != nil {
			return err
		}
	}

	if err := m.initProvisionOps(); err != nil {
		return err
	}

	return nil
}

func (m *Maker[T]) initProvisionOps() error {
	m.ProvisionOps = []provision.Option{
		provision.WithKubernetesEndpoint(m.Provisioner.GetExternalKubernetesControlPlaneEndpoint(m.ClusterRequest.Network, m.Ops.ControlPlanePort)),
	}

	return nil
}

func (m *Maker[T]) initConfigBundleOps() error {
	configBundleOps := []bundle.Option{}

	configPatchBundleOps, err := getConfigPatchBundleOps(m.Ops)
	if err != nil {
		return err
	}

	configBundleOps = append(configBundleOps, configPatchBundleOps...)

	m.ConfigBundleOps = configBundleOps

	return nil
}

func (m *Maker[T]) initVersionContract() error {
	if m.Ops.TalosVersion == "latest" {
		m.VersionContract = nil

		return nil
	}

	versionContract, err := config.ParseContractFromVersion(m.Ops.TalosVersion)
	if err != nil {
		return fmt.Errorf("error parsing Talos version %q: %w", m.Ops.TalosVersion, err)
	}

	m.VersionContract = versionContract

	return nil
}

// GetClusterConfigs prepares and returns the cluster create request data. This method is ment to be called after the implemeting maker
// logic has been run.
func (m *Maker[T]) GetClusterConfigs() (clusterops.ClusterConfigs, error) {
	var configBundle *bundle.Bundle

	if !m.WithOmni {
		cfgBundle, err := m.finalizeMachineConfigs()
		if err != nil {
			return clusterops.ClusterConfigs{}, err
		}

		configBundle = cfgBundle
	} else {
		err := m.applyOmniConfigs()
		if err != nil {
			return clusterops.ClusterConfigs{}, err
		}
	}

	return clusterops.ClusterConfigs{
		ClusterRequest:   m.ClusterRequest,
		ProvisionOptions: m.ProvisionOps,
		ConfigBundle:     configBundle,
	}, nil
}

func (m *Maker[T]) applyOmniConfigs() error {
	cfg := siderolink.NewConfigV1Alpha1()

	parsedURL, err := url.Parse(m.Ops.OmniAPIEndpoint)
	if err != nil {
		return fmt.Errorf("error parsing omni api url: %w", err)
	}

	cfg.APIUrlConfig.URL = parsedURL

	mode, err := runtime.ParseMode(runtime.ModeMetal.String())
	if err != nil {
		return err
	}

	_, err = cfg.Validate(mode)
	if err != nil {
		return err
	}

	ctr, err := container.New(cfg)
	if err != nil {
		return err
	}

	m.ForEachNode(func(i int, node *provision.NodeRequest) {
		node.Config = ctr
		node.Name = m.Ops.RootOps.ClusterName + "-machine-" + strconv.Itoa(i+1)
	})

	return nil
}

func (m *Maker[T]) finalizeMachineConfigs() (*bundle.Bundle, error) {
	// These options needs to be generated after the implementing maker has made changes to the cluster request.
	provisionGenOps, provisionBundleOps := m.Provisioner.GenOptions(m.ClusterRequest.Network, m.VersionContract)
	m.GenOps = slices.Concat(m.GenOps, provisionGenOps)
	m.ConfigBundleOps = slices.Concat(m.ConfigBundleOps, provisionBundleOps)
	m.GenOps = slices.Concat(m.GenOps, []generate.Option{generate.WithEndpointList(m.Endpoints)})

	m.ConfigBundleOps = append(m.ConfigBundleOps,
		bundle.WithInputOptions(
			&bundle.InputOptions{
				ClusterName: m.Ops.RootOps.ClusterName,
				Endpoint:    m.InClusterEndpoint,
				KubeVersion: strings.TrimPrefix(m.Ops.KubernetesVersion, "v"),
				GenOptions:  m.GenOps,
			}),
	)

	configBundle, err := bundle.NewBundle(m.ConfigBundleOps...)
	if err != nil {
		return nil, err
	}

	if m.ClusterRequest.Nodes[0].Type == machine.TypeInit {
		m.ClusterRequest.Nodes[0].Config = configBundle.Init()
	}

	m.ForEachControlplaneNode(func(i, controlplaneIndex int, node *provision.NodeRequest) {
		node.Config = configBundle.ControlPlane()
	})
	m.ForEachWorkerNode(func(i, workerI int, node *provision.NodeRequest) {
		node.Config = configBundle.Worker()
	})

	if m.Ops.WireguardCIDR != "" {
		wireguardConfigBundle, err := helpers.NewWireguardConfigBundle(m.IPs[0], m.Ops.WireguardCIDR, 51111, m.Ops.Controlplanes)
		if err != nil {
			return nil, err
		}

		for i := range m.ClusterRequest.Nodes {
			node := &m.ClusterRequest.Nodes[i]

			patchedCfg, err := wireguardConfigBundle.PatchConfig(node.IPs[0], node.Config)
			if err != nil {
				return nil, err
			}

			node.Config = patchedCfg
		}
	}

	m.ProvisionOps = append(m.ProvisionOps, provision.WithTalosConfig(configBundle.TalosConfig()))

	return configBundle, nil
}

// ForEachNode iterates over all nodes allowing modification of each node.
func (m *Maker[T]) ForEachNode(fn func(i int, node *provision.NodeRequest)) {
	for i := range m.ClusterRequest.Nodes {
		fn(i, &m.ClusterRequest.Nodes[i])
	}
}

// ForEachWorkerNode iterates over all worker nodes allowing modification of each worker node.
func (m *Maker[T]) ForEachWorkerNode(fn func(i, workerI int, node *provision.NodeRequest)) {
	workerIndex := 0

	for i := range m.ClusterRequest.Nodes {
		if m.ClusterRequest.Nodes[i].Type != machine.TypeWorker {
			continue
		}

		fn(i, workerIndex, &m.ClusterRequest.Nodes[i])
		workerIndex++
	}
}

// ForEachControlplaneNode iterates over all controlplane nodes allowing modification of each controlplane node.
func (m *Maker[T]) ForEachControlplaneNode(fn func(i, controlplaneIndex int, node *provision.NodeRequest)) {
	controlplaneIndex := 0

	for i := range m.ClusterRequest.Nodes {
		if m.ClusterRequest.Nodes[i].Type != machine.TypeControlPlane {
			continue
		}

		fn(i, controlplaneIndex, &m.ClusterRequest.Nodes[i])
		controlplaneIndex++
	}
}

func (m *Maker[T]) initCIDRs() error {
	cidr4, err := netip.ParsePrefix(m.Ops.NetworkCIDR)
	if err != nil {
		return fmt.Errorf("error validating cidr block: %w", err)
	}

	if !cidr4.Addr().Is4() {
		return errors.New("IPV4 CIDR expected, got IPV6 CIDR")
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

	if m.Ops.NetworkIPv4 {
		cidrs = append(cidrs, cidr4)
	}

	if m.Ops.NetworkIPv6 {
		cidrs = append(cidrs, cidr6)
	}

	if len(cidrs) == 0 {
		return errors.New("neither IPv4 nor IPv6 network was enabled")
	}

	m.Cidrs = cidrs

	return nil
}

func (m *Maker[T]) initGenOps() error {
	genOptions := []generate.Option{
		generate.WithDNSDomain(m.Ops.DNSDomain),
		generate.WithClusterDiscovery(m.Ops.EnableClusterDiscovery),
		generate.WithDebug(m.Ops.ConfigDebug),
	}

	registryMirrorOps, err := getRegistryMirrorGenOps(m.Ops)
	if err != nil {
		return err
	}

	for _, registryHost := range m.Ops.RegistryInsecure {
		genOptions = append(genOptions, generate.WithRegistryInsecureSkipVerify(registryHost))
	}

	genOptions = append(genOptions, registryMirrorOps...)

	genOptions = append(genOptions, generate.WithVersionContract(m.VersionContract))

	if m.Ops.ControlPlanePort != constants.DefaultControlPlanePort {
		genOptions = slices.Concat(genOptions, []generate.Option{
			generate.WithLocalAPIServerPort(m.Ops.ControlPlanePort),
		})
	}

	if m.Ops.KubePrismPort != constants.DefaultKubePrismPort {
		genOptions = slices.Concat(genOptions, []generate.Option{
			generate.WithKubePrismPort(m.Ops.KubePrismPort),
		})
	}

	if m.Ops.EnableKubeSpan {
		genOptions = slices.Concat(genOptions,
			[]generate.Option{generate.WithNetworkOptions(
				v1alpha1.WithKubeSpan(),
			)},
		)
	}

	m.GenOps = genOptions

	return nil
}

func getConfigPatchBundleOps(cOps clusterops.Common) ([]bundle.Option, error) {
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
	if err := addConfigPatch(cOps.ConfigPatch, bundle.WithPatch); err != nil {
		return nil, err
	}

	if err := addConfigPatch(cOps.ConfigPatchControlPlane, bundle.WithPatchControlPlane); err != nil {
		return nil, err
	}

	if err := addConfigPatch(cOps.ConfigPatchWorker, bundle.WithPatchWorker); err != nil {
		return nil, err
	}

	return configBundleOpts, nil
}

func (m *Maker[T]) initIPs() error {
	nodes := m.Ops.Controlplanes + m.Ops.Workers
	ips := make([][]netip.Addr, len(m.Cidrs))

	for cidrIndex, cidr := range m.Cidrs {
		for nodeIndex := range nodes {
			ip, err := sideronet.NthIPInNetwork(cidr, nodesOffset+nodeIndex)
			if err != nil {
				return err
			}

			ips[cidrIndex] = append(ips[cidrIndex], ip)
		}
	}

	m.IPs = ips

	return nil
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

func getRegistryMirrorGenOps(cOps clusterops.Common) ([]generate.Option, error) {
	ops := make([]generate.Option, 0, len(cOps.RegistryMirrors))

	for _, registryMirror := range cOps.RegistryMirrors {
		left, right, ok := strings.Cut(registryMirror, "=")
		if !ok {
			return nil, fmt.Errorf("invalid registry mirror spec: %q", registryMirror)
		}

		ops = append(ops, generate.WithRegistryMirror(left, right))
	}

	return ops, nil
}

func (m *Maker[T]) initClusterRequest() {
	m.ClusterRequest = provision.ClusterRequest{
		Name:           m.Ops.RootOps.ClusterName,
		SelfExecutable: os.Args[0],
		StateDirectory: m.Ops.RootOps.StateDir,

		Network: provision.NetworkRequest{
			Name:              m.Ops.RootOps.ClusterName,
			CIDRs:             m.Cidrs,
			GatewayAddrs:      m.GatewayIPs,
			MTU:               m.Ops.NetworkMTU,
			LoadBalancerPorts: []int{m.Ops.ControlPlanePort},
		},
	}
}

func (m *Maker[T]) initGatewayIPs() error {
	gatewayIPs := make([]netip.Addr, len(m.Cidrs))

	for j := range gatewayIPs {
		ip, err := sideronet.NthIPInNetwork(m.Cidrs[j], gatewayOffset)
		if err != nil {
			return err
		}

		gatewayIPs[j] = ip
	}

	m.GatewayIPs = gatewayIPs

	return nil
}

func (m *Maker[T]) initEndpoints() error {
	m.InClusterEndpoint = m.Provisioner.GetInClusterKubernetesControlPlaneEndpoint(m.ClusterRequest.Network, m.Ops.ControlPlanePort)
	m.Endpoints = m.Provisioner.GetTalosAPIEndpoints(m.ClusterRequest.Network)

	return nil
}

func parseResources(res clusterops.NodeResources) (clusterops.ParsedNodeResources, error) {
	nanoCPUs, err := parseCPUShare(res.CPU)
	if err != nil {
		return clusterops.ParsedNodeResources{}, err
	}

	return clusterops.ParsedNodeResources{
		NanoCPUs: nanoCPUs,
		Memory:   res.Memory,
	}, nil
}

func (m *Maker[T]) initNodeRequests() error {
	controlplaneResources, err := parseResources(m.Ops.ControlplaneResources)
	if err != nil {
		return fmt.Errorf("error parsing controlplane resources: %s", err)
	}

	workerResources, err := parseResources(m.Ops.WorkerResources)
	if err != nil {
		return fmt.Errorf("error parsing worker resources: %s", err)
	}

	if m.Ops.Controlplanes < 1 {
		return errors.New("number of controlplanes can't be less than 1")
	}

	nodes := []provision.NodeRequest{}

	for i := range m.Ops.Controlplanes {
		nodeUUID := uuid.New()
		machineName := fmt.Sprintf("%s-%s-%d", m.Ops.RootOps.ClusterName, "controlplane", i+1)

		if m.Ops.WithUUIDHostnames {
			machineName = fmt.Sprintf("%s-%s", "machine", nodeUUID)
		}

		machineType := machine.TypeControlPlane

		if m.Ops.WithInitNode && i == 0 {
			machineType = machine.TypeInit
		}

		ips := getNodeIPs(m.IPs, i)
		nodes = append(nodes, provision.NodeRequest{
			Name:                machineName,
			IPs:                 ips,
			Type:                machineType,
			Memory:              int64(controlplaneResources.Memory.Bytes()),
			NanoCPUs:            controlplaneResources.NanoCPUs,
			UUID:                pointer.To(nodeUUID),
			SkipInjectingConfig: m.Ops.SkipInjectingConfig,
		})
	}

	for workerIndex := range m.Ops.Workers {
		nodeUUID := uuid.New()
		machineName := fmt.Sprintf("%s-%s-%d", m.Ops.RootOps.ClusterName, "worker", workerIndex+1)

		if m.Ops.WithUUIDHostnames {
			machineName = fmt.Sprintf("%s-%s", "machine", nodeUUID)
		}

		nodeIndex := m.Ops.Controlplanes + workerIndex
		ips := getNodeIPs(m.IPs, nodeIndex)
		nodes = append(nodes, provision.NodeRequest{
			Name:                machineName,
			IPs:                 ips,
			Type:                machine.TypeWorker,
			Memory:              int64(workerResources.Memory.Bytes()),
			NanoCPUs:            workerResources.NanoCPUs,
			UUID:                pointer.To(nodeUUID),
			SkipInjectingConfig: m.Ops.SkipInjectingConfig,
		})
	}

	m.ClusterRequest.Nodes = nodes

	return nil
}

func getNodeIPs(ips [][]netip.Addr, nodeIndex int) []netip.Addr {
	return xslices.Map(ips, func(ips []netip.Addr) netip.Addr {
		return ips[nodeIndex]
	})
}
