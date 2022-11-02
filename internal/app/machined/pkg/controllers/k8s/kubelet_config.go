// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"context"
	"fmt"
	"net/netip"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/slices"
	"github.com/siderolabs/go-pointer"
	"go.uber.org/zap"

	talosconfig "github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

// KubeletConfigController renders manifests based on templates and config/secrets.
type KubeletConfigController struct{}

// Name implements controller.Controller interface.
func (ctrl *KubeletConfigController) Name() string {
	return "k8s.KubeletConfigController"
}

// Inputs implements controller.Controller interface.
func (ctrl *KubeletConfigController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        pointer.To(config.V1Alpha1ID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: k8s.NamespaceName,
			Type:      k8s.StaticPodServerStatusType,
			ID:        pointer.To(k8s.StaticPodServerStatusResourceID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *KubeletConfigController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: k8s.KubeletConfigType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *KubeletConfigController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		staticPodListURL, err := getStaticPodListURL(ctx, r)
		if err != nil {
			if state.IsNotFoundError(err) {
				logger.Warn("static pod list url is not available yet; not creating kubelet config", zap.Error(err))

				continue
			}

			return fmt.Errorf("error accessing static pod server status resource: %w", err)
		}

		cfg, err := safe.ReaderGet[*config.MachineConfig](ctx, r, resource.NewMetadata(config.NamespaceName, config.MachineConfigType, config.V1Alpha1ID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting config: %w", err)
		}

		cfgProvider := cfg.Config()

		if err = r.Modify(
			ctx,
			k8s.NewKubeletConfig(k8s.NamespaceName, k8s.KubeletID),
			modifyKubeletConfig(cfgProvider, staticPodListURL),
		); err != nil {
			return fmt.Errorf("error modifying KubeletConfig resource: %w", err)
		}
	}
}

func getStaticPodListURL(ctx context.Context, r controller.Runtime) (string, error) {
	staticPodURLRes, err := safe.ReaderGet[*k8s.StaticPodServerStatus](
		ctx,
		r,
		resource.NewMetadata(
			k8s.NamespaceName,
			k8s.StaticPodServerStatusType,
			k8s.StaticPodServerStatusResourceID,
			resource.VersionUndefined,
		))
	if err != nil {
		return "", err
	}

	return staticPodURLRes.TypedSpec().URL, nil
}

func modifyKubeletConfig(cfgProvider talosconfig.Provider, staticPodListURL string) func(resource.Resource) error {
	return func(r resource.Resource) error {
		kubeletConfig := r.(*k8s.KubeletConfig).TypedSpec()

		kubeletConfig.Image = cfgProvider.Machine().Kubelet().Image()

		kubeletConfig.ClusterDNS = cfgProvider.Machine().Kubelet().ClusterDNS()

		if len(kubeletConfig.ClusterDNS) == 0 {
			addrs, err := cfgProvider.Cluster().Network().DNSServiceIPs()
			if err != nil {
				return fmt.Errorf("error building DNS service IPs: %w", err)
			}

			kubeletConfig.ClusterDNS = slices.Map(addrs, netip.Addr.String)
		}

		kubeletConfig.ClusterDomain = cfgProvider.Cluster().Network().DNSDomain()
		kubeletConfig.ExtraArgs = cfgProvider.Machine().Kubelet().ExtraArgs()
		kubeletConfig.ExtraMounts = cfgProvider.Machine().Kubelet().ExtraMounts()
		kubeletConfig.ExtraConfig = cfgProvider.Machine().Kubelet().ExtraConfig()
		kubeletConfig.CloudProviderExternal = cfgProvider.Cluster().ExternalCloudProvider().Enabled()
		kubeletConfig.DefaultRuntimeSeccompEnabled = cfgProvider.Machine().Kubelet().DefaultRuntimeSeccompProfileEnabled()
		kubeletConfig.SkipNodeRegistration = cfgProvider.Machine().Kubelet().SkipNodeRegistration()
		kubeletConfig.StaticPodListURL = staticPodListURL
		kubeletConfig.DisableManifestsDirectory = cfgProvider.Machine().Kubelet().DisableManifestsDirectory()

		return nil
	}
}
