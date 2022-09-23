// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import (
	"context"
	"fmt"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/slices"
	"github.com/siderolabs/go-pointer"
	talosnet "github.com/talos-systems/net"
	"go.uber.org/zap"

	"github.com/talos-systems/talos/pkg/argsbuilder"
	"github.com/talos-systems/talos/pkg/images"
	talosconfig "github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/machinery/nethelpers"
	"github.com/talos-systems/talos/pkg/machinery/resources/config"
	"github.com/talos-systems/talos/pkg/machinery/resources/k8s"
)

// K8sControlPlaneController manages Kubernetes control plane resources based on configuration.
type K8sControlPlaneController struct{}

// Name implements controller.Controller interface.
func (ctrl *K8sControlPlaneController) Name() string {
	return "config.K8sControlPlaneController"
}

// Inputs implements controller.Controller interface.
func (ctrl *K8sControlPlaneController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        pointer.To(config.V1Alpha1ID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineTypeType,
			ID:        pointer.To(config.MachineTypeID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *K8sControlPlaneController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: k8s.AdmissionControlConfigType,
			Kind: controller.OutputExclusive,
		},
		{
			Type: k8s.AuditPolicyConfigType,
			Kind: controller.OutputExclusive,
		},
		{
			Type: k8s.APIServerConfigType,
			Kind: controller.OutputExclusive,
		},
		{
			Type: k8s.ControllerManagerConfigType,
			Kind: controller.OutputExclusive,
		},
		{
			Type: k8s.ExtraManifestsConfigType,
			Kind: controller.OutputExclusive,
		},
		{
			Type: k8s.BootstrapManifestsConfigType,
			Kind: controller.OutputExclusive,
		},
		{
			Type: k8s.SchedulerConfigType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *K8sControlPlaneController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		cfg, err := r.Get(ctx, resource.NewMetadata(config.NamespaceName, config.MachineConfigType, config.V1Alpha1ID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				if err = ctrl.teardownAll(ctx, r, logger); err != nil {
					return fmt.Errorf("error destroying resources: %w", err)
				}

				continue
			}

			return fmt.Errorf("error getting config: %w", err)
		}

		cfgProvider := cfg.(*config.MachineConfig).Config()

		machineTypeRes, err := r.Get(ctx, resource.NewMetadata(config.NamespaceName, config.MachineTypeType, config.MachineTypeID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting machine type: %w", err)
		}

		machineType := machineTypeRes.(*config.MachineType).MachineType()

		if machineType == machine.TypeWorker {
			if err = ctrl.teardownAll(ctx, r, logger); err != nil {
				return fmt.Errorf("error destroying resources: %w", err)
			}

			continue
		}

		for _, f := range []func(context.Context, controller.Runtime, *zap.Logger, talosconfig.Provider) error{
			ctrl.manageAPIServerConfig,
			ctrl.manageAdmissionControlConfig,
			ctrl.manageAuditPolicyConfig,
			ctrl.manageControllerManagerConfig,
			ctrl.manageSchedulerConfig,
			ctrl.manageManifestsConfig,
			ctrl.manageExtraManifestsConfig,
		} {
			if err = f(ctx, r, logger, cfgProvider); err != nil {
				return fmt.Errorf("error updating objects: %w", err)
			}
		}
	}
}

func convertVolumes(volumes []talosconfig.VolumeMount) []k8s.ExtraVolume {
	return slices.Map(volumes, func(v talosconfig.VolumeMount) k8s.ExtraVolume {
		return k8s.ExtraVolume{
			Name:      v.Name(),
			HostPath:  v.HostPath(),
			MountPath: v.MountPath(),
			ReadOnly:  v.ReadOnly(),
		}
	})
}

func (ctrl *K8sControlPlaneController) manageAPIServerConfig(ctx context.Context, r controller.Runtime, logger *zap.Logger, cfgProvider talosconfig.Provider) error {
	var cloudProvider string
	if cfgProvider.Cluster().ExternalCloudProvider().Enabled() {
		cloudProvider = "external"
	}

	advertisedAddress := "$(POD_IP)"
	if cfgProvider.Machine().Kubelet().SkipNodeRegistration() {
		advertisedAddress = ""
	}

	return r.Modify(ctx, k8s.NewAPIServerConfig(), func(r resource.Resource) error {
		*r.(*k8s.APIServerConfig).TypedSpec() = k8s.APIServerConfigSpec{
			Image:                    cfgProvider.Cluster().APIServer().Image(),
			CloudProvider:            cloudProvider,
			ControlPlaneEndpoint:     cfgProvider.Cluster().Endpoint().String(),
			EtcdServers:              []string{fmt.Sprintf("https://%s", nethelpers.JoinHostPort("localhost", constants.EtcdClientPort))},
			LocalPort:                cfgProvider.Cluster().LocalAPIServerPort(),
			ServiceCIDRs:             cfgProvider.Cluster().Network().ServiceCIDRs(),
			ExtraArgs:                cfgProvider.Cluster().APIServer().ExtraArgs(),
			ExtraVolumes:             convertVolumes(cfgProvider.Cluster().APIServer().ExtraVolumes()),
			EnvironmentVariables:     cfgProvider.Cluster().APIServer().Env(),
			PodSecurityPolicyEnabled: !cfgProvider.Cluster().APIServer().DisablePodSecurityPolicy(),
			AdvertisedAddress:        advertisedAddress,
		}

		return nil
	})
}

func (ctrl *K8sControlPlaneController) manageAdmissionControlConfig(ctx context.Context, r controller.Runtime, logger *zap.Logger, cfgProvider talosconfig.Provider) error {
	spec := k8s.AdmissionControlConfigSpec{}

	for _, cfg := range cfgProvider.Cluster().APIServer().AdmissionControl() {
		spec.Config = append(spec.Config,
			k8s.AdmissionPluginSpec{
				Name:          cfg.Name(),
				Configuration: cfg.Configuration(),
			},
		)
	}

	return r.Modify(ctx, k8s.NewAdmissionControlConfig(), func(r resource.Resource) error {
		*r.(*k8s.AdmissionControlConfig).TypedSpec() = spec

		return nil
	})
}

func (ctrl *K8sControlPlaneController) manageAuditPolicyConfig(ctx context.Context, r controller.Runtime, logger *zap.Logger, cfgProvider talosconfig.Provider) error {
	spec := k8s.AuditPolicyConfigSpec{}

	spec.Config = cfgProvider.Cluster().APIServer().AuditPolicy()

	return r.Modify(ctx, k8s.NewAuditPolicyConfig(), func(r resource.Resource) error {
		*r.(*k8s.AuditPolicyConfig).TypedSpec() = spec

		return nil
	})
}

func (ctrl *K8sControlPlaneController) manageControllerManagerConfig(ctx context.Context, r controller.Runtime, logger *zap.Logger, cfgProvider talosconfig.Provider) error {
	var cloudProvider string
	if cfgProvider.Cluster().ExternalCloudProvider().Enabled() {
		cloudProvider = "external"
	}

	return r.Modify(ctx, k8s.NewControllerManagerConfig(), func(r resource.Resource) error {
		*r.(*k8s.ControllerManagerConfig).TypedSpec() = k8s.ControllerManagerConfigSpec{
			Enabled:              !cfgProvider.Machine().Controlplane().ControllerManager().Disabled(),
			Image:                cfgProvider.Cluster().ControllerManager().Image(),
			CloudProvider:        cloudProvider,
			PodCIDRs:             cfgProvider.Cluster().Network().PodCIDRs(),
			ServiceCIDRs:         cfgProvider.Cluster().Network().ServiceCIDRs(),
			ExtraArgs:            cfgProvider.Cluster().ControllerManager().ExtraArgs(),
			ExtraVolumes:         convertVolumes(cfgProvider.Cluster().ControllerManager().ExtraVolumes()),
			EnvironmentVariables: cfgProvider.Cluster().ControllerManager().Env(),
		}

		return nil
	})
}

func (ctrl *K8sControlPlaneController) manageSchedulerConfig(ctx context.Context, r controller.Runtime, logger *zap.Logger, cfgProvider talosconfig.Provider) error {
	return r.Modify(ctx, k8s.NewSchedulerConfig(), func(r resource.Resource) error {
		*r.(*k8s.SchedulerConfig).TypedSpec() = k8s.SchedulerConfigSpec{
			Enabled:              !cfgProvider.Machine().Controlplane().Scheduler().Disabled(),
			Image:                cfgProvider.Cluster().Scheduler().Image(),
			ExtraArgs:            cfgProvider.Cluster().Scheduler().ExtraArgs(),
			ExtraVolumes:         convertVolumes(cfgProvider.Cluster().Scheduler().ExtraVolumes()),
			EnvironmentVariables: cfgProvider.Cluster().Scheduler().Env(),
		}

		return nil
	})
}

func (ctrl *K8sControlPlaneController) manageManifestsConfig(ctx context.Context, r controller.Runtime, logger *zap.Logger, cfgProvider talosconfig.Provider) error {
	dnsServiceIPs, err := cfgProvider.Cluster().Network().DNSServiceIPs()
	if err != nil {
		return fmt.Errorf("error calculating DNS service IPs: %w", err)
	}

	dnsServiceIP := ""
	dnsServiceIPv6 := ""

	for _, ip := range dnsServiceIPs {
		if dnsServiceIP == "" && ip.To4().Equal(ip) {
			dnsServiceIP = ip.String()
		}

		if dnsServiceIPv6 == "" && talosnet.IsNonLocalIPv6(ip) {
			dnsServiceIPv6 = ip.String()
		}
	}

	return r.Modify(ctx, k8s.NewBootstrapManifestsConfig(), func(r resource.Resource) error {
		images := images.List(cfgProvider)

		proxyArgs, err := getProxyArgs(cfgProvider)
		if err != nil {
			return err
		}

		*r.(*k8s.BootstrapManifestsConfig).TypedSpec() = k8s.BootstrapManifestsConfigSpec{
			Server:        cfgProvider.Cluster().Endpoint().String(),
			ClusterDomain: cfgProvider.Cluster().Network().DNSDomain(),

			PodCIDRs: cfgProvider.Cluster().Network().PodCIDRs(),

			ProxyEnabled: cfgProvider.Cluster().Proxy().Enabled(),
			ProxyImage:   cfgProvider.Cluster().Proxy().Image(),
			ProxyArgs:    proxyArgs,

			CoreDNSEnabled: cfgProvider.Cluster().CoreDNS().Enabled(),
			CoreDNSImage:   cfgProvider.Cluster().CoreDNS().Image(),

			DNSServiceIP:   dnsServiceIP,
			DNSServiceIPv6: dnsServiceIPv6,

			FlannelEnabled:  cfgProvider.Cluster().Network().CNI().Name() == constants.FlannelCNI,
			FlannelImage:    images.Flannel,
			FlannelCNIImage: images.FlannelCNI,

			PodSecurityPolicyEnabled: !cfgProvider.Cluster().APIServer().DisablePodSecurityPolicy(),

			TalosAPIServiceEnabled: cfgProvider.Machine().Features().KubernetesTalosAPIAccess().Enabled(),
		}

		return nil
	})
}

func (ctrl *K8sControlPlaneController) manageExtraManifestsConfig(ctx context.Context, r controller.Runtime, logger *zap.Logger, cfgProvider talosconfig.Provider) error {
	return r.Modify(ctx, k8s.NewExtraManifestsConfig(), func(r resource.Resource) error {
		spec := k8s.ExtraManifestsConfigSpec{}

		for _, url := range cfgProvider.Cluster().Network().CNI().URLs() {
			spec.ExtraManifests = append(spec.ExtraManifests, k8s.ExtraManifest{
				Name:     url,
				URL:      url,
				Priority: "05", // push CNI to the top
			})
		}

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

		*r.(*k8s.ExtraManifestsConfig).TypedSpec() = spec

		return nil
	})
}

func (ctrl *K8sControlPlaneController) teardownAll(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	return nil
}

func getProxyArgs(cfgProvider talosconfig.Provider) ([]string, error) {
	clusterCidr := strings.Join(cfgProvider.Cluster().Network().PodCIDRs(), ",")

	builder := argsbuilder.Args{
		"cluster-cidr":           clusterCidr,
		"hostname-override":      "$(NODE_NAME)",
		"kubeconfig":             "/etc/kubernetes/kubeconfig",
		"proxy-mode":             cfgProvider.Cluster().Proxy().Mode(),
		"conntrack-max-per-core": "0",
	}

	policies := argsbuilder.MergePolicies{
		"kubeconfig": argsbuilder.MergeDenied,
	}

	if err := builder.Merge(cfgProvider.Cluster().Proxy().ExtraArgs(), argsbuilder.WithMergePolicies(policies)); err != nil {
		return nil, err
	}

	return builder.Args(), nil
}
