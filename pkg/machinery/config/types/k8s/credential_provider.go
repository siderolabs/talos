// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
)

//docgen:jsonschema

// KubeCredentialProviderConfig defines the KubeCredentialProviderConfig configuration name.
const KubeCredentialProviderConfig = "KubeCredentialProviderConfig"

func init() {
	registry.Register(KubeCredentialProviderConfig, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &KubeCredentialProviderConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.K8sCredentialProviderConfig  = &KubeCredentialProviderConfigV1Alpha1{}
	_ config.SecretDocument               = &KubeCredentialProviderConfigV1Alpha1{}
	_ config.Validator                    = &KubeCredentialProviderConfigV1Alpha1{}
	_ container.V1Alpha1ConflictValidator = &KubeCredentialProviderConfigV1Alpha1{}
)

// KubeCredentialProviderConfigV1Alpha1 configures kubelet's credential provider.
//
//	examples:
//	  - value: exampleKubeCredentialProviderConfigV1Alpha1()
//	alias: KubeCredentialProviderConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/KubeCredentialProviderConfig
type KubeCredentialProviderConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Kubelet credential provider configuration (used for image registry authentication).
	//     The value is the literal kubelet's credential provider configuration.
	//   schema:
	//     type: object
	CredentialProviderConfig meta.Unstructured `yaml:"configuration" merge:"replace"`

	//   description: |
	//     Additional configuration files for the credential provider plugins.
	//
	//     Each file is written to the `/etc/kubernetes/credentialproviders` directory
	//     with the permissions `0600`, so it can be referenced in the `args` of a credential provider.
	//   examples:
	//     - value: exampleKubeCredentialProviderConfigFileV1Alpha1()
	CredentialProviderConfigFiles []KubeCredentialProviderConfigFileV1Alpha1 `yaml:"configFiles,omitempty" merge:"replace"`
}

// KubeCredentialProviderConfigFileV1Alpha1 is a configuration file for a credential provider plugin.
type KubeCredentialProviderConfigFileV1Alpha1 struct {
	//   description: |
	//     Name of the file in the `/etc/kubernetes/credentialproviders` directory.
	//   examples:
	//     - value: >
	//         "azure.json"
	FileName string `yaml:"name"`

	//   description: |
	//     Content of the file.
	FileContent string `yaml:"content"`
}

// NewKubeCredentialProviderConfigV1Alpha1 creates a new KubeCredentialProviderConfig config document.
func NewKubeCredentialProviderConfigV1Alpha1() *KubeCredentialProviderConfigV1Alpha1 {
	return &KubeCredentialProviderConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       KubeCredentialProviderConfig,
		},
	}
}

func exampleKubeCredentialProviderConfigV1Alpha1() *KubeCredentialProviderConfigV1Alpha1 {
	cfg := NewKubeCredentialProviderConfigV1Alpha1()
	cfg.CredentialProviderConfig.Object = map[string]any{
		"apiVersion": "kubelet.config.k8s.io/v1",
		"kind":       "CredentialProviderConfig",
		"providers": []any{
			map[string]any{
				"name":       "ecr-credential-provider",
				"apiVersion": "credentialprovider.kubelet.k8s.io/v1",
				"matchImages": []any{
					"*.dkr.ecr.*.amazonaws.com",
					"*.dkr.ecr.*.amazonaws.com.cn",
					"*.dkr.ecr-fips.*.amazonaws.com",
					"*.dkr.ecr.us-iso-east-1.c2s.ic.gov",
					"*.dkr.ecr.us-isob-east-1.sc2s.sgov.gov",
				},
				"defaultCacheDuration": "12h",
			},
		},
	}

	return cfg
}

func exampleKubeCredentialProviderConfigFileV1Alpha1() []KubeCredentialProviderConfigFileV1Alpha1 {
	return []KubeCredentialProviderConfigFileV1Alpha1{
		{
			FileName:    "azure.json",
			FileContent: "{\"cloud\": \"AzurePublicCloud\", \"useManagedIdentityExtension\": true}\n",
		},
	}
}

// V1Alpha1ConflictValidate implements container.V1Alpha1ConflictValidator interface.
func (s *KubeCredentialProviderConfigV1Alpha1) V1Alpha1ConflictValidate(v1alpha1Cfg *v1alpha1.Config) error {
	if v1alpha1Cfg.MachineConfig != nil && v1alpha1Cfg.MachineConfig.MachineKubelet != nil { //nolint:staticcheck // testing deprecated field
		return errors.New("cannot use KubeCredentialProviderConfig with legacy kubelet configuration (.machine.kubelet)")
	}

	return nil
}

// Validate implements config.Validator interface.
func (s *KubeCredentialProviderConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var errs error

	names := make(map[string]struct{}, len(s.CredentialProviderConfigFiles))

	for i, file := range s.CredentialProviderConfigFiles {
		if err := validateCredentialProviderConfigFileName(file.FileName); err != nil {
			errs = errors.Join(errs, fmt.Errorf("configFiles[%d]: %w", i, err))

			continue
		}

		if _, exists := names[file.FileName]; exists {
			errs = errors.Join(errs, fmt.Errorf("configFiles[%d]: duplicate file name %q", i, file.FileName))
		}

		names[file.FileName] = struct{}{}
	}

	return nil, errs
}

func validateCredentialProviderConfigFileName(name string) error {
	if name == "" {
		return errors.New("file name is required")
	}

	if strings.ContainsRune(name, filepath.Separator) || !filepath.IsLocal(name) || strings.HasPrefix(name, ".") {
		return fmt.Errorf("file name %q must be a plain file name", name)
	}

	return nil
}

// Redact implements config.SecretDocument interface.
func (s *KubeCredentialProviderConfigV1Alpha1) Redact(replacement string) {
	for i := range s.CredentialProviderConfigFiles {
		if s.CredentialProviderConfigFiles[i].FileContent != "" {
			s.CredentialProviderConfigFiles[i].FileContent = replacement
		}
	}
}

// Clone implements config.Document interface.
func (s *KubeCredentialProviderConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// K8sCredentialProviderConfigSignal implements config.K8sCredentialProviderConfig interface.
func (s *KubeCredentialProviderConfigV1Alpha1) K8sCredentialProviderConfigSignal() {}

// Configuration implements config.K8sCredentialProviderConfig interface.
func (s *KubeCredentialProviderConfigV1Alpha1) Configuration() map[string]any {
	return s.CredentialProviderConfig.Object
}

// ConfigFiles implements config.K8sCredentialProviderConfig interface.
func (s *KubeCredentialProviderConfigV1Alpha1) ConfigFiles() map[string]string {
	if len(s.CredentialProviderConfigFiles) == 0 {
		return nil
	}

	files := make(map[string]string, len(s.CredentialProviderConfigFiles))

	for _, file := range s.CredentialProviderConfigFiles {
		files[file.FileName] = file.FileContent
	}

	return files
}
