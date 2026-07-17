// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"errors"
	"fmt"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
	"github.com/siderolabs/talos/pkg/machinery/labels"
)

//docgen:jsonschema

// KubeStaticPodConfig defines the KubeStaticPodConfig configuration name.
const KubeStaticPodConfig = "KubeStaticPodConfig"

func init() {
	registry.Register(KubeStaticPodConfig, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &KubeStaticPodConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.K8sStaticPodConfig = &KubeStaticPodConfigV1Alpha1{}
	_ config.NamedDocument      = &KubeStaticPodConfigV1Alpha1{}
	_ config.Validator          = &KubeStaticPodConfigV1Alpha1{}
)

// KubeStaticPodConfigV1Alpha1 configures a pod definition to be run as a static pod by the kubelet.
//
//	examples:
//	  - value: exampleKubeStaticPodConfigV1Alpha1()
//	alias: KubeStaticPodConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/KubeStaticPodConfig
type KubeStaticPodConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Name of the static pod.
	//   schemaRequired: true
	MetaName string `yaml:"name"`

	//   description: |
	//     Static pods can be used to run components which should be started before the Kubernetes control plane is up.
	//     Talos doesn't validate the pod definition.
	//     Updates to this field can be applied without a reboot.
	//
	//     See https://kubernetes.io/docs/tasks/configure-pod-container/static-pod/.
	//   schema:
	//     type: object
	//   schemaRequired: true
	PodSpec meta.Unstructured `yaml:"pod" merge:"replace"`
}

// NewKubeStaticPodConfigV1Alpha1 creates a new KubeStaticPodConfig config document.
func NewKubeStaticPodConfigV1Alpha1() *KubeStaticPodConfigV1Alpha1 {
	return &KubeStaticPodConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       KubeStaticPodConfig,
		},
	}
}

func exampleKubeStaticPodConfigV1Alpha1() *KubeStaticPodConfigV1Alpha1 {
	cfg := NewKubeStaticPodConfigV1Alpha1()
	cfg.MetaName = "nginx"
	cfg.PodSpec.Object = map[string]any{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]any{
			"name": "nginx",
		},
		"spec": map[string]any{
			"containers": []any{
				map[string]any{
					"name":  "nginx",
					"image": "nginx",
				},
			},
		},
	}

	return cfg
}

// Validate implements config.Validator interface.
func (s *KubeStaticPodConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var (
		errs     error
		warnings []string
	)

	if s.MetaName == "" {
		errs = errors.Join(errs, errors.New("static pod name is required"))
	} else if err := labels.ValidateDNS1123Subdomain(s.MetaName); err != nil {
		errs = errors.Join(errs, fmt.Errorf("static pod name is invalid: %w", err))
	}

	if len(s.PodSpec.Object) == 0 {
		errs = errors.Join(errs, errors.New("static pod spec is required"))
	}

	return warnings, errs
}

// Clone implements config.Document interface.
func (s *KubeStaticPodConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Name implements config.NamedDocument interface.
func (s *KubeStaticPodConfigV1Alpha1) Name() string {
	return s.MetaName
}

// K8sStaticPodConfigSignal implements config.K8sStaticPodConfig interface.
func (s *KubeStaticPodConfigV1Alpha1) K8sStaticPodConfigSignal() {}

// Pod implements config.K8sStaticPodConfig interface.
func (s *KubeStaticPodConfigV1Alpha1) Pod() map[string]any {
	return s.PodSpec.Object
}
