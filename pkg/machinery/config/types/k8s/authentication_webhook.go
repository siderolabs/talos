// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/go-pointer"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
)

//docgen:jsonschema

// KubeAuthenticationWebhookConfig defines the KubeAuthenticationWebhookConfig configuration name.
const KubeAuthenticationWebhookConfig = "KubeAuthenticationWebhookConfig"

func init() {
	registry.Register(KubeAuthenticationWebhookConfig, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &KubeAuthenticationWebhookConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.K8sAuthenticationWebhookConfig = &KubeAuthenticationWebhookConfigV1Alpha1{}
	_ config.Validator                      = &KubeAuthenticationWebhookConfigV1Alpha1{}
	_ config.SecretDocument                 = &KubeAuthenticationWebhookConfigV1Alpha1{}
	_ container.V1Alpha1ConflictValidator   = &KubeAuthenticationWebhookConfigV1Alpha1{}
	_ container.ControlplaneOnlyConfig      = &KubeAuthenticationWebhookConfigV1Alpha1{}
)

// KubeAuthenticationWebhookConfigV1Alpha1 configures kube-apiserver webhook token authentication.
//
//	description: |
//	  Bearer tokens which are not recognized by other authenticators are sent to the webhook
//	  (a `TokenReview` request) to be authenticated, e.g. for
//	  [aws-iam-authenticator](https://github.com/kubernetes-sigs/aws-iam-authenticator).
//
//	  Talos writes the kubeconfig to a file inside the kube-apiserver static pod and sets the
//	  `--authentication-token-webhook-*` flags of kube-apiserver accordingly,
//	  so these flags are not allowed in `KubeAPIServerConfig` `extraArgs`.
//
//	  See [webhook token authentication](https://kubernetes.io/docs/reference/access-authn-authz/authentication/#webhook-token-authentication).
//	examples:
//	  - value: exampleKubeAuthenticationWebhookConfigV1Alpha1()
//	alias: KubeAuthenticationWebhookConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/KubeAuthenticationWebhookConfig
type KubeAuthenticationWebhookConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Kubeconfig kube-apiserver uses to connect to the authentication webhook.
	//
	//     The kubeconfig should be self-contained: kube-apiserver can't access any files on the host,
	//     so certificates and keys should be specified inline (`certificate-authority-data`, `client-certificate-data`,
	//     `client-key-data`), and `tokenFile`, `exec` and `auth-provider` are not supported.
	//     If the certificate authority is not specified, kube-apiserver uses the trusted root CAs of the machine.
	//   schemaRequired: true
	WebhookKubeconfig string `yaml:"kubeconfig"`
	//   description: |
	//     The API version of the `authentication.k8s.io` `TokenReview` to send to and expect from the webhook.
	//
	//     Defaults to the kube-apiserver default (`v1beta1`).
	//   values:
	//     - v1
	//     - v1beta1
	WebhookVersion string `yaml:"version,omitempty"`
	//   description: |
	//     The duration to cache responses from the webhook token authenticator.
	//
	//     Defaults to the kube-apiserver default (2 minutes), set to `0s` to disable caching.
	//   schema:
	//     type: string
	//     pattern: ^[-+]?(((\d+(\.\d*)?|\.\d+|\d+)([nuµm]?s|m|h))|0)+$
	WebhookCacheTTL *time.Duration `yaml:"cacheTTL,omitempty"`
}

// NewKubeAuthenticationWebhookConfigV1Alpha1 creates a new KubeAuthenticationWebhookConfig config document.
func NewKubeAuthenticationWebhookConfigV1Alpha1() *KubeAuthenticationWebhookConfigV1Alpha1 {
	return &KubeAuthenticationWebhookConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       KubeAuthenticationWebhookConfig,
		},
	}
}

