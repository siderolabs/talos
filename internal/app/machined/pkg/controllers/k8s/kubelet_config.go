// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"context"
	"net/netip"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/transform"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xerrors"
	"github.com/siderolabs/gen/xslices"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

// KubeletConfigController renders kubelet configuration based on machine config.
type KubeletConfigController = transform.Controller[*config.MachineConfig, *k8s.KubeletConfig]

// NewKubeletConfigController instantiates the config controller.
//
//nolint:gocyclo
func NewKubeletConfigController() *KubeletConfigController {
	return transform.NewController(
		transform.Settings[*config.MachineConfig, *k8s.KubeletConfig]{
			Name: "k8s.KubeletConfigController",
			MapMetadataOptionalFunc: func(cfg *config.MachineConfig) optional.Optional[*k8s.KubeletConfig] { //nolint:dupl
				if cfg.Metadata().ID() != config.ActiveID {
					return optional.None[*k8s.KubeletConfig]()
				}

				if cfg.Config().Cluster() == nil || cfg.Config().Machine() == nil {
					return optional.None[*k8s.KubeletConfig]()
				}

				if cfg.Config().K8sNetworkConfig() == nil {
					return optional.None[*k8s.KubeletConfig]()
				}

				if cfg.Config().K8sNodeConfig() == nil {
					return optional.None[*k8s.KubeletConfig]()
				}

				if cfg.Config().K8sKubeletConfig() == nil {
					return optional.None[*k8s.KubeletConfig]()
				}

				return optional.Some(k8s.NewKubeletConfig(k8s.NamespaceName, k8s.KubeletID))
			},
			TransformFunc: func(ctx context.Context, r controller.Reader, logger *zap.Logger, cfg *config.MachineConfig, res *k8s.KubeletConfig) error {
				staticPodURL, err := safe.ReaderGetByID[*k8s.StaticPodServerStatus](ctx, r, k8s.StaticPodServerStatusResourceID)
				if err != nil {
					if state.IsNotFoundError(err) {
						return xerrors.NewTaggedf[transform.SkipReconcileTag]("static pod server status resource not found; not creating kubelet config")
					}

					return err
				}

				kubeletConfig := res.TypedSpec()
				cfgProvider := cfg.Config()

				kubeletConfig.Image = cfgProvider.K8sKubeletConfig().Image()

				kubeletConfig.ClusterDNS = cfgProvider.K8sKubeletConfig().ClusterDNS()

				if len(kubeletConfig.ClusterDNS) == 0 {
					addrs := k8s.DNSServiceAddrs(cfgProvider.K8sNetworkConfig().ServiceCIDRs())

					kubeletConfig.ClusterDNS = xslices.Map(addrs, netip.Addr.String)
				}

				extraArgs := make(map[string]k8s.ArgValues, len(cfgProvider.K8sKubeletConfig().ExtraArgs()))
				for k, v := range cfgProvider.K8sKubeletConfig().ExtraArgs() {
					extraArgs[k] = k8s.ArgValues{Values: v}
				}

				kubeletConfig.ClusterDomain = cfgProvider.K8sNetworkConfig().DNSDomain()
				kubeletConfig.ExtraArgs = extraArgs
				kubeletConfig.ExtraMounts = cfgProvider.K8sKubeletConfig().ExtraMounts()
				kubeletConfig.ExtraConfig = cfgProvider.K8sKubeletConfig().ExtraConfig()
				kubeletConfig.CloudProviderExternal = cfgProvider.Cluster().ExternalCloudProvider().Enabled()
				kubeletConfig.DefaultRuntimeSeccompEnabled = cfgProvider.K8sKubeletConfig().DefaultRuntimeSeccompProfileEnabled()
				kubeletConfig.SkipNodeRegistration = cfgProvider.K8sNodeConfig().SkipNodeRegistration()
				kubeletConfig.StaticPodListURL = staticPodURL.TypedSpec().URL
				kubeletConfig.DisableManifestsDirectory = cfgProvider.K8sKubeletConfig().DisableManifestsDirectory()
				kubeletConfig.EnableFSQuotaMonitoring = cfgProvider.Machine().Features().DiskQuotaSupportEnabled()
				kubeletConfig.RegisterWithTaints = cfgProvider.K8sNodeConfig().Taints()

				if k8sCredentialProviderConfig := cfgProvider.K8sCredentialProviderConfig(); k8sCredentialProviderConfig != nil {
					kubeletConfig.CredentialProviderConfig = k8sCredentialProviderConfig.Configuration()
				} else {
					kubeletConfig.CredentialProviderConfig = nil
				}

				return nil
			},
		},
		transform.WithExtraInputs(
			controller.Input{
				Namespace: k8s.NamespaceName,
				Type:      k8s.StaticPodServerStatusType,
				ID:        optional.Some(k8s.StaticPodServerStatusResourceID),
				Kind:      controller.InputWeak,
			},
		),
	)
}
