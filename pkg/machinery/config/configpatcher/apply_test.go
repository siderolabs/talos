// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package configpatcher_test

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/configpatcher"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
)

//go:embed testdata/apply/config.yaml
var config []byte

//go:embed testdata/apply/expected.yaml
var expected string

//go:embed testdata/patchpreserve/base.yaml
var preserveBase []byte

//go:embed testdata/patchpreserve/omitted_expected.yaml
var preserveOmittedExpected string

//go:embed testdata/patchpreserve/explicit_expected.yaml
var preserveExplicitExpected string

// TestApplyPreservesOmitted is a regression test for https://github.com/siderolabs/talos/issues/14584:
// fields with custom merge (CEL expressions, sizes) omitted in the patch should keep the base value,
// while explicit values (including `false`) should still replace it.
func TestApplyPreservesOmitted(t *testing.T) {
	for _, tc := range []struct {
		name     string
		patch    string
		expected string
	}{
		{
			name:     "Omitted",
			patch:    "@testdata/patchpreserve/omitted.yaml",
			expected: preserveOmittedExpected,
		},
		{
			name:     "Explicit",
			patch:    "@testdata/patchpreserve/explicit.yaml",
			expected: preserveExplicitExpected,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			patches, err := configpatcher.LoadPatches([]string{tc.patch})
			require.NoError(t, err)

			cfg, err := configloader.NewFromBytes(preserveBase)
			require.NoError(t, err)

			for _, tt := range []struct {
				name  string
				input configpatcher.Input
			}{
				{
					name:  "WithConfig",
					input: configpatcher.WithConfig(cfg),
				},
				{
					name:  "WithBytes",
					input: configpatcher.WithBytes(preserveBase),
				},
			} {
				t.Run(tt.name, func(t *testing.T) {
					out, err := configpatcher.Apply(tt.input, patches)
					require.NoError(t, err)

					data, err := out.Bytes()
					require.NoError(t, err)

					assert.Equal(t, tc.expected, string(data))
				})
			}
		})
	}
}

//go:embed testdata/multidoc/config.yaml
var configMultidoc []byte

//go:embed testdata/multidoc/expected.yaml
var expectedMultidoc string

//go:embed testdata/apply/expected_manifests.yaml
var expectedManifests string

func TestApply(t *testing.T) {
	patches, err := configpatcher.LoadPatches([]string{
		"@testdata/apply/strategic1.yaml",
		"@testdata/apply/jsonpatch1.yaml",
		"@testdata/apply/jsonpatch2.yaml",
		"@testdata/apply/strategic2.yaml",
		"@testdata/apply/strategic3.yaml",
	})
	require.NoError(t, err)

	cfg, err := configloader.NewFromBytes(config)
	require.NoError(t, err)

	for _, tt := range []struct {
		name  string
		input configpatcher.Input
	}{
		{
			name:  "WithConfig",
			input: configpatcher.WithConfig(cfg),
		},
		{
			name:  "WithBytes",
			input: configpatcher.WithBytes(config),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			out, err := configpatcher.Apply(tt.input, patches)
			require.NoError(t, err)

			bytes, err := out.Bytes()
			require.NoError(t, err)

			assert.Equal(t, expected, string(bytes))
		})
	}
}

func TestApplyMultiDocFail(t *testing.T) {
	patches, err := configpatcher.LoadPatches([]string{
		"@testdata/multidoc/jsonpatch.yaml",
		"@testdata/multidoc/strategic1.yaml",
	})
	require.NoError(t, err)

	cfg, err := configloader.NewFromBytes(configMultidoc)
	require.NoError(t, err)

	for _, tt := range []struct {
		name  string
		input configpatcher.Input
	}{
		{
			name:  "WithConfig",
			input: configpatcher.WithConfig(cfg),
		},
		{
			name:  "WithBytes",
			input: configpatcher.WithBytes(configMultidoc),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := configpatcher.Apply(tt.input, patches)
			assert.EqualError(t, err, "JSON6902 patches are not supported for multi-document machine configuration")
		})
	}
}

