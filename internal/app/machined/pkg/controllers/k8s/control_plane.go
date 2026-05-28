// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"context"
	"fmt"
	"net/netip"
	"slices"
	"strconv"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic"
	"github.com/cosi-project/runtime/pkg/controller/generic/transform"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-kubernetes/kubernetes/compatibility"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"

	"github.com/siderolabs/talos/pkg/argsbuilder"
	"github.com/siderolabs/talos/pkg/images"
	"github.com/siderolabs/talos/pkg/kubernetes"
	talosconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// controlplaneMapFunc is a shared "map" func for transform controller which guards on:
// * machine config is there
// * it has cluster & machine parts
// * machine is controlplane one.
func controlplaneMapFunc[Output generic.ResourceWithRD](output Output, extraChecks ...func(*config.MachineConfig) bool) func(cfg *config.MachineConfig) optional.Optional[Output] {
	return func(cfg *config.MachineConfig) optional.Optional[Output] {
		if cfg.Metadata().ID() != config.ActiveID {
			return optional.None[Output]()
		}

		if cfg.Config().Cluster() == nil || cfg.Config().Machine() == nil {
			return optional.None[Output]()
		}

		if !cfg.Config().Machine().Type().IsControlPlane() {
			return optional.None[Output]()
		}

		for _, check := range extraChecks {
			if !check(cfg) {
				return optional.None[Output]()
			}
		}

		return optional.Some(output)
	}
}

// ControlPlaneAdmissionControlController manages k8s.AdmissionControlConfig based on configuration.
type ControlPlaneAdmissionControlController = transform.Controller[*config.MachineConfig, *k8s.AdmissionControlConfig]

// NewControlPlaneAdmissionControlController instantiates the controller.
func NewControlPlaneAdmissionControlController() *ControlPlaneAdmissionControlController {
	return transform.NewController(
		transform.Settings[*config.MachineConfig, *k8s.AdmissionControlConfig]{
			Name:                    "k8s.ControlPlaneAdmissionControlController",
			MapMetadataOptionalFunc: controlplaneMapFunc(k8s.NewAdmissionControlConfig()),
			TransformFunc: func(ctx context.Context, r controller.Reader, logger *zap.Logger, machineConfig *config.MachineConfig, res *k8s.AdmissionControlConfig) error {
				cfgProvider := machineConfig.Config()

				res.TypedSpec().Config = nil

				for _, cfg := range cfgProvider.Cluster().APIServer().AdmissionControl() {
					res.TypedSpec().Config = append(
						res.TypedSpec().Config,
						k8s.AdmissionPluginSpec{
							Name:          cfg.Name(),
							Configuration: cfg.Configuration(),
						},
					)
				}

				return nil
			},
		},
	)
}

// ControlPlaneAuditPolicyController manages k8s.AuditPolicyConfig based on configuration.
type ControlPlaneAuditPolicyController = transform.Controller[*config.MachineConfig, *k8s.AuditPolicyConfig]

// NewControlPlaneAuditPolicyController instantiates the controller.
func NewControlPlaneAuditPolicyController() *ControlPlaneAuditPolicyController {
	return transform.NewController(
		transform.Settings[*config.MachineConfig, *k8s.AuditPolicyConfig]{
			Name:                    "k8s.ControlPlaneAuditPolicyController",
			MapMetadataOptionalFunc: controlplaneMapFunc(k8s.NewAuditPolicyConfig()),
			TransformFunc: func(ctx context.Context, r controller.Reader, logger *zap.Logger, machineConfig *config.MachineConfig, res *k8s.AuditPolicyConfig) error {
				cfgProvider := machineConfig.Config()

				res.TypedSpec().Config = cfgProvider.Cluster().APIServer().AuditPolicy()

				return nil
			},
		},
	)
}

// ControlPlaneAuthorizationController manages k8s.AuthorizationConfig based on configuration.
type ControlPlaneAuthorizationController = transform.Controller[*config.MachineConfig, *k8s.AuthorizationConfig]

