// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"errors"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

//docgen:jsonschema

// KubeCoreDNSConfig defines the KubeCoreDNSConfig configuration name.
const KubeCoreDNSConfig = "KubeCoreDNSConfig"

func init() {
	registry.Register(KubeCoreDNSConfig, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &KubeCoreDNSConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.K8sCoreDNSConfig             = &KubeCoreDNSConfigV1Alpha1{}
	_ container.V1Alpha1ConflictValidator = &KubeCoreDNSConfigV1Alpha1{}
)

// KubeCoreDNSConfigV1Alpha1 configures CoreDNS deployment.
//
//	examples:
//	  - value: exampleKubeCoreDNSConfigV1Alpha1()
//	alias: KubeCoreDNSConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/KubeCoreDNSConfig
type KubeCoreDNSConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     By default, CoreDNS deployment is enabled.
	//     Set to false to disable the CoreDNS deployment.
	PodEnabled *bool `yaml:"enabled,omitempty"`
	//   description: |
	//     The container image used to run the CoreDNS.
	//
	//     If the value is not set, the default image will be used.
	PodImage string `yaml:"image,omitempty"`
}

// NewKubeCoreDNSConfigV1Alpha1 creates a new KubeCoreDNSConfig config document.
func NewKubeCoreDNSConfigV1Alpha1() *KubeCoreDNSConfigV1Alpha1 {
	return &KubeCoreDNSConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       KubeCoreDNSConfig,
		},
	}
}

func exampleKubeCoreDNSConfigV1Alpha1() *KubeCoreDNSConfigV1Alpha1 {
	cfg := NewKubeCoreDNSConfigV1Alpha1()
	cfg.PodEnabled = new(true)
	cfg.PodImage = constants.CoreDNSImage + ":" + constants.DefaultCoreDNSVersion

	return cfg
}

// Clone implements config.Document interface.
func (s *KubeCoreDNSConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// V1Alpha1ConflictValidate implements container.V1Alpha1ConflictValidator interface.
func (s *KubeCoreDNSConfigV1Alpha1) V1Alpha1ConflictValidate(v1alpha1Cfg *v1alpha1.Config) error {
	if v1alpha1Cfg.ClusterConfig != nil && v1alpha1Cfg.ClusterConfig.CoreDNSConfig != nil { //nolint:staticcheck // testing deprecated field
		return errors.New("CoreDNS config is already set in v1alpha1 config (.cluster.coreDNS)")
	}

	return nil
}

// K8sCoreDNSConfigSignal implements config.K8sCoreDNSConfig interface.
func (s *KubeCoreDNSConfigV1Alpha1) K8sCoreDNSConfigSignal() {}

// Enabled implements config.K8sCoreDNSConfig interface.
func (s *KubeCoreDNSConfigV1Alpha1) Enabled() bool {
	if s.PodEnabled == nil {
		return true
	}

	return *s.PodEnabled
}

// Image implements config.K8sCoreDNSConfig interface.
func (s *KubeCoreDNSConfigV1Alpha1) Image() string {
	if s.PodImage == "" {
		return constants.CoreDNSImage + ":" + constants.DefaultCoreDNSVersion
	}

	return s.PodImage
}