func TestApplyMultiDoc(t *testing.T) {
	patches, err := configpatcher.LoadPatches([]string{
		"@testdata/multidoc/strategic1.yaml",
		"@testdata/multidoc/strategic2.yaml",
		"@testdata/multidoc/strategic3.yaml",
	})
	require.NoError(t, err)

	cfg, err := configloader.NewFromBytes(configMultidoc)
	require.NoError(t, err)

	for _, tt := range []struct {
		name  string
		input configpatcher.Input
	}{
		{
			name:  "WithConfig",
			input: configpatcher.WithConfig(cfg),
		},
		{
			name:  "WithBytes",
			input: configpatcher.WithBytes(configMultidoc),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			out, err := configpatcher.Apply(tt.input, patches)
			require.NoError(t, err)

			bytes, err := out.Bytes()
			require.NoError(t, err)

			assert.Equal(t, expectedMultidoc, string(bytes))
		})
	}
}

func TestApplyToEmptyConfig(t *testing.T) {
	patches, err := configpatcher.LoadPatches([]string{
		"@testdata/multidoc/strategic2.yaml",
	})
	require.NoError(t, err)

	cfg, err := container.New()
	require.NoError(t, err)

	out, err := configpatcher.Apply(configpatcher.WithConfig(cfg), patches)
	require.NoError(t, err)

	patched, err := out.Config()
	require.NoError(t, err)

	require.Len(t, patched.Documents(), 2)
}

//go:embed testdata/auditpolicy/config.yaml
var configAudit []byte

//go:embed testdata/auditpolicy/expected.yaml
var expectedAudit []byte

func TestApplyAuditPolicy(t *testing.T) {
	patches, err := configpatcher.LoadPatches([]string{
		"@testdata/auditpolicy/patch1.yaml",
	})
	require.NoError(t, err)

	cfg, err := configloader.NewFromBytes(configAudit)
	require.NoError(t, err)

	for _, tt := range []struct {
		name  string
		input configpatcher.Input
	}{
		{
			name:  "WithConfig",
			input: configpatcher.WithConfig(cfg),
		},
		{
			name:  "WithBytes",
			input: configpatcher.WithBytes(configAudit),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			out, err := configpatcher.Apply(tt.input, patches)
			require.NoError(t, err)

			bytes, err := out.Bytes()
			require.NoError(t, err)

			assert.Equal(t, string(expectedAudit), string(bytes))
		})
	}
}

func TestApplyWithManifestNewline(t *testing.T) {
	patches, err := configpatcher.LoadPatches([]string{
		"@testdata/apply/strategic4.yaml",
	})
	require.NoError(t, err)

	cfg, err := configloader.NewFromBytes(config)
	require.NoError(t, err)

	for _, tt := range []struct {
		name  string
		input configpatcher.Input
	}{
		{
			name:  "WithConfig",
			input: configpatcher.WithConfig(cfg),
		},
		{
			name:  "WithBytes",
			input: configpatcher.WithBytes(config),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			out, err := configpatcher.Apply(tt.input, patches)
			require.NoError(t, err)

			bytes, err := out.Bytes()
			require.NoError(t, err)

			// Verify that after all our transformations the YAML is still valid and newline is removed
			_, err = configloader.NewFromBytes(bytes)
			require.NoError(t, err)

			assert.Equal(t, expectedManifests, string(bytes))
		})
	}
}

//go:embed testdata/patchdelete/config.yaml
var configMultidocDelete []byte

//go:embed testdata/patchdelete/expected.yaml
var expectedMultidocDelete string

func TestApplyMultiDocDelete(t *testing.T) {
	patches, err := configpatcher.LoadPatches([]string{
		"@testdata/patchdelete/strategic1.yaml",
	})
	require.NoError(t, err)

	cfg, err := configloader.NewFromBytes(configMultidocDelete)
	require.NoError(t, err)

	for _, tt := range []struct {
		name  string
		input configpatcher.Input
	}{
		{
			name:  "WithConfig",
			input: configpatcher.WithConfig(cfg),
		},
		{
			name:  "WithBytes",
			input: configpatcher.WithBytes(configMultidocDelete),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			out, err := configpatcher.Apply(tt.input, patches)
			require.NoError(t, err)

			bytes, err := out.Bytes()
			require.NoError(t, err)

			assert.Equal(t, expectedMultidocDelete, string(bytes))
		})
	}
}