// NewControlPlaneAuthorizationController instantiates the controller.
func NewControlPlaneAuthorizationController() *ControlPlaneAuthorizationController {
	return transform.NewController(
		transform.Settings[*config.MachineConfig, *k8s.AuthorizationConfig]{
			Name:                    "k8s.ControlPlaneAuthorizationPolicyController",
			MapMetadataOptionalFunc: controlplaneMapFunc(k8s.NewAuthorizationConfig()),
			TransformFunc: func(ctx context.Context, r controller.Reader, logger *zap.Logger, machineConfig *config.MachineConfig, res *k8s.AuthorizationConfig) error {
				cfgProvider := machineConfig.Config()

				res.TypedSpec().Image = cfgProvider.Cluster().APIServer().Image()

				if !compatibility.VersionFromImageRef(cfgProvider.Cluster().APIServer().Image()).KubeAPIServerSupportsAuthorizationConfigFile() {
					return nil
				}

				if cfgProvider.Cluster().APIServer().AuthorizationConfig() == nil {
					res.TypedSpec().Config = v1alpha1.APIServerDefaultAuthorizationConfigAuthorizers

					return nil
				}

				var authorizers []k8s.AuthorizationAuthorizersSpec

				for _, authorizer := range cfgProvider.Cluster().APIServer().AuthorizationConfig() {
					authorizers = slices.Concat(authorizers, []k8s.AuthorizationAuthorizersSpec{
						{
							Type:    authorizer.Type(),
							Name:    authorizer.Name(),
							Webhook: authorizer.Webhook(),
						},
					})
				}

				if !slices.ContainsFunc(authorizers, func(a k8s.AuthorizationAuthorizersSpec) bool {
					return a.Type == "Node"
				}) {
					authorizers = slices.Insert(authorizers, 0, k8s.AuthorizationAuthorizersSpec{
						Type: "Node",
						Name: "node",
					})
				}

				if !slices.ContainsFunc(authorizers, func(a k8s.AuthorizationAuthorizersSpec) bool {
					return a.Type == "RBAC"
				}) {
					authorizers = slices.Insert(authorizers, 1, k8s.AuthorizationAuthorizersSpec{
						Type: "RBAC",
						Name: "rbac",
					})
				}

				res.TypedSpec().Config = authorizers

				return nil
			},
		},
	)
}

// ControlPlaneAPIServerController manages k8s.APIServerConfig based on configuration.
type ControlPlaneAPIServerController = transform.Controller[*config.MachineConfig, *k8s.APIServerConfig]

// NewControlPlaneAPIServerController instantiates the controller.
func NewControlPlaneAPIServerController() *ControlPlaneAPIServerController {
	return transform.NewController(
		transform.Settings[*config.MachineConfig, *k8s.APIServerConfig]{
			Name: "k8s.ControlPlaneAPIServerController",
			MapMetadataOptionalFunc: controlplaneMapFunc(
				k8s.NewAPIServerConfig(),
				func(machineConfig *config.MachineConfig) bool {
					return machineConfig.Config().K8sNetworkConfig() != nil
				},
			),
			TransformFunc: func(ctx context.Context, r controller.Reader, logger *zap.Logger, machineConfig *config.MachineConfig, res *k8s.APIServerConfig) error {
				cfgProvider := machineConfig.Config()

				var cloudProvider string
				if cfgProvider.Cluster().ExternalCloudProvider().Enabled() {
					cloudProvider = CloudProviderExternal
				}

				advertisedAddress := "$(POD_IP)"
				if cfgProvider.Machine().Kubelet().SkipNodeRegistration() {
					advertisedAddress = ""
				}

				extraArgs := make(map[string]k8s.ArgValues, len(cfgProvider.Cluster().APIServer().ExtraArgs()))
				for k, v := range cfgProvider.Cluster().APIServer().ExtraArgs() {
					extraArgs[k] = k8s.ArgValues{Values: v}
				}

				*res.TypedSpec() = k8s.APIServerConfigSpec{
					Image:                cfgProvider.Cluster().APIServer().Image(),
					CloudProvider:        cloudProvider,
					ControlPlaneEndpoint: cfgProvider.Cluster().Endpoint().String(),
					EtcdServers:          []string{fmt.Sprintf("https://%s", nethelpers.JoinHostPort("127.0.0.1", constants.EtcdClientPort))},
					LocalPort:            cfgProvider.Cluster().LocalAPIServerPort(),
					ServiceCIDRs:         xslices.Map(cfgProvider.K8sNetworkConfig().ServiceCIDRs(), netip.Prefix.String),
					ExtraArgs:            extraArgs,
					ExtraVolumes:         convertVolumes(cfgProvider.Cluster().APIServer().ExtraVolumes()),
					EnvironmentVariables: cfgProvider.Cluster().APIServer().Env(),
					AdvertisedAddress:    advertisedAddress,
					Resources:            convertResources(cfgProvider.Cluster().APIServer().Resources()),
				}

				return nil
			},
		},
	)
}

