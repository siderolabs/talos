// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package generate

import (
	"fmt"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/types/k8s"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

func (in *Input) generateKubernetesControlplaneConfigs() []config.Document {
	if !in.Options.VersionContract.MultidocKubernetesConfigSupported() {
		return nil
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

	schedulerConfig := k8s.NewKubeSchedulerConfigV1Alpha1()
	schedulerConfig.PodImage = fmt.Sprintf("%s:v%s", constants.KubernetesSchedulerImage, in.KubernetesVersion)

	return []config.Document{
		etcdEncryptionConfig,
		schedulerConfig,
	}
}
