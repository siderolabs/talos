// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"errors"
	"fmt"
	"strings"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
	"github.com/siderolabs/talos/pkg/machinery/labels"
)

//docgen:jsonschema

// KubeInlineManifestConfig defines the KubeInlineManifestConfig configuration name.
const KubeInlineManifestConfig = "KubeInlineManifestConfig"

func init() {
	registry.Register(KubeInlineManifestConfig, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &KubeInlineManifestConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.K8sInlineManifestConfig = &KubeInlineManifestConfigV1Alpha1{}
	_ config.NamedDocument           = &KubeInlineManifestConfigV1Alpha1{}
	_ config.Validator               = &KubeInlineManifestConfigV1Alpha1{}
)

// KubeInlineManifestConfigV1Alpha1 configures a Kubernetes manifest to be applied to the cluster.
//
//	examples:
//	  - value: exampleKubeInlineManifestConfigV1Alpha1()
//	alias: KubeInlineManifestConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/KubeInlineManifestConfig
type KubeInlineManifestConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Name of manifest.
	//   schemaRequired: true
	MetaName string `yaml:"name"`

	//   description: |
	//     Kubernetes manifest definition, it is supplied as a raw string.
	//     It might contain a set of YAML documents separated by `---`.
	//     The format matches what can be supplied as `kubectl apply -f <file>`.
	//   schemaRequired: true
	ManifestSpec string `yaml:"manifest"`
}

// NewKubeInlineManifestConfigV1Alpha1 creates a new KubeInlineManifestConfig config document.
func NewKubeInlineManifestConfigV1Alpha1() *KubeInlineManifestConfigV1Alpha1 {
	return &KubeInlineManifestConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       KubeInlineManifestConfig,
		},
	}
}

func exampleKubeInlineManifestConfigV1Alpha1() *KubeInlineManifestConfigV1Alpha1 {
	cfg := NewKubeInlineManifestConfigV1Alpha1()
	cfg.MetaName = "namespace-ci"
	cfg.ManifestSpec = strings.TrimSpace(`
apiVersion: v1
kind: Namespace
metadata:
  name: ci
`)

	return cfg
}

// Validate implements config.Validator interface.
func (s *KubeInlineManifestConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var (
		errs     error
		warnings []string
	)

	if s.MetaName == "" {
		errs = errors.Join(errs, errors.New("manifest name is required"))
	} else if err := labels.ValidateDNS1123Subdomain(s.MetaName); err != nil {
		errs = errors.Join(errs, fmt.Errorf("manifest name is invalid: %w", err))
	}

	return warnings, errs
}

// Clone implements config.Document interface.
func (s *KubeInlineManifestConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Name implements config.NamedDocument interface.
func (s *KubeInlineManifestConfigV1Alpha1) Name() string {
	return s.MetaName
}

// K8sInlineManifestConfigSignal implements config.K8sInlineManifestConfig interface.
func (s *KubeInlineManifestConfigV1Alpha1) K8sInlineManifestConfigSignal() {}

// Contents implements config.K8sInlineManifestConfig interface.
func (s *KubeInlineManifestConfigV1Alpha1) Contents() string {
	return s.ManifestSpec
}