//go:embed testdata/patchdelete/controlplane_orig.yaml
var controlPlane []byte

//go:embed testdata/patchdelete/controlplane_expected.yaml
var controlPlaneExpected string

func TestApplyMultiDocCPDelete(t *testing.T) {
	patches, err := configpatcher.LoadPatches([]string{
		"@testdata/patchdelete/strategic2.yaml",
		"@testdata/patchdelete/strategic3.yaml",
		"@testdata/patchdelete/strategic4.yaml",
	})
	require.NoError(t, err)

	cfg, err := configloader.NewFromBytes(controlPlane)
	require.NoError(t, err)

	for _, tt := range []struct {
		name  string
		input configpatcher.Input
	}{
		{
			name:  "WithConfig",
			input: configpatcher.WithConfig(cfg),
		},
		{
			name:  "WithBytes",
			input: configpatcher.WithBytes(controlPlane),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			out, err := configpatcher.Apply(tt.input, patches)
			require.NoError(t, err)

			bytes, err := out.Bytes()
			require.NoError(t, err)

			assert.Equal(t, controlPlaneExpected, string(bytes))
		})
	}
}

//go:embed testdata/patchdeletemissing/config.yaml
var configPatchDeleteMissing []byte

func TestPatchDeleteMissing(t *testing.T) {
	patches, err := configpatcher.LoadPatches([]string{
		"@testdata/patchdeletemissing/strategic1.yaml",
	})
	require.NoError(t, err)

	cfg, err := configloader.NewFromBytes(configPatchDeleteMissing)
	require.NoError(t, err)

	for _, tt := range []struct {
		name  string
		input configpatcher.Input
	}{
		{
			name:  "WithConfig",
			input: configpatcher.WithConfig(cfg),
		},
		{
			name:  "WithBytes",
			input: configpatcher.WithBytes(configPatchDeleteMissing),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := configpatcher.Apply(tt.input, patches)
			require.Error(t, err)
			require.ErrorContains(t, err, `patch delete: path 'machine.network.hostname' in document '/v1alpha1': failed to delete path 'machine.network.hostname': lookup failed`)
		})
	}
}

//go:embed testdata/patchlink/base.yaml
var configPatchBase []byte

//go:embed testdata/patchlink/expected.yaml
var configPatchExpected string

func TestPatchLink(t *testing.T) {
	patches, err := configpatcher.LoadPatches([]string{
		"@testdata/patchlink/patch.yaml",
	})
	require.NoError(t, err)

	cfg, err := configloader.NewFromBytes(configPatchBase)
	require.NoError(t, err)

	for _, tt := range []struct {
		name  string
		input configpatcher.Input
	}{
		{
			name:  "WithConfig",
			input: configpatcher.WithConfig(cfg),
		},
		{
			name:  "WithBytes",
			input: configpatcher.WithBytes(configPatchBase),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			out, err := configpatcher.Apply(tt.input, patches)
			require.NoError(t, err)

			bytes, err := out.Bytes()
			require.NoError(t, err)

			assert.Equal(t, configPatchExpected, string(bytes))
		})
	}
}

//go:embed testdata/patchmixed/base.yaml
var mixedBase []byte

//go:embed testdata/patchmixed/expected.yaml
var mixedExpected string

func TestPatchMixed(t *testing.T) {
	patches, err := configpatcher.LoadPatches([]string{
		"@testdata/patchmixed/patch.yaml",
	})
	require.NoError(t, err)

	cfg, err := configloader.NewFromBytes(mixedBase)
	require.NoError(t, err)

	for _, tt := range []struct {
		name  string
		input configpatcher.Input
	}{
		{
			name:  "WithConfig",
			input: configpatcher.WithConfig(cfg),
		},
		{
			name:  "WithBytes",
			input: configpatcher.WithBytes(mixedBase),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			out, err := configpatcher.Apply(tt.input, patches)
			require.NoError(t, err)

			bytes, err := out.Bytes()
			require.NoError(t, err)

			assert.Equal(t, mixedExpected, string(bytes))
		})
	}
}