// ControlPlaneControllerManagerController manages k8s.ControllerManagerConfig based on configuration.
type ControlPlaneControllerManagerController = transform.Controller[*config.MachineConfig, *k8s.ControllerManagerConfig]

// NewControlPlaneControllerManagerController instantiates the controller.
func NewControlPlaneControllerManagerController() *ControlPlaneControllerManagerController {
	return transform.NewController(
		transform.Settings[*config.MachineConfig, *k8s.ControllerManagerConfig]{
			Name: "k8s.ControlPlaneControllerManagerController",
			MapMetadataOptionalFunc: controlplaneMapFunc(
				k8s.NewControllerManagerConfig(k8s.ControllerManagerConfigID),
				func(machineConfig *config.MachineConfig) bool {
					return machineConfig.Config().K8sControllerManagerConfig() != nil
				},
				func(machineConfig *config.MachineConfig) bool {
					return machineConfig.Config().K8sNetworkConfig() != nil
				},
			),
			TransformFunc: func(ctx context.Context, r controller.Reader, logger *zap.Logger, machineConfig *config.MachineConfig, res *k8s.ControllerManagerConfig) error {
				controllerManagerConfig := machineConfig.Config().K8sControllerManagerConfig()

				var cloudProvider string

				if machineConfig.Config().Cluster().ExternalCloudProvider().Enabled() {
					cloudProvider = CloudProviderExternal
				}

				extraArgs := make(map[string]k8s.ArgValues, len(controllerManagerConfig.ExtraArgs()))
				for k, v := range controllerManagerConfig.ExtraArgs() {
					extraArgs[k] = k8s.ArgValues{Values: v}
				}

				*res.TypedSpec() = k8s.ControllerManagerConfigSpec{
					Enabled:              controllerManagerConfig.Enabled(),
					Image:                controllerManagerConfig.Image(),
					CloudProvider:        cloudProvider,
					PodCIDRs:             xslices.Map(machineConfig.Config().K8sNetworkConfig().PodCIDRs(), netip.Prefix.String),
					ServiceCIDRs:         xslices.Map(machineConfig.Config().K8sNetworkConfig().ServiceCIDRs(), netip.Prefix.String),
					ExtraArgs:            extraArgs,
					ExtraVolumes:         convertVolumes(controllerManagerConfig.ExtraVolumes()),
					EnvironmentVariables: controllerManagerConfig.Env(),
					Resources:            convertResources(controllerManagerConfig.Resources()),
				}

				return nil
			},
		},
		transform.WithOutputKind(controller.OutputShared),
	)
}

// ControlPlaneSchedulerController manages k8s.SchedulerConfig based on configuration.
type ControlPlaneSchedulerController = transform.Controller[*config.MachineConfig, *k8s.SchedulerConfig]