func exampleKubeAuthenticationWebhookConfigV1Alpha1() *KubeAuthenticationWebhookConfigV1Alpha1 {
	cfg := NewKubeAuthenticationWebhookConfigV1Alpha1()
	cfg.WebhookVersion = "v1beta1"
	cfg.WebhookCacheTTL = pointer.To(5 * time.Minute)
	cfg.WebhookKubeconfig = `apiVersion: v1
kind: Config
clusters:
  - name: aws-iam-authenticator
    cluster:
      server: https://127.0.0.1:21362/authenticate
      certificate-authority-data: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCi4uLgotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0tCg==
users:
  - name: apiserver
contexts:
  - name: webhook
    context:
      cluster: aws-iam-authenticator
      user: apiserver
current-context: webhook
`

	return cfg
}

// Clone implements config.Document interface.
func (s *KubeAuthenticationWebhookConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

var allowedAuthenticationWebhookVersions = []string{"v1", "v1beta1"}

// Validate implements config.Validator interface.
func (s *KubeAuthenticationWebhookConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var errs error

	if s.WebhookKubeconfig == "" {
		errs = errors.Join(errs, errors.New("authentication webhook kubeconfig is required"))
	} else if err := validateWebhookKubeconfig(s.WebhookKubeconfig); err != nil {
		errs = errors.Join(errs, fmt.Errorf("authentication webhook kubeconfig: %w", err))
	}

	if s.WebhookVersion != "" && !slices.Contains(allowedAuthenticationWebhookVersions, s.WebhookVersion) {
		errs = errors.Join(errs, fmt.Errorf("authentication webhook version %q is not allowed, allowed versions are %v", s.WebhookVersion, allowedAuthenticationWebhookVersions))
	}

	if s.WebhookCacheTTL != nil && *s.WebhookCacheTTL < 0 {
		errs = errors.Join(errs, errors.New("authentication webhook cache TTL should not be negative"))
	}

	return nil, errs
}

// Redact implements config.SecretDocument interface.
func (s *KubeAuthenticationWebhookConfigV1Alpha1) Redact(replacement string) {
	if s.WebhookKubeconfig != "" {
		s.WebhookKubeconfig = replacement
	}
}

// V1Alpha1ConflictValidate implements container.V1Alpha1ConflictValidator interface.
func (s *KubeAuthenticationWebhookConfigV1Alpha1) V1Alpha1ConflictValidate(v1alpha1Cfg *v1alpha1.Config) error {
	if v1alpha1Cfg.ClusterConfig == nil || v1alpha1Cfg.ClusterConfig.APIServerConfig == nil { //nolint:staticcheck // legacy configuration
		return nil
	}

	for arg := range v1alpha1Cfg.ClusterConfig.APIServerConfig.ExtraArgsConfig { //nolint:staticcheck // legacy configuration
		if strings.HasPrefix(arg, "authentication-token-webhook-") {
			return fmt.Errorf("kube-apiserver extra argument %q is already set in v1alpha1 config (.cluster.apiServer.extraArgs)", arg)
		}
	}

	return nil
}

// K8sAuthenticationWebhookConfigSignal implements config.K8sAuthenticationWebhookConfig interface.
func (s *KubeAuthenticationWebhookConfigV1Alpha1) K8sAuthenticationWebhookConfigSignal() {}

// Kubeconfig implements config.K8sAuthenticationWebhookConfig interface.
func (s *KubeAuthenticationWebhookConfigV1Alpha1) Kubeconfig() string {
	return s.WebhookKubeconfig
}

// Version implements config.K8sAuthenticationWebhookConfig interface.
func (s *KubeAuthenticationWebhookConfigV1Alpha1) Version() string {
	return s.WebhookVersion
}

// CacheTTL implements config.K8sAuthenticationWebhookConfig interface.
func (s *KubeAuthenticationWebhookConfigV1Alpha1) CacheTTL() optional.Optional[time.Duration] {
	if s.WebhookCacheTTL == nil {
		return optional.None[time.Duration]()
	}

	return optional.Some(*s.WebhookCacheTTL)
}

// ControlplaneOnlyDocument implements container.ControlplaneOnlyConfig interface.
func (s *KubeAuthenticationWebhookConfigV1Alpha1) ControlplaneOnlyDocument() {}
