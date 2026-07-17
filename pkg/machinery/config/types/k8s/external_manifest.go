// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"errors"
	"fmt"
	"net/url"

	"github.com/siderolabs/gen/ensure"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
	"github.com/siderolabs/talos/pkg/machinery/labels"
)

//docgen:jsonschema

// KubeExternalManifestConfig defines the KubeExternalManifestConfig configuration name.
const KubeExternalManifestConfig = "KubeExternalManifestConfig"

func init() {
	registry.Register(KubeExternalManifestConfig, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &KubeExternalManifestConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.K8sExternalManifestConfig = &KubeExternalManifestConfigV1Alpha1{}
	_ config.NamedDocument             = &KubeExternalManifestConfigV1Alpha1{}
	_ config.Validator                 = &KubeExternalManifestConfigV1Alpha1{}
)

// KubeExternalManifestConfigV1Alpha1 configures a Kubernetes manifest which is downloaded from a URL.
//
//	examples:
//	  - value: exampleKubeExternalManifestConfigV1Alpha1()
//	alias: KubeExternalManifestConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/KubeExternalManifestConfig
type KubeExternalManifestConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Name of manifest.
	//   schemaRequired: true
	MetaName string `yaml:"name"`

	//   description: |
	//     Optional HTTP headers to use when downloading the manifest.
	HeadersSpec map[string]string `yaml:"headers,omitempty"`

	//   description: |
	//     Kubernetes manifest definition, via the URL to download it from.
	//     Please note that Talos does not watch URL contents, and might download
	//     the manifest only once, during the boot.
	//   schema:
	//     type: string
	//     pattern: "^(http|https)://"
	//   schemaRequired: true
	URLSpec meta.URL `yaml:"url"`
}

// NewKubeExternalManifestConfigV1Alpha1 creates a new KubeExternalManifestConfig config document.
func NewKubeExternalManifestConfigV1Alpha1() *KubeExternalManifestConfigV1Alpha1 {
	return &KubeExternalManifestConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       KubeExternalManifestConfig,
		},
	}
}

func exampleKubeExternalManifestConfigV1Alpha1() *KubeExternalManifestConfigV1Alpha1 {
	cfg := NewKubeExternalManifestConfigV1Alpha1()
	cfg.MetaName = "example-cni"
	cfg.URLSpec = meta.URL{URL: ensure.Value(url.Parse("https://www.example.com/manifest1.yaml"))}

	return cfg
}

// Validate implements config.Validator interface.
func (s *KubeExternalManifestConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var (
		errs     error
		warnings []string
	)

	if s.MetaName == "" {
		errs = errors.Join(errs, errors.New("manifest name is required"))
	} else if err := labels.ValidateDNS1123Subdomain(s.MetaName); err != nil {
		errs = errors.Join(errs, fmt.Errorf("manifest name is invalid: %w", err))
	}

	if s.URLSpec == (meta.URL{}) {
		errs = errors.Join(errs, errors.New("manifest URL is required"))
	} else if s.URLSpec.URL.Scheme != "http" && s.URLSpec.URL.Scheme != "https" {
		errs = errors.Join(errs, fmt.Errorf("manifest URL scheme must be http or https: %q", s.URLSpec.URL.Scheme))
	}

	return warnings, errs
}

// Clone implements config.Document interface.
func (s *KubeExternalManifestConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Name implements config.NamedDocument interface.
func (s *KubeExternalManifestConfigV1Alpha1) Name() string {
	return s.MetaName
}

// K8sExternalManifestConfigSignal implements config.K8sExternalManifestConfig interface.
func (s *KubeExternalManifestConfigV1Alpha1) K8sExternalManifestConfigSignal() {}

// Headers implements config.K8sExternalManifestConfig interface.
func (s *KubeExternalManifestConfigV1Alpha1) Headers() map[string]string {
	return s.HeadersSpec
}

// Contents implements config.K8sExternalManifestConfig interface.
func (s *KubeExternalManifestConfigV1Alpha1) URL() string {
	return s.URLSpec.URL.String()
}