// NewControlPlaneSchedulerController instantiates the controller.
func NewControlPlaneSchedulerController() *ControlPlaneSchedulerController {
	return transform.NewController(
		transform.Settings[*config.MachineConfig, *k8s.SchedulerConfig]{
			Name: "k8s.ControlPlaneSchedulerController",
			MapMetadataOptionalFunc: controlplaneMapFunc(
				k8s.NewSchedulerConfig(k8s.SchedulerConfigID),
				func(machineConfig *config.MachineConfig) bool {
					return machineConfig.Config().K8sSchedulerConfig() != nil
				},
			),
			TransformFunc: func(ctx context.Context, r controller.Reader, logger *zap.Logger, machineConfig *config.MachineConfig, res *k8s.SchedulerConfig) error {
				schedulerConfig := machineConfig.Config().K8sSchedulerConfig()

				extraArgs := make(map[string]k8s.ArgValues, len(schedulerConfig.ExtraArgs()))
				for k, v := range schedulerConfig.ExtraArgs() {
					extraArgs[k] = k8s.ArgValues{Values: v}
				}

				*res.TypedSpec() = k8s.SchedulerConfigSpec{
					Enabled:              schedulerConfig.Enabled(),
					Image:                schedulerConfig.Image(),
					ExtraArgs:            extraArgs,
					ExtraVolumes:         convertVolumes(schedulerConfig.ExtraVolumes()),
					EnvironmentVariables: schedulerConfig.Env(),
					Resources:            convertResources(schedulerConfig.Resources()),
					Config:               schedulerConfig.Config(),
				}

				return nil
			},
		},
		transform.WithOutputKind(controller.OutputShared),
	)
}

// ControlPlaneBootstrapManifestsController manages k8s.BootstrapManifestsConfig based on configuration.
type ControlPlaneBootstrapManifestsController = transform.Controller[*config.MachineConfig, *k8s.BootstrapManifestsConfig]

