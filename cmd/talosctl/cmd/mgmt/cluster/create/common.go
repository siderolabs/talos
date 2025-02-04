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
	"slices"
	"strings"

	"github.com/siderolabs/go-kubeconfig"
	sideronet "github.com/siderolabs/net"
	"k8s.io/client-go/tools/clientcmd"

	clustercmd "github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster"
	"github.com/siderolabs/talos/cmd/talosctl/pkg/mgmt/helpers"
	clusterpkg "github.com/siderolabs/talos/pkg/cluster"
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

const (
	// gatewayOffset is the offset from the network address of the IP address of the network gateway.
	gatewayOffset = 1

	// nodesOffset is the offset from the network address of the beginning of the IP addresses to be used for nodes.
	nodesOffset  = 2
	jsonLogsPort = 4003
)

type clusterCreateBase = struct {
	clusterRequest    provision.ClusterRequestBase
	provisionOptions  []provision.Option
	cidr4             netip.Prefix
	ips               [][]netip.Addr
	verionContract    *config.VersionContract
	bundleTalosconfig *clientconfig.Config
}

type getTalosVersion = func() string

type additionalOptions = struct {
	genOpts       []generate.Option
	provisionOpts []provision.Option
	cfgBundleOpts []bundle.Option

	// can be optionally set to rewrite the default inClusterEndpoint that's received from the provider
	inClusterEndpoint string
}
type additionalOptsGetter = func(cOps CommonOps, base clusterCreateBase) (additional additionalOptions, err error)

// provider specific options.
func getBase(
	cOps CommonOps, // asd
	provisioner provision.Provisioner,
	getTalosVersion getTalosVersion,
	getadditionalOptions additionalOptsGetter,
) (clusterCreateBase, error) {
	result, err := _getBase(cOps, provisioner, getTalosVersion, getadditionalOptions)

	return result.base, err
}

type _getBaseResult = struct {
	base             clusterCreateBase
	genOptions       []generate.Option
	configBundle     *bundle.Bundle
	configBundleOpts []bundle.Option
}

// _getBase is used for tests.
//
//nolint:gocyclo
func _getBase(
	cOps CommonOps,
	provisioner provision.Provisioner,
	getTalosVersion getTalosVersion,
	getadditionalOptions additionalOptsGetter,
) (_getBaseResult, error) {
	cOps.TalosVersion = getTalosVersion()

	var configBundleOpts []bundle.Option

	networkRequestBase, cidr4, err := getNetworkRequestBase(cOps)
	if err != nil {
		return _getBaseResult{}, err
	}

	ips, err := getIps(networkRequestBase.CIDRs, cOps)
	if err != nil {
		return _getBaseResult{}, err
	}

	baseRequest := provision.ClusterRequestBase{
		Name:           cOps.RootOps.ClusterName,
		SelfExecutable: os.Args[0],
		StateDirectory: cOps.RootOps.StateDir,
		Network:        networkRequestBase,
	}

	provisionOptions := []provision.Option{}
	if cOps.WithJSONLogs {
		provisionOptions = append(provisionOptions, provision.WithJSONLogs(nethelpers.JoinHostPort(networkRequestBase.GatewayAddrs[0].String(), jsonLogsPort)))
	}

	externalKubernetesEndpoint := provisioner.GetExternalKubernetesControlPlaneEndpoint(networkRequestBase, cOps.ControlPlanePort)
	provisionOptions = append(provisionOptions, provision.WithKubernetesEndpoint(externalKubernetesEndpoint))

	genOptions, versionContract, err := getCommonGenOptions(cOps, provisioner, ips, networkRequestBase)
	if err != nil {
		return _getBaseResult{}, err
	}

	configBundleOpts, err = getCommonConfigBundleBaseOps(cOps, networkRequestBase.GatewayAddrs[0].String())
	if err != nil {
		return _getBaseResult{}, err
	}

	base := clusterCreateBase{
		clusterRequest:   baseRequest,
		provisionOptions: provisionOptions,
		cidr4:            cidr4,
		ips:              ips,
		verionContract:   versionContract,
	}

	additional, err := getadditionalOptions(cOps, base)
	if err != nil {
		return _getBaseResult{}, err
	}

	configBundleOpts = append(configBundleOpts, additional.cfgBundleOpts...)
	genOptions = append(genOptions, additional.genOpts...)
	base.provisionOptions = append(base.provisionOptions, additional.provisionOpts...)

	var inClusterEndpoint string

	if cOps.InputDir != "" {
		configBundleOpts = append(configBundleOpts, bundle.WithExistingConfigs(cOps.InputDir))
	} else {
		inClusterEndpoint = additional.inClusterEndpoint
		if inClusterEndpoint == "" {
			inClusterEndpoint = provisioner.GetInClusterKubernetesControlPlaneEndpoint(networkRequestBase, cOps.ControlPlanePort)
		}

		configBundleOpts = append(configBundleOpts, getConfigBudnleInputOption(cOps, genOptions, inClusterEndpoint))
	}

	configBundle, bundleTalosconfig, err := getConfigBundle(cOps, configBundleOpts)
	if err != nil {
		return _getBaseResult{}, err
	}

	base.provisionOptions = append(base.provisionOptions, provision.WithTalosConfig(configBundle.TalosConfig()))

	controlplanes, workers, err := getBaseNodeRequests(cOps, configBundle, networkRequestBase.CIDRs, ips)
	if err != nil {
		return _getBaseResult{}, err
	}

	base.clusterRequest.Workers = workers
	base.clusterRequest.Controlplanes = controlplanes
	base.bundleTalosconfig = bundleTalosconfig

	return _getBaseResult{base, genOptions, configBundle, configBundleOpts}, nil
}

