// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"errors"
	"fmt"
	"slices"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
)

//docgen:jsonschema

// KubeAuthorizerConfig defines the KubeAuthorizerConfig configuration name.
const KubeAuthorizerConfig = "KubeAuthorizerConfig"

func init() {
	registry.Register(KubeAuthorizerConfig, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &KubeAuthorizerConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.K8sAuthorizerConfig          = &KubeAuthorizerConfigV1Alpha1{}
	_ config.NamedDocument                = &KubeAuthorizerConfigV1Alpha1{}
	_ config.Validator                    = &KubeAuthorizerConfigV1Alpha1{}
	_ container.V1Alpha1ConflictValidator = &KubeAuthorizerConfigV1Alpha1{}
)

// KubeAuthorizerConfigV1Alpha1 configures kube-apiserver authorization by configuring a specific authorization plugin.
//
//	examples:
//	  - value: exampleKubeAuthorizerConfigV1Alpha1()
//	  - value: exampleKubeAuthorizerConfigV1Alpha2()
//	  - value: exampleKubeAuthorizerConfigV1Alpha3()
//	  - value: exampleKubeAuthorizerConfigV1Alpha4()
//	alias: KubeAuthorizerConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/KubeAuthorizerConfig
type KubeAuthorizerConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Name of the authorizer, should be be DNS1123 labels like myauthorizername or subdomains like myauthorizer.example.domain.
	MetaName string `yaml:"name"`
	//   description: |
	//     Type is the name of the authorizer.
	//   values:
	//     - Node
	//     - RBAC
	//     - Webhook
	AuthorizerType string `yaml:"type"`
	//   description: |
	//     Webhook is the configuration for the webhook authorizer.
	//
	//     This field is required if the AuthorizerType is Webhook, should not be set for other authorizer types.
	//     The value is the literal Kubernetes webhook authorizer configuration.
	//   schema:
	//     type: object
	AuthorizerWebhook meta.Unstructured `yaml:"webhook,omitempty"`
}

// NewKubeAuthorizerConfigV1Alpha1 creates a new KubeAuthorizerConfig config document.
func NewKubeAuthorizerConfigV1Alpha1() *KubeAuthorizerConfigV1Alpha1 {
	return &KubeAuthorizerConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       KubeAuthorizerConfig,
		},
	}
}

// DefaultAuthorizationConfig returns a default Kubernetes authorization configuration.
func DefaultAuthorizationConfig() []*KubeAuthorizerConfigV1Alpha1 {
	nodeAuthorizer := NewKubeAuthorizerConfigV1Alpha1()
	nodeAuthorizer.MetaName = "node"
	nodeAuthorizer.AuthorizerType = "Node"

	rbacAuthorizer := NewKubeAuthorizerConfigV1Alpha1()
	rbacAuthorizer.MetaName = "rbac"
	rbacAuthorizer.AuthorizerType = "RBAC"

	return []*KubeAuthorizerConfigV1Alpha1{nodeAuthorizer, rbacAuthorizer}
}

func exampleKubeAuthorizerConfigV1Alpha1() *KubeAuthorizerConfigV1Alpha1 {
	return DefaultAuthorizationConfig()[0]
}

func exampleKubeAuthorizerConfigV1Alpha2() *KubeAuthorizerConfigV1Alpha1 {
	return DefaultAuthorizationConfig()[1]
}

func exampleKubeAuthorizerConfigV1Alpha3() *KubeAuthorizerConfigV1Alpha1 {
	cfg := NewKubeAuthorizerConfigV1Alpha1()

	cfg.MetaName = "webhook"
	cfg.AuthorizerType = "Webhook"
	cfg.AuthorizerWebhook = meta.Unstructured{
		Object: map[string]any{
			"timeout":                    "3s",
			"subjectAccessReviewVersion": "v1",
			"matchConditionSubjectAccessReviewVersion": "v1",
			"failurePolicy": "Deny",
			"connectionInfo": map[string]any{
				"type": "InClusterConfig",
			},
			"matchConditions": []map[string]any{
				{
					"expression": "has(request.resourceAttributes)",
				},
				{
					"expression": "!(\\'system:serviceaccounts:kube-system\\' in request.groups)",
				},
			},
		},
	}

	return cfg
}

func exampleKubeAuthorizerConfigV1Alpha4() *KubeAuthorizerConfigV1Alpha1 {
	cfg := NewKubeAuthorizerConfigV1Alpha1()

	cfg.MetaName = "in-cluster-authorizer"
	cfg.AuthorizerType = "Webhook"
	cfg.AuthorizerWebhook = meta.Unstructured{
		Object: map[string]any{
			"timeout":                    "3s",
			"subjectAccessReviewVersion": "v1",
			"matchConditionSubjectAccessReviewVersion": "v1",
			"failurePolicy": "NoOpinion",
			"connectionInfo": map[string]any{
				"type": "InClusterConfig",
			},
		},
	}

	return cfg
}

// Clone implements config.Document interface.
func (s *KubeAuthorizerConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

var allowedAuthorizationAuthorizerTypes = []string{"Node", "RBAC", "Webhook"}

// Validate implements config.Validator interface.
func (s *KubeAuthorizerConfigV1Alpha1) Validate(_ validation.RuntimeMode, opts ...validation.Option) ([]string, error) {
	var (
		errs     error
		warnings []string
	)

	if s.MetaName == "" {
		errs = errors.Join(errs, errors.New("authorizer name is required"))
	}

	if s.AuthorizerType == "" {
		errs = errors.Join(errs, errors.New("authorizer type is required"))
	}

	if !slices.Contains(allowedAuthorizationAuthorizerTypes, s.AuthorizerType) {
		errs = errors.Join(errs, fmt.Errorf("authorizer type %s is not allowed, allowed types are %v", s.AuthorizerType, allowedAuthorizationAuthorizerTypes))
	}

	if s.AuthorizerType == "Webhook" && len(s.AuthorizerWebhook.Object) == 0 {
		errs = errors.Join(errs, errors.New("authorizer webhook configuration is required for Webhook authorizer type"))
	}

	if s.AuthorizerType != "Webhook" && len(s.AuthorizerWebhook.Object) > 0 {
		warnings = append(warnings, "authorizer webhook configuration is not allowed non-Webhook authorizer types")
	}

	return warnings, errs
}

// V1Alpha1ConflictValidate implements container.V1Alpha1ConflictValidator interface.
func (s *KubeAuthorizerConfigV1Alpha1) V1Alpha1ConflictValidate(v1alpha1Cfg *v1alpha1.Config) error {
	if len(v1alpha1Cfg.K8sAdmissionControlPluginConfigs()) > 0 {
		return errors.New("admission control plugin config is already set in v1alpha1 config")
	}

	return nil
}

// Name implements config.NamedDocument interface.
func (s *KubeAuthorizerConfigV1Alpha1) Name() string {
	return s.MetaName
}

// K8sAuthorizerConfigSignal implements config.K8sAuthorizerConfig interface.
func (s *KubeAuthorizerConfigV1Alpha1) K8sAuthorizerConfigSignal() {}

// Type implements config.K8sAuthorizerConfig interface.
func (s *KubeAuthorizerConfigV1Alpha1) Type() string {
	return s.AuthorizerType
}

// Webhook implements config.K8sAuthorizerConfig interface.
func (s *KubeAuthorizerConfigV1Alpha1) Webhook() map[string]any {
	return s.AuthorizerWebhook.Object
}
