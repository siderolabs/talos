// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
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