// getNetworkRequestBase validates network related ops and creates a base network request.
func getNetworkRequestBase(cOps CommonOps) (req provision.NetworkRequestBase, cidr4 netip.Prefix, err error) {
	cidr4, err = netip.ParsePrefix(cOps.NetworkCIDR)
	if err != nil {
		return req, cidr4, fmt.Errorf("error validating cidr block: %w", err)
	}

	if !cidr4.Addr().Is4() {
		return req, cidr4, errors.New("--cidr is expected to be IPV4 CIDR")
	}

	var cidrs []netip.Prefix
	if cOps.NetworkIPv4 {
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
		return req, cidr4, fmt.Errorf("error validating cidr IPv6 block: %w", err)
	}

	if cOps.NetworkIPv6 {
		cidrs = append(cidrs, cidr6)
	}

	// Gateway addr at 1st IP in range, ex. 192.168.0.1
	gatewayIPs := make([]netip.Addr, len(cidrs))

	for j := range gatewayIPs {
		gatewayIPs[j], err = sideronet.NthIPInNetwork(cidrs[j], gatewayOffset)
		if err != nil {
			return req, cidr4, err
		}
	}

	return provision.NetworkRequestBase{
		Name:         cOps.RootOps.ClusterName,
		CIDRs:        cidrs,
		GatewayAddrs: gatewayIPs,
		MTU:          cOps.NetworkMTU,
	}, cidr4, nil
}

