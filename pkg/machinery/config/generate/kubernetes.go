// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package generate

import (
	"fmt"
	"net/netip"
	"slices"

	"github.com/siderolabs/gen/xslices"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/types/k8s"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

func (in *Input) generateKubernetesControlplaneConfigs() []config.Document {
	if !in.Options.VersionContract.MultidocKubernetesConfigSupported() {
		return nil
	}

	var flannelConfig *k8s.KubeFlannelCNIConfigV1Alpha1

	if in.Options.CNICustomURL == "" {
		flannelConfig = k8s.NewKubeFlannelCNIConfigV1Alpha1()
		flannelConfig.FlannelBackendType = constants.FlannelDefaultBackend
		flannelConfig.FlannelBackendPort = constants.FlannelDefaultBackendPort
	}

	etcdEncryptionConfig := k8s.NewKubeEtcdEncryptionConfigV1Alpha1()
	etcdEncryptionConfig.Config.Object = map[string]any{
		"resources": []any{
			map[string]any{
				"providers": []any{
					map[string]any{
						"secretbox": map[string]any{
							"keys": []any{
								map[string]any{
									"name":   "key1",
									"secret": in.Options.SecretsBundle.Secrets.SecretboxEncryptionSecret,
								},
							},
						},
					},
				},
				"resources": []any{
					"secrets",
				},
			},
		},
	}

	apiServerConfig := k8s.NewKubeAPIServerConfigV1Alpha1()
	apiServerConfig.PodImage = fmt.Sprintf("%s:v%s", constants.KubernetesAPIServerImage, in.KubernetesVersion)

	if in.Options.LocalAPIServerPort != 0 {
		apiServerConfig.PodAPIPort = new(in.Options.LocalAPIServerPort)
	}

	controllerManagerConfig := k8s.NewKubeControllerManagerConfigV1Alpha1()
	controllerManagerConfig.PodImage = fmt.Sprintf("%s:v%s", constants.KubernetesControllerManagerImage, in.KubernetesVersion)

	schedulerConfig := k8s.NewKubeSchedulerConfigV1Alpha1()
	schedulerConfig.PodImage = fmt.Sprintf("%s:v%s", constants.KubernetesSchedulerImage, in.KubernetesVersion)

	proxyConfig := k8s.NewKubeProxyConfigV1Alpha1()
	proxyConfig.ProxyImage = fmt.Sprintf("%s:v%s", constants.KubeProxyImage, in.KubernetesVersion)

	result := slices.Concat(
		[]config.Document{
			k8s.DefaultPodSecurityAdmissionControlConfig(),
			k8s.DefaultAuditPolicyConfig(),
			k8s.DefaultAuthenticationConfig(),
		},
		xslices.Map(
			k8s.DefaultAuthorizationConfig(),
			func(c *k8s.KubeAuthorizerConfigV1Alpha1) config.Document { return c },
		),
		[]config.Document{
			etcdEncryptionConfig,
			apiServerConfig,
			controllerManagerConfig,
			schedulerConfig,
			proxyConfig,
		},
	)

	if flannelConfig != nil {
		result = append(result, flannelConfig)
	}

	return result
}

func (in *Input) generateKubernetesUniversalConfigs() []config.Document {
	if !in.Options.VersionContract.MultidocKubernetesConfigSupported() {
		return nil
	}

	networkConfig := k8s.NewKubeNetworkConfigV1Alpha1()
	networkConfig.NetworkDNSDomain = in.Options.DNSDomain
	networkConfig.NetworkPodSubnets = xslices.Map(
		in.PodNet,
		func(s string) meta.Prefix {
			return meta.Prefix{Prefix: netip.MustParsePrefix(s)}
		},
	)
	networkConfig.NetworkServiceSubnets = xslices.Map(
		in.ServiceNet,
		func(s string) meta.Prefix {
			return meta.Prefix{Prefix: netip.MustParsePrefix(s)}
		},
	)

	return []config.Document{
		networkConfig,
	}
}
