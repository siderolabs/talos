// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cri_test

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/cri"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
)

//go:embed testdata/cricustomizationconfig.yaml
var expectedCRICustomizationConfigDocument []byte

const customizationContent = `[metrics]
  address = "0.0.0.0:11234"
`

func TestCRICustomizationConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := cri.NewCRICustomizationConfigV1Alpha1("enable-metrics")
	cfg.CustomizationContent = customizationContent

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	assert.Equal(t, expectedCRICustomizationConfigDocument, marshaled)
}

func TestCRICustomizationConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedCRICustomizationConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, &cri.CRICustomizationConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       cri.CRICustomizationConfigKind,
		},
		MetaName:             "enable-metrics",
		CustomizationContent: customizationContent,
	}, docs[0])
}

func TestCRICustomizationConfigValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name          string
		configName    string
		content       string
		expectedError string
	}{
		{
			name:       "valid",
			configName: "enable-metrics",
			content:    customizationContent,
		},
		{
			name:          "empty name",
			content:       customizationContent,
			expectedError: `invalid name: name cannot be empty: ""`,
		},
		{
			name:       "qualified name",
			configName: "example.com/customization",
			content:    customizationContent,
		},
		{
			name:          "reserved legacy name",
			configName:    "customization",
			content:       customizationContent,
			expectedError: `name "customization" is reserved for the legacy CRI customization`,
		},
		{
			name:          "invalid qualified name",
			configName:    "example.com/foo/bar",
			content:       customizationContent,
			expectedError: "invalid name:",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := cri.NewCRICustomizationConfigV1Alpha1(test.configName)
			cfg.CustomizationContent = test.content

			warnings, err := cfg.Validate(validationMode{})
			assert.Empty(t, warnings)

			if test.expectedError == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, test.expectedError)
			}
		})
	}
}