//nolint:gocyclo
func getBaseNodeRequests(cOps CommonOps, configBundle *bundle.Bundle, cidrs []netip.Prefix, ips [][]netip.Addr) (controls, workers provision.BaseNodeRequests, err error) {
	if cOps.Controlplanes < 1 {
		return controls, workers, errors.New("number of controlplanes can't be less than 1")
	}

	controlPlaneNanoCPUs, err := parseCPUShare(cOps.ControlPlaneCpus)
	if err != nil {
		return controls, workers, fmt.Errorf("error parsing --cpus: %s", err)
	}

	workerNanoCPUs, err := parseCPUShare(cOps.WorkersCpus)
	if err != nil {
		return controls, workers, fmt.Errorf("error parsing --cpus-workers: %s", err)
	}

	controlPlaneMemory := int64(cOps.ControlPlaneMemory) * 1024 * 1024
	workerMemory := int64(cOps.WorkersMemory) * 1024 * 1024

	var wireguardConfigBundle *helpers.WireguardConfigBundle
	if cOps.WireguardCIDR != "" {
		wireguardConfigBundle, err = helpers.NewWireguardConfigBundle(ips[0], cOps.WireguardCIDR, 51111, cOps.Controlplanes)
		if err != nil {
			return controls, workers, err
		}
	}

	for i := range cOps.Controlplanes {
		cfg := configBundle.ControlPlane()
		machineType := machine.TypeControlPlane

		if cOps.WithInitNode && i == 0 {
			cfg = configBundle.InitCfg
			machineType = machine.TypeInit
		}

		nodeIPs := getNodeIP(cidrs, ips, i)

		cfg, err = patchWireguard(wireguardConfigBundle, cfg, nodeIPs)
		if err != nil {
			return controls, workers, err
		}

		controls = append(controls, provision.NodeRequestBase{
			Index:               i,
			Name:                fmt.Sprintf("%s-%s-%d", cOps.RootOps.ClusterName, "controlplane", i+1),
			IPs:                 nodeIPs,
			Type:                machineType,
			Memory:              controlPlaneMemory,
			NanoCPUs:            controlPlaneNanoCPUs,
			SkipInjectingConfig: cOps.SkipInjectingConfig,
			Config:              cfg,
		})
	}

	for i := range cOps.Workers {
		cfg := configBundle.Worker()
		nodeIndex := cOps.Controlplanes + i
		nodeIPs := getNodeIP(cidrs, ips, nodeIndex)

		cfg, err = patchWireguard(wireguardConfigBundle, cfg, nodeIPs)
		if err != nil {
			return controls, workers, err
		}

		workers = append(workers, provision.NodeRequestBase{
			Index:               nodeIndex,
			Name:                fmt.Sprintf("%s-%s-%d", cOps.RootOps.ClusterName, "worker", i+1),
			IPs:                 nodeIPs,
			Type:                machine.TypeWorker,
			Memory:              workerMemory,
			NanoCPUs:            workerNanoCPUs,
			SkipInjectingConfig: cOps.SkipInjectingConfig,
			Config:              cfg,
		})
	}

	return controls, workers, nil
}

func patchWireguard(wireguardConfigBundle *helpers.WireguardConfigBundle, cfg config.Provider, nodeIPs []netip.Addr) (config.Provider, error) {
	if wireguardConfigBundle != nil {
		return wireguardConfigBundle.PatchConfig(nodeIPs[0], cfg)
	}

	return cfg, nil
}

func getConfigBudnleInputOption(cOps CommonOps, genOptions []generate.Option, inClusterEndpoint string) bundle.Option {
	return bundle.WithInputOptions(
		&bundle.InputOptions{
			ClusterName: cOps.RootOps.ClusterName,
			Endpoint:    inClusterEndpoint,
			KubeVersion: strings.TrimPrefix(cOps.KubernetesVersion, "v"),
			GenOptions:  genOptions,
		})
}

//nolint:gocyclo
func postCreate(
	ctx context.Context,
	cOps CommonOps,
	bundleTalosconfig *clientconfig.Config,
	cluster provision.Cluster,
	provisionOptions []provision.Option,
	nodeApplyCfgs []clusterpkg.NodeApplyConfig,
) error {
	_postCreate := func() error {
		// No talosconfig in the bundle - skip the operations below
		if bundleTalosconfig == nil {
			return nil
		}

		clusterAccess := access.NewAdapter(cluster, provisionOptions...)
		defer clusterAccess.Close() //nolint:errcheck

		// Create and save the talosctl configuration file.
		if err := saveConfig(bundleTalosconfig, cOps); err != nil {
			return err
		}

		if cOps.ApplyConfigEnabled {
			if err := clusterAccess.ApplyConfig(ctx, nodeApplyCfgs, nil, os.Stdout); err != nil {
				return err
			}
		}

		if !cOps.WithInitNode {
			if err := clusterAccess.Bootstrap(ctx, os.Stdout); err != nil {
				return fmt.Errorf("bootstrap error: %w", err)
			}
		}

		if !cOps.ClusterWait {
			return nil
		}

		// Run cluster readiness checks
		checkCtx, checkCtxCancel := context.WithTimeout(ctx, cOps.ClusterWaitTimeout)
		defer checkCtxCancel()

		checks := check.DefaultClusterChecks()

		if cOps.SkipK8sNodeReadinessCheck {
			checks = slices.Concat(check.PreBootSequenceChecks(), check.K8sComponentsReadinessChecks())
		}

		checks = append(checks, check.ExtraClusterChecks()...)

		if err := check.Wait(checkCtx, clusterAccess, checks, check.StderrReporter()); err != nil {
			return err
		}

		if !cOps.SkipKubeconfig {
			if err := mergeKubeconfig(ctx, clusterAccess); err != nil {
				return err
			}
		}

		return nil
	}

	if err := _postCreate(); err != nil {
		return err
	}

	return clustercmd.ShowCluster(cluster)
}