// NewControlPlaneBootstrapManifestsController instantiates the controller.
//
//nolint:gocyclo
func NewControlPlaneBootstrapManifestsController() *ControlPlaneBootstrapManifestsController {
	return transform.NewController(
		transform.Settings[*config.MachineConfig, *k8s.BootstrapManifestsConfig]{
			Name: "k8s.ControlPlaneBootstrapManifestsController",
			MapMetadataOptionalFunc: controlplaneMapFunc(
				k8s.NewBootstrapManifestsConfig(),
				func(machineConfig *config.MachineConfig) bool {
					return machineConfig.Config().K8sNetworkConfig() != nil
				},
			),
			TransformFunc: func(ctx context.Context, r controller.Reader, logger *zap.Logger, machineConfig *config.MachineConfig, res *k8s.BootstrapManifestsConfig) error {
				cfgProvider := machineConfig.Config()

				dnsServiceIPs := k8s.DNSServiceAddrs(cfgProvider.K8sNetworkConfig().ServiceCIDRs())

				dnsServiceIP := ""
				dnsServiceIPv6 := ""

				for _, ip := range dnsServiceIPs {
					if dnsServiceIP == "" && ip.Is4() {
						dnsServiceIP = ip.String()
					}

					if dnsServiceIPv6 == "" && ip.Is6() {
						dnsServiceIPv6 = ip.String()
					}
				}

				images := images.List(cfgProvider)

				proxyArgs, err := getProxyArgs(cfgProvider)
				if err != nil {
					return err
				}

				var (
					server                                         string
					flannelKubeServiceHost, flannelKubeServicePort string
				)

				if cfgProvider.Machine().Features().KubePrism().Enabled() {
					server = fmt.Sprintf("https://127.0.0.1:%d", cfgProvider.Machine().Features().KubePrism().Port())
					flannelKubeServiceHost = "127.0.0.1"
					flannelKubeServicePort = strconv.Itoa(cfgProvider.Machine().Features().KubePrism().Port())
				} else {
					server = cfgProvider.Cluster().Endpoint().String()
				}

				*res.TypedSpec() = k8s.BootstrapManifestsConfigSpec{
					Server:        server,
					ClusterDomain: cfgProvider.K8sNetworkConfig().DNSDomain(),

					PodCIDRs: xslices.Map(cfgProvider.K8sNetworkConfig().PodCIDRs(), netip.Prefix.String),

					ProxyEnabled: cfgProvider.Cluster().Proxy().Enabled(),
					ProxyImage:   cfgProvider.Cluster().Proxy().Image(),
					ProxyArgs:    proxyArgs,

					CoreDNSEnabled: cfgProvider.Cluster().CoreDNS().Enabled(),
					CoreDNSImage:   cfgProvider.Cluster().CoreDNS().Image(),

					DNSServiceIP:   dnsServiceIP,
					DNSServiceIPv6: dnsServiceIPv6,

					TalosAPIServiceEnabled: cfgProvider.Machine().Features().KubernetesTalosAPIAccess().Enabled(),
				}

				if k8sFlannelCNIConfig := cfgProvider.K8sFlannelCNIConfig(); k8sFlannelCNIConfig != nil {
					res.TypedSpec().FlannelEnabled = true
					res.TypedSpec().FlannelImage = images.Flannel.String()
					res.TypedSpec().FlannelBackendType = k8sFlannelCNIConfig.BackendType()
					res.TypedSpec().FlannelBackendPort = k8sFlannelCNIConfig.BackendPort().ValueOrZero()

					configuredMTU := k8sFlannelCNIConfig.BackendMTU()

					// default Flannel MTU to KubeSpan MTU if KubeSpan is enabled
					if !configuredMTU.IsPresent() {
						if kubespanConfig := cfgProvider.NetworkKubeSpanConfig(); kubespanConfig != nil && kubespanConfig.Enabled() {
							configuredMTU = optional.Some(kubespanConfig.MTU())
						}
					}

					res.TypedSpec().FlannelBackendMTU = configuredMTU.ValueOrZero()
					res.TypedSpec().FlannelBackendExtraConfig = k8sFlannelCNIConfig.BackendExtraConfig()
					res.TypedSpec().FlannelResources = convertResources(k8sFlannelCNIConfig.Resources())
					res.TypedSpec().FlannelExtraArgs = k8sFlannelCNIConfig.ExtraArgs()
					res.TypedSpec().FlannelKubeServiceHost = flannelKubeServiceHost
					res.TypedSpec().FlannelKubeServicePort = flannelKubeServicePort
					res.TypedSpec().FlannelKubeNetworkPoliciesEnabled = k8sFlannelCNIConfig.KubeNetworkPoliciesEnabled()
					res.TypedSpec().FlannelKubeNetworkPoliciesImage = images.KubeNetworkPolicies.String()
					res.TypedSpec().CNIName = constants.FlannelCNI
				} else {
					res.TypedSpec().FlannelEnabled = false
					res.TypedSpec().FlannelImage = ""
					res.TypedSpec().FlannelBackendType = ""
					res.TypedSpec().FlannelBackendPort = 0
					res.TypedSpec().FlannelBackendMTU = 0
					res.TypedSpec().FlannelBackendExtraConfig = nil
					res.TypedSpec().FlannelResources = k8s.Resources{}
					res.TypedSpec().FlannelExtraArgs = nil
					res.TypedSpec().FlannelKubeServiceHost = ""
					res.TypedSpec().FlannelKubeServicePort = ""
					res.TypedSpec().FlannelKubeNetworkPoliciesEnabled = false
					res.TypedSpec().FlannelKubeNetworkPoliciesImage = ""
					res.TypedSpec().CNIName = constants.NoneCNI
				}

				return nil
			},
		},
		transform.WithExtraInputs(
			controller.Input{
				Namespace: network.NamespaceName,
				Type:      network.HostDNSConfigType,
				ID:        optional.Some(network.HostDNSConfigID),
				Kind:      controller.InputWeak,
			},
		),
	)
}

// ControlPlaneExtraManifestsController manages k8s.ExtraManifestsConfig based on configuration.
type ControlPlaneExtraManifestsController = transform.Controller[*config.MachineConfig, *k8s.ExtraManifestsConfig]

