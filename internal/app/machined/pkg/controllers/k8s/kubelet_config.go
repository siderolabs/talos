// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"context"
	"fmt"
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

// NewKubeletConfigController instanciates the config controller.
func NewKubeletConfigController() *KubeletConfigController {
	return transform.NewController(
		transform.Settings[*config.MachineConfig, *k8s.KubeletConfig]{
			Name: "k8s.KubeletConfigController",
			MapMetadataOptionalFunc: func(cfg *config.MachineConfig) optional.Optional[*k8s.KubeletConfig] {
				if cfg.Metadata().ID() != config.V1Alpha1ID {
					return optional.None[*k8s.KubeletConfig]()
				}

				if cfg.Config().Cluster() == nil || cfg.Config().Machine() == nil {
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

				kubeletConfig.Image = cfgProvider.Machine().Kubelet().Image()

				kubeletConfig.ClusterDNS = cfgProvider.Machine().Kubelet().ClusterDNS()

				if len(kubeletConfig.ClusterDNS) == 0 {
					addrs, err := cfgProvider.Cluster().Network().DNSServiceIPs()
					if err != nil {
						return fmt.Errorf("error building DNS service IPs: %w", err)
					}

					kubeletConfig.ClusterDNS = xslices.Map(addrs, netip.Addr.String)
				}

				kubeletConfig.ClusterDomain = cfgProvider.Cluster().Network().DNSDomain()
				kubeletConfig.ExtraArgs = cfgProvider.Machine().Kubelet().ExtraArgs()
				kubeletConfig.ExtraMounts = cfgProvider.Machine().Kubelet().ExtraMounts()
				kubeletConfig.ExtraConfig = cfgProvider.Machine().Kubelet().ExtraConfig()
				kubeletConfig.CloudProviderExternal = cfgProvider.Cluster().ExternalCloudProvider().Enabled()
				kubeletConfig.DefaultRuntimeSeccompEnabled = cfgProvider.Machine().Kubelet().DefaultRuntimeSeccompProfileEnabled()
				kubeletConfig.SkipNodeRegistration = cfgProvider.Machine().Kubelet().SkipNodeRegistration()
				kubeletConfig.StaticPodListURL = staticPodURL.TypedSpec().URL
				kubeletConfig.DisableManifestsDirectory = cfgProvider.Machine().Kubelet().DisableManifestsDirectory()
				kubeletConfig.EnableFSQuotaMonitoring = cfgProvider.Machine().Features().DiskQuotaSupportEnabled()
				kubeletConfig.CredentialProviderConfig = cfgProvider.Machine().Kubelet().CredentialProviderConfig()

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
