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
	"github.com/siderolabs/talos/pkg/machinery/labels"
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
	_ config.SecretDocument               = &KubeAuthorizerConfigV1Alpha1{}
	_ container.V1Alpha1ConflictValidator = &KubeAuthorizerConfigV1Alpha1{}
	_ container.ControlplaneOnlyConfig    = &KubeAuthorizerConfigV1Alpha1{}
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
	//
	//     When `kubeconfig` is set, `connectionInfo` should be omitted, as Talos fills it in.
	//   schema:
	//     type: object
	AuthorizerWebhook meta.Unstructured `yaml:"webhook,omitempty"`
	//   description: |
	//     Kubeconfig kube-apiserver uses to connect to the webhook authorizer.
	//
	//     Talos writes the kubeconfig to a file inside the kube-apiserver static pod and sets
	//     `webhook.connectionInfo` to `type: KubeConfigFile` pointing to that file.
	//     This field can only be set for the Webhook authorizer type.
	//
	//     The kubeconfig should be self-contained: kube-apiserver can't access any files on the host,
	//     so certificates and keys should be specified inline (`certificate-authority-data`, `client-certificate-data`,
	//     `client-key-data`), and `tokenFile`, `exec` and `auth-provider` are not supported.
	//     If the certificate authority is not specified, kube-apiserver uses the trusted root CAs of the machine.
	AuthorizerWebhookKubeconfig string `yaml:"kubeconfig,omitempty"`
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
	cfg.AuthorizerWebhookKubeconfig = `apiVersion: v1
kind: Config
clusters:
  - name: webhook
    cluster:
      server: https://authz.example.com/authorize
      certificate-authority-data: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCi4uLgotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0tCg==
users:
  - name: kube-apiserver
    user:
      token: webhook-token
contexts:
  - name: webhook
    context:
      cluster: webhook
      user: kube-apiserver
current-context: webhook
`

	return cfg
}

func exampleKubeAuthorizerConfigV1Alpha4() *KubeAuthorizerConfigV1Alpha1 {
	cfg := NewKubeAuthorizerConfigV1Alpha1()

	cfg.MetaName = "local-authorizer"
	cfg.AuthorizerType = "Webhook"
	cfg.AuthorizerWebhook = meta.Unstructured{
		Object: map[string]any{
			"timeout":                    "3s",
			"subjectAccessReviewVersion": "v1",
			"matchConditionSubjectAccessReviewVersion": "v1",
			"failurePolicy": "NoOpinion",
		},
	}
	cfg.AuthorizerWebhookKubeconfig = `apiVersion: v1
kind: Config
clusters:
  - name: local-authorizer
    cluster:
      server: http://127.0.0.1:9051/authorize
users:
  - name: local-authorizer
contexts:
  - name: local-authorizer
    context:
      cluster: local-authorizer
      user: local-authorizer
current-context: local-authorizer
`

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
	} else if err := labels.ValidateDNS1123Subdomain(s.MetaName); err != nil {
		errs = errors.Join(errs, fmt.Errorf("authorizer name is invalid: %w", err))
	}

	if s.AuthorizerType == "" {
		errs = errors.Join(errs, errors.New("authorizer type is required"))
	}

	if !slices.Contains(allowedAuthorizationAuthorizerTypes, s.AuthorizerType) {
		errs = errors.Join(errs, fmt.Errorf("authorizer type %s is not allowed, allowed types are %v", s.AuthorizerType, allowedAuthorizationAuthorizerTypes))
	}

	if s.AuthorizerType == "Webhook" {
		errs = errors.Join(errs, s.validateWebhook())
	}

	if s.AuthorizerType != "Webhook" && len(s.AuthorizerWebhook.Object) > 0 {
		warnings = append(warnings, "authorizer webhook configuration is not allowed non-Webhook authorizer types")
	}

	if s.AuthorizerType != "Webhook" && s.AuthorizerWebhookKubeconfig != "" {
		errs = errors.Join(errs, errors.New("authorizer kubeconfig is only allowed for Webhook authorizer type"))
	}

	return warnings, errs
}

func (s *KubeAuthorizerConfigV1Alpha1) validateWebhook() error {
	if len(s.AuthorizerWebhook.Object) == 0 {
		return errors.New("authorizer webhook configuration is required for Webhook authorizer type")
	}

	connectionInfo, hasConnectionInfo := s.AuthorizerWebhook.Object["connectionInfo"]

	if s.AuthorizerWebhookKubeconfig != "" {
		if hasConnectionInfo {
			return errors.New("authorizer webhook connectionInfo should not be set when kubeconfig is set, Talos fills it in")
		}

		if err := validateWebhookKubeconfig(s.AuthorizerWebhookKubeconfig); err != nil {
			return fmt.Errorf("authorizer kubeconfig: %w", err)
		}

		return nil
	}

	if !hasConnectionInfo {
		return errors.New("authorizer kubeconfig is required for Webhook authorizer type")
	}

	if connectionInfoMap, ok := connectionInfo.(map[string]any); ok && connectionInfoMap["type"] == "KubeConfigFile" {
		return errors.New("authorizer webhook connectionInfo type KubeConfigFile is not supported, as kube-apiserver can't access host files: use kubeconfig instead")
	}

	return nil
}

// Redact implements config.SecretDocument interface.
func (s *KubeAuthorizerConfigV1Alpha1) Redact(replacement string) {
	if s.AuthorizerWebhookKubeconfig != "" {
		s.AuthorizerWebhookKubeconfig = replacement
	}
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

// Kubeconfig implements config.K8sAuthorizerConfig interface.
func (s *KubeAuthorizerConfigV1Alpha1) Kubeconfig() string {
	return s.AuthorizerWebhookKubeconfig
}

// ControlplaneOnlyDocument implements container.ControlplaneOnlyConfig interface.
func (s *KubeAuthorizerConfigV1Alpha1) ControlplaneOnlyDocument() {}
