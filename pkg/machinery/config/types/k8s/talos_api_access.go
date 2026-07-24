// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"errors"
	"fmt"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
	"github.com/siderolabs/talos/pkg/machinery/role"
)

//docgen:jsonschema

// KubeTalosAPIAccessConfig defines the KubeTalosAPIAccessConfig configuration name.
const KubeTalosAPIAccessConfig = "KubeTalosAPIAccessConfig"

func init() {
	registry.Register(KubeTalosAPIAccessConfig, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &KubeTalosAPIAccessConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.K8sTalosAPIAccessConfig      = &KubeTalosAPIAccessConfigV1Alpha1{}
	_ config.Validator                    = &KubeTalosAPIAccessConfigV1Alpha1{}
	_ container.V1Alpha1ConflictValidator = &KubeTalosAPIAccessConfigV1Alpha1{}
	_ container.ControlplaneOnlyConfig    = &KubeTalosAPIAccessConfigV1Alpha1{}
)

// KubeTalosAPIAccessConfigV1Alpha1 configures access to Talos API from Kubernetes pods via service accounts.
//
//	examples:
//	  - value: exampleKubeTalosAPIAccessConfigV1Alpha1()
//	alias: KubeTalosAPIAccessConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/KubeTalosAPIAccessConfig
type KubeTalosAPIAccessConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     The list of Talos API roles which can be granted for access from Kubernetes pods.
	//
	//     Empty list means that no roles can be granted, so access is blocked.
	AccessAllowedRoles []string `yaml:"allowedRoles,omitempty"`
	//   description: |
	//     The list of Kubernetes namespaces Talos API access is available from.
	AccessAllowedKubernetesNamespaces []string `yaml:"allowedKubernetesNamespaces,omitempty"`
}

// NewKubeTalosAPIAccessConfigV1Alpha1 creates a new KubeTalosAPIAccessConfig config document.
func NewKubeTalosAPIAccessConfigV1Alpha1() *KubeTalosAPIAccessConfigV1Alpha1 {
	return &KubeTalosAPIAccessConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       KubeTalosAPIAccessConfig,
		},
	}
}

func exampleKubeTalosAPIAccessConfigV1Alpha1() *KubeTalosAPIAccessConfigV1Alpha1 {
	cfg := NewKubeTalosAPIAccessConfigV1Alpha1()
	cfg.AccessAllowedRoles = []string{
		string(role.Reader),
	}
	cfg.AccessAllowedKubernetesNamespaces = []string{
		"kube-system",
	}

	return cfg
}

// Clone implements config.Document interface.
func (s *KubeTalosAPIAccessConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Validate implements config.Validator interface.
//
//nolint:dupl
func (s *KubeTalosAPIAccessConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var (
		errs     error
		warnings []string
	)

	for _, r := range s.AccessAllowedRoles {
		if !role.All.Includes(role.Role(r)) {
			errs = errors.Join(errs, fmt.Errorf("invalid role %q in .allowedRoles", r))
		}
	}

	return warnings, errs
}

// V1Alpha1ConflictValidate implements container.V1Alpha1ConflictValidator interface.
//
//nolint:gocyclo
func (s *KubeTalosAPIAccessConfigV1Alpha1) V1Alpha1ConflictValidate(v1alpha1Cfg *v1alpha1.Config) error {
	if v1alpha1Cfg.MachineConfig != nil && v1alpha1Cfg.MachineConfig.MachineFeatures != nil {
		if v1alpha1Cfg.MachineConfig.MachineFeatures.KubernetesTalosAPIAccessConfig != nil { //nolint:staticcheck // testing deprecated field
			return errors.New(".machine.features.kubernetesTalosAPIAccess is already set in v1alpha1 config")
		}
	}

	return nil
}

// AllowedRoles implements config.K8sTalosAPIAccessConfig interface.
func (s *KubeTalosAPIAccessConfigV1Alpha1) AllowedRoles() []string {
	return s.AccessAllowedRoles
}

// AllowedKubernetesNamespaces implements config.K8sTalosAPIAccessConfig interface.
func (s *KubeTalosAPIAccessConfigV1Alpha1) AllowedKubernetesNamespaces() []string {
	return s.AccessAllowedKubernetesNamespaces
}

// ControlplaneOnlyDocument implements container.ControlplaneOnlyConfig interface.
func (s *KubeTalosAPIAccessConfigV1Alpha1) ControlplaneOnlyDocument() {}
