// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package helpers_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	yaml "gopkg.in/yaml.v3"

	"github.com/talos-systems/talos/cmd/talosctl/pkg/talos/helpers"
)

type cfg struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
}

func TestExtractFileFromTarGz(t *testing.T) {
	file, err := os.Open("./testdata/archive.tar.gz")
	assert.NoError(t, err)

	data, err := helpers.ExtractFileFromTarGz("kubeconfig", file)
	assert.NoError(t, err)

	// just some primitive sanity check that yaml file inside was not corrupted somehow
	var c cfg
	err = yaml.Unmarshal(data, &c)
	assert.NoError(t, err)

	assert.Equal(t, c.APIVersion, "v1")
	assert.Equal(t, c.Kind, "Config")

	_, err = helpers.ExtractFileFromTarGz("void", file)
	assert.Error(t, err)
}