// NewControlPlaneExtraManifestsController instantiates the controller.
func NewControlPlaneExtraManifestsController() *ControlPlaneExtraManifestsController {
	return transform.NewController(
		transform.Settings[*config.MachineConfig, *k8s.ExtraManifestsConfig]{
			Name:                    "k8s.ControlPlaneExtraManifestsController",
			MapMetadataOptionalFunc: controlplaneMapFunc(k8s.NewExtraManifestsConfig()),
			TransformFunc: func(ctx context.Context, r controller.Reader, logger *zap.Logger, machineConfig *config.MachineConfig, res *k8s.ExtraManifestsConfig) error {
				cfgProvider := machineConfig.Config()

				spec := k8s.ExtraManifestsConfigSpec{}

				for _, url := range cfgProvider.Cluster().ExternalCloudProvider().ManifestURLs() {
					spec.ExtraManifests = append(spec.ExtraManifests, k8s.ExtraManifest{
						Name:     url,
						URL:      url,
						Priority: "30", // after default manifests
					})
				}

				for _, url := range cfgProvider.Cluster().ExtraManifestURLs() {
					spec.ExtraManifests = append(spec.ExtraManifests, k8s.ExtraManifest{
						Name:         url,
						URL:          url,
						Priority:     "99", // make sure extra manifests come last, when PSP is already created
						ExtraHeaders: cfgProvider.Cluster().ExtraManifestHeaderMap(),
					})
				}

				for _, manifest := range cfgProvider.Cluster().InlineManifests() {
					spec.ExtraManifests = append(spec.ExtraManifests, k8s.ExtraManifest{
						Name:           manifest.Name(),
						Priority:       "99", // make sure extra manifests come last, when PSP is already created
						InlineManifest: manifest.Contents(),
					})
				}

				*res.TypedSpec() = spec

				return nil
			},
		},
	)
}

func convertVolumes(volumes []talosconfig.VolumeMount) []k8s.ExtraVolume {
	return xslices.Map(volumes, func(v talosconfig.VolumeMount) k8s.ExtraVolume {
		return k8s.ExtraVolume{
			Name:      v.Name(),
			HostPath:  v.HostPath(),
			MountPath: v.MountPath(),
			ReadOnly:  v.ReadOnly(),
		}
	})
}

func convertResources(resources talosconfig.Resources) k8s.Resources {
	var convertedLimits map[string]string

	cpuLimits := resources.CPULimits()
	memoryLimits := resources.MemoryLimits()

	if cpuLimits != "" || memoryLimits != "" {
		convertedLimits = map[string]string{}

		if cpuLimits != "" {
			convertedLimits[string(v1.ResourceCPU)] = cpuLimits
		}

		if memoryLimits != "" {
			convertedLimits[string(v1.ResourceMemory)] = memoryLimits
		}
	}

	return k8s.Resources{
		Requests: map[string]string{
			string(v1.ResourceCPU):    resources.CPURequests(),
			string(v1.ResourceMemory): resources.MemoryRequests(),
		},
		Limits: convertedLimits,
	}
}

func getProxyArgs(cfgProvider talosconfig.Config) ([]string, error) {
	clusterCidr := strings.Join(xslices.Map(cfgProvider.K8sNetworkConfig().PodCIDRs(), netip.Prefix.String), ",")

	proxyMode := cfgProvider.Cluster().Proxy().Mode()

	if proxyMode == "" {
		// determine proxy mode based on kube-proxy version via the image, use 'nftables' for Kubernetes >= 1.31
		if kubernetes.VersionGTE(cfgProvider.Cluster().Proxy().Image(), semver.MustParse("1.31.0")) {
			proxyMode = "nftables"
		} else {
			proxyMode = "iptables"
		}
	}

	builder := argsbuilder.Args{
		"cluster-cidr":           {clusterCidr},
		"hostname-override":      {"$(NODE_NAME)"},
		"kubeconfig":             {"/etc/kubernetes/kubeconfig"},
		"proxy-mode":             {proxyMode},
		"conntrack-max-per-core": {"0"},
	}

	policies := argsbuilder.MergePolicies{
		"kubeconfig": argsbuilder.MergeDenied,
	}

	if err := builder.Merge(cfgProvider.Cluster().Proxy().ExtraArgs(), argsbuilder.WithMergePolicies(policies)); err != nil {
		return nil, err
	}

	return builder.Args(), nil
}
