// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"encoding/json"
	"fmt"

	yaml "go.yaml.in/yaml/v4"
	apiserverv1 "k8s.io/apiserver/pkg/apis/apiserver/v1"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
)

// EncryptionConfigurationKind is the document kind for native K8s EncryptionConfiguration.
const EncryptionConfigurationKind = "EncryptionConfiguration"

// EncryptionConfigurationAPIVersion is the expected apiVersion for EncryptionConfiguration.
const EncryptionConfigurationAPIVersion = "apiserver.config.k8s.io/v1"

func init() {
	registry.Register(EncryptionConfigurationKind, func(version string) config.Document {
		switch version {
		case EncryptionConfigurationAPIVersion:
			return &EncryptionConfigurationDoc{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.EtcdEncryptionConfig = &EncryptionConfigurationDoc{}
	_ config.Validator            = &EncryptionConfigurationDoc{}
)

// EncryptionConfigurationDoc wraps a native Kubernetes EncryptionConfiguration document.
//
// It allows users to provide a K8s EncryptionConfiguration directly in the Talos machine config,
// using the upstream apiVersion and kind fields.
type EncryptionConfigurationDoc struct {
	Fields map[string]any `yaml:",inline"`
}

// Kind implements config.Document interface.
func (d *EncryptionConfigurationDoc) Kind() string {
	return EncryptionConfigurationKind
}

// APIVersion implements config.Document interface.
func (d *EncryptionConfigurationDoc) APIVersion() string {
	return EncryptionConfigurationAPIVersion
}

// Clone implements config.Document interface.
func (d *EncryptionConfigurationDoc) Clone() config.Document {
	return &EncryptionConfigurationDoc{
		Fields: deepCopyMap(d.Fields),
	}
}

// Merge implements the merger interface for the merge package.
//
// EncryptionConfiguration is replaced entirely rather than recursively merged,
// since the inline map structure doesn't support meaningful partial merges.
func (d *EncryptionConfigurationDoc) Merge(other any) error {
	otherDoc, ok := other.(EncryptionConfigurationDoc)
	if !ok {
		return fmt.Errorf("unexpected type for merge: %T", other)
	}

	d.Fields = maps.Clone(otherDoc.Fields)

	return nil
}

// EtcdEncryptionConfig implements config.EtcdEncryptionConfig interface.
func (d *EncryptionConfigurationDoc) EtcdEncryptionConfig() string {
	data, err := yaml.Marshal(d.Fields)
	if err != nil {
		return ""
	}

	return string(data)
}

// Validate implements config.Validator interface.
func (d *EncryptionConfigurationDoc) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	jsonData, err := json.Marshal(d.Fields)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal EncryptionConfiguration to JSON: %w", err)
	}

	var cfg apiserverv1.EncryptionConfiguration
	if err := json.Unmarshal(jsonData, &cfg); err != nil {
		return nil, fmt.Errorf("invalid EncryptionConfiguration: %w", err)
	}

	if len(cfg.Resources) == 0 {
		return nil, fmt.Errorf("EncryptionConfiguration must have at least one resource entry")
	}

	return nil, nil
}

// deepCopyMap creates a deep copy of a map[string]any.
func deepCopyMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}

	cp := make(map[string]any, len(m))

	for k, v := range m {
		cp[k] = deepCopyValue(v)
	}

	return cp
}

func deepCopyValue(v any) any {
	switch val := v.(type) {
	case map[string]any:
		return deepCopyMap(val)
	case []any:
		cp := make([]any, len(val))
		for i, item := range val {
			cp[i] = deepCopyValue(item)
		}

		return cp
	default:
		return v
	}
}
