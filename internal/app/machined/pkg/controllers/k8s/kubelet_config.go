// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"context"
	"fmt"
	"net"

	"github.com/AlekSi/pointer"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/talos-systems/talos/pkg/machinery/resources/config"
	"github.com/talos-systems/talos/pkg/machinery/resources/k8s"
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
			ID:        pointer.ToString(config.V1Alpha1ID),
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

		cfg, err := r.Get(ctx, resource.NewMetadata(config.NamespaceName, config.MachineConfigType, config.V1Alpha1ID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting config: %w", err)
		}

		cfgProvider := cfg.(*config.MachineConfig).Config()

		if err = r.Modify(
			ctx,
			k8s.NewKubeletConfig(k8s.NamespaceName, k8s.KubeletID),
			func(r resource.Resource) error {
				kubeletConfig := r.(*k8s.KubeletConfig).TypedSpec()

				kubeletConfig.Image = cfgProvider.Machine().Kubelet().Image()

				kubeletConfig.ClusterDNS = cfgProvider.Machine().Kubelet().ClusterDNS()

				if len(kubeletConfig.ClusterDNS) == 0 {
					var addrs []net.IP

					addrs, err = cfgProvider.Cluster().Network().DNSServiceIPs()
					if err != nil {
						return fmt.Errorf("error building DNS service IPs: %w", err)
					}

					kubeletConfig.ClusterDNS = make([]string, 0, len(addrs))

					for _, addr := range addrs {
						kubeletConfig.ClusterDNS = append(kubeletConfig.ClusterDNS, addr.String())
					}
				}

				kubeletConfig.ClusterDomain = cfgProvider.Cluster().Network().DNSDomain()
				kubeletConfig.ExtraArgs = cfgProvider.Machine().Kubelet().ExtraArgs()
				kubeletConfig.ExtraMounts = cfgProvider.Machine().Kubelet().ExtraMounts()
				kubeletConfig.CloudProviderExternal = cfgProvider.Cluster().ExternalCloudProvider().Enabled()

				return nil
			},
		); err != nil {
			return fmt.Errorf("error modifying KubeletConfig resource: %w", err)
		}
	}
}