func saveConfig(talosConfigObj *clientconfig.Config, commonOps CommonOps) (err error) {
	c, err := clientconfig.Open(commonOps.Talosconfig)
	if err != nil {
		return fmt.Errorf("error opening talos config: %w", err)
	}

	renames := c.Merge(talosConfigObj)
	for _, rename := range renames {
		fmt.Fprintf(os.Stderr, "renamed talosconfig context %s\n", rename.String())
	}

	return c.Save(commonOps.Talosconfig)
}

func mergeKubeconfig(ctx context.Context, clusterAccess *access.Adapter) error {
	kubeconfigPath, err := kubeconfig.DefaultPath()
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

// getIps calculates ips for nodes and the virtual ip.
func getIps(cidrs []netip.Prefix, commonOps CommonOps) (ips [][]netip.Addr, err error) {
	// Set starting ip at 2nd ip in range, ex: 192.168.0.2
	ips = make([][]netip.Addr, len(cidrs))

	for j := range cidrs {
		ips[j] = make([]netip.Addr, commonOps.Controlplanes+commonOps.Workers)

		for i := range ips[j] {
			ips[j][i], err = sideronet.NthIPInNetwork(cidrs[j], nodesOffset+i)
			if err != nil {
				return ips, err
			}
		}
	}

	return ips, err
}

func getCommonGenOptions(cOps CommonOps, provisioner provision.Provisioner, ips [][]netip.Addr, networkReq provision.NetworkRequestBase) ([]generate.Option, *config.VersionContract, error) {
	genOptions := []generate.Option{
		generate.WithDebug(cOps.ConfigDebug),
		generate.WithDNSDomain(cOps.DNSDomain),
		generate.WithClusterDiscovery(cOps.EnableClusterDiscovery),
	}
	genOptions = append(genOptions, provisioner.GenOptions(networkReq)...)

	for _, registryMirror := range cOps.RegistryMirrors {
		left, right, ok := strings.Cut(registryMirror, "=")
		if !ok {
			return genOptions, nil, fmt.Errorf("invalid registry mirror spec: %q", registryMirror)
		}

		genOptions = append(genOptions, generate.WithRegistryMirror(left, right))
	}

	for _, registryHost := range cOps.RegistryInsecure {
		genOptions = append(genOptions, generate.WithRegistryInsecureSkipVerify(registryHost))
	}

	if cOps.CustomCNIUrl != "" {
		genOptions = append(genOptions, generate.WithClusterCNIConfig(&v1alpha1.CNIConfig{
			CNIName: constants.CustomCNI,
			CNIUrls: []string{cOps.CustomCNIUrl},
		}))
	}

	var versionContract *config.VersionContract
	if cOps.TalosVersion != "latest" {
		versionContract, err := config.ParseContractFromVersion(cOps.TalosVersion)
		if err != nil {
			return genOptions, nil, fmt.Errorf("error parsing Talos version %q: %w", cOps.TalosVersion, err)
		}

		genOptions = append(genOptions, generate.WithVersionContract(versionContract))
	}

	if cOps.KubePrismPort != constants.DefaultKubePrismPort {
		genOptions = append(genOptions,
			generate.WithKubePrismPort(cOps.KubePrismPort),
		)
	}

	if cOps.ControlPlanePort != constants.DefaultControlPlanePort {
		genOptions = append(genOptions,
			generate.WithLocalAPIServerPort(cOps.ControlPlanePort),
		)
	}

	if cOps.EnableKubeSpan {
		genOptions = append(genOptions,
			generate.WithNetworkOptions(
				v1alpha1.WithKubeSpan(),
			),
		)
	}

	endpointList := provisioner.GetTalosAPIEndpoints(networkReq)
	genOptions = append(genOptions, getEnpointListGenOption(cOps, endpointList, ips)...)

	return genOptions, versionContract, nil
}

func getEnpointListGenOption(cOps CommonOps, endpointList []string, ips [][]netip.Addr) []generate.Option {
	genOptions := []generate.Option{}

	switch {
	case cOps.ForceEndpoint != "":
		// using non-default endpoints, provision additional cert SANs and fix endpoint list
		endpointList = []string{cOps.ForceEndpoint}
		genOptions = append(genOptions, generate.WithAdditionalSubjectAltNames(endpointList))
	case cOps.ForceInitNodeAsEndpoint:
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
		for i := range cOps.Controlplanes {
			endpointList = append(endpointList, ips[0][i].String())
		}
	}

	return append(genOptions, generate.WithEndpointList(endpointList))
}

// getCommonConfigBundleBaseOps returns config bundle options that are applicable even if a config file is set.
func getCommonConfigBundleBaseOps(cOps CommonOps, gatewayIP string) ([]bundle.Option, error) {
	var configBundleOpts []bundle.Option

	addConfigPatch := func(configPatches []string, configOpt func([]configpatcher.Patch) bundle.Option) error {
		var patches []configpatcher.Patch

		patches, err := configpatcher.LoadPatches(configPatches)
		if err != nil {
			return fmt.Errorf("error parsing config JSON patch: %w", err)
		}

		configBundleOpts = append(configBundleOpts, configOpt(patches))

		return nil
	}

	if err := addConfigPatch(cOps.ConfigPatch, bundle.WithPatch); err != nil {
		return configBundleOpts, err
	}

	if err := addConfigPatch(cOps.ConfigPatchControlPlane, bundle.WithPatchControlPlane); err != nil {
		return configBundleOpts, err
	}

	if err := addConfigPatch(cOps.ConfigPatchWorker, bundle.WithPatchWorker); err != nil {
		return configBundleOpts, err
	}

	if cOps.WithJSONLogs {
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
										Host:   nethelpers.JoinHostPort(gatewayIP, jsonLogsPort),
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

	return configBundleOpts, nil
}

func getConfigBundle(cOps CommonOps, configBundleOpts []bundle.Option) (configBundle *bundle.Bundle, bundleTalosconfig *clientconfig.Config, err error) {
	configBundle, err = bundle.NewBundle(configBundleOpts...)
	if err != nil {
		return nil, nil, err
	}

	bundleTalosconfig = configBundle.TalosConfig()
	if bundleTalosconfig == nil {
		if cOps.ClusterWait {
			return nil, nil, errors.New("no talosconfig in the config bundle: cannot wait for cluster")
		}

		if cOps.ApplyConfigEnabled {
			return nil, nil, errors.New("no talosconfig in the config bundle: cannot apply config")
		}
	}

	if cOps.SkipInjectingConfig {
		types := []machine.Type{machine.TypeControlPlane, machine.TypeWorker}

		if cOps.WithInitNode {
			types = slices.Insert(types, 0, machine.TypeInit)
		}

		if err = configBundle.Write(".", encoder.CommentsAll, types...); err != nil {
			return nil, nil, err
		}
	}

	return
}

func getNodeIP(cidrs []netip.Prefix, ips [][]netip.Addr, nodeIndex int) []netip.Addr {
	nodeIPs := make([]netip.Addr, len(cidrs))
	for j := range nodeIPs {
		nodeIPs[j] = ips[j][nodeIndex]
	}

	return nodeIPs
}
