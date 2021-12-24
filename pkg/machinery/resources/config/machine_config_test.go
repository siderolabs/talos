// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config_test

import (
	"regexp"
	"testing"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/talos-systems/talos/pkg/machinery/config/configloader"
	"github.com/talos-systems/talos/pkg/machinery/resources/config"
)

func TestMachineConfigMarshal(t *testing.T) {
	cfg, err := configloader.NewFromBytes([]byte(`version: v1alpha1
persist: true # foo
debug: false
machine:
  type: controlplane
`))
	require.NoError(t, err)

	r := config.NewMachineConfig(cfg)

	m, err := resource.MarshalYAML(r)
	require.NoError(t, err)

	enc, err := yaml.Marshal(m)
	require.NoError(t, err)

	enc = regexp.MustCompile("(created|updated): [0-9-:TZ+]+").ReplaceAll(enc, nil)

	assert.Equal(t,
		"metadata:\n    namespace: config\n    type: MachineConfigs.config.talos.dev\n    id: v1alpha1\n    version: 1\n    owner:\n    phase: running\n    \n    \n"+
			"spec:\n    version: v1alpha1\n    persist: true # foo\n    debug: false\n    machine:\n      type: controlplane\n",
		string(enc))
}
