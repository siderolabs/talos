// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	yaml "go.yaml.in/yaml/v4"

	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
)

func TestBase64Bytes(t *testing.T) {
	// docgen: nodoc
	type test struct {
		CA v1alpha1.Base64Bytes `yaml:"ca,omitempty"`
	}

	input := test{
		CA: []byte{0xde, 0xad, 0xbe, 0xef},
	}

	out, err := yaml.Marshal(&input)
	require.NoError(t, err)

	assert.Equal(t, "ca: 3q2+7w==\n", string(out))

	var decoded test

	require.NoError(t, yaml.Unmarshal(out, &decoded))

	assert.Equal(t, input.CA, decoded.CA)
}

func TestDiskSizeMatcherUnmarshal(t *testing.T) {
	obj := struct {
		M *v1alpha1.InstallDiskSizeMatcher `yaml:"m"`
	}{
		M: &v1alpha1.InstallDiskSizeMatcher{},
	}

	for _, test := range []struct {
		condition string
		size      string
		match     bool
		err       bool
	}{
		{
			condition: "<= 256GB",
			size:      "200GB",
			match:     true,
		},
		{
			condition: ">= 256GB",
			size:      "200GB",
			match:     false,
		},
		{
			condition: "<256GB",
			size:      "256GB",
			match:     false,
		},
		{
			condition: ">256GB",
			size:      "256GB",
			match:     false,
		},
		{
			condition: ">256GB",
			size:      "257GB",
			match:     true,
		},
		{
			condition: "==256GB",
			size:      "256GB",
			match:     true,
		},
		{
			condition: "==256GB",
			size:      "257GB",
			match:     false,
		},
		{
			condition: "==   256GB",
			size:      "256GB",
			match:     true,
		},
		{
			condition: "256GB",
			size:      "256GB",
			match:     true,
		},
		{
			condition: "   256GB",
			size:      "256GB",
			match:     true,
		},
		{
			condition: "9a256GB",
			err:       true,
		},
		{
			condition: "--256GB",
			err:       true,
		},
		{
			condition: "<<256GB",
			err:       true,
		},
		{
			condition: ">1",
			size:      "1GB",
			match:     true,
		},
		{
			condition: "< 1",
			size:      "1GB",
			match:     false,
		},
	} {
		err := yaml.Unmarshal([]byte(fmt.Sprintf("m: '%s'\n", test.condition)), &obj)
		if test.err {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
		}
	}
}
