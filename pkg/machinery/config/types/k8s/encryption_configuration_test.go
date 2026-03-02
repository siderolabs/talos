// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/types/k8s"
)

func TestEncryptionConfigurationDecode(t *testing.T) {
	t.Parallel()

	input := []byte(`apiVersion: apiserver.config.k8s.io/v1
kind: EncryptionConfiguration
resources:
  - resources:
      - secrets
    providers:
      - aescbc:
          keys:
            - name: key1
              secret: c2VjcmV0IGlzIHNlY3VyZQ==
      - identity: {}
`)

	provider, err := configloader.NewFromBytes(input)
	require.NoError(t, err)

	encryptionConfig := provider.EtcdEncryption()
	require.NotNil(t, encryptionConfig)

	yamlOutput := encryptionConfig.EtcdEncryptionConfig()
	assert.Contains(t, yamlOutput, "resources:")
	assert.Contains(t, yamlOutput, "aescbc:")
	assert.Contains(t, yamlOutput, "c2VjcmV0IGlzIHNlY3VyZQ==")
}

func TestEncryptionConfigurationValidate(t *testing.T) {
	t.Parallel()

	t.Run("valid", func(t *testing.T) {
		t.Parallel()

		doc := &k8s.EncryptionConfigurationDoc{
			Fields: map[string]any{
				"apiVersion": "apiserver.config.k8s.io/v1",
				"kind":       "EncryptionConfiguration",
				"resources": []any{
					map[string]any{
						"resources": []any{"secrets"},
						"providers": []any{
							map[string]any{
								"identity": map[string]any{},
							},
						},
					},
				},
			},
		}

		warnings, err := doc.Validate(nil)
		assert.NoError(t, err)
		assert.Empty(t, warnings)
	})

	t.Run("empty resources", func(t *testing.T) {
		t.Parallel()

		doc := &k8s.EncryptionConfigurationDoc{
			Fields: map[string]any{
				"apiVersion": "apiserver.config.k8s.io/v1",
				"kind":       "EncryptionConfiguration",
			},
		}

		_, err := doc.Validate(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "at least one resource entry")
	})
}

func TestEncryptionConfigurationClone(t *testing.T) {
	t.Parallel()

	original := &k8s.EncryptionConfigurationDoc{
		Fields: map[string]any{
			"apiVersion": "apiserver.config.k8s.io/v1",
			"kind":       "EncryptionConfiguration",
			"resources": []any{
				map[string]any{
					"resources": []any{"secrets"},
				},
			},
		},
	}

	cloned := original.Clone().(*k8s.EncryptionConfigurationDoc)

	// Modify original and verify clone is not affected.
	original.Fields["extra"] = "modified"

	assert.NotContains(t, cloned.Fields, "extra")
	assert.Equal(t, "EncryptionConfiguration", cloned.Kind())
}
