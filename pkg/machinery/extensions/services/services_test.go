// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package services_test

import (
	_ "embed"
	"testing"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/talos/pkg/machinery/extensions/services"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
)

//go:embed "testdata/hello.yaml"
var helloYAML []byte

func TestUnmarshal(t *testing.T) {
	var spec services.Spec

	require.NoError(t, yaml.Unmarshal(helloYAML, &spec))

	assert.Equal(t, services.Spec{
		Name: "hello",
		Container: services.Container{
			Entrypoint: "hello-world",
			Args:       []string{"--development", "--log=debug"},
			Mounts: []specs.Mount{
				{
					Destination: "/var/lib/example",
					Type:        "bind",
					Source:      "/var/lib/example",
					Options:     []string{"rbind", "ro"},
				},
			},
		},
		Depends: []services.Dependency{
			{
				Service: "cri",
			},
			{
				Path: "/system/run/machined/machined.sock",
			},
			{
				Network: []nethelpers.Status{nethelpers.StatusAddresses},
			},
		},
		Restart: services.RestartNever,
	}, spec)

	assert.NoError(t, spec.Validate())
}

func TestValidate(t *testing.T) {
	for _, tt := range []struct {
		name          string
		spec          services.Spec
		expectedError string
	}{
		{
			name:          "empty",
			spec:          services.Spec{},
			expectedError: "3 errors occurred:\n\t* name \"\" is invalid\n\t* restart kind is invalid: RestartKind(0)\n\t* container endpoint can't be empty\n\n",
		},
		{
			name: "invalid name",
			spec: services.Spec{
				Name: "FOO",
				Container: services.Container{
					Entrypoint: "foo",
				},
				Restart: services.RestartAlways,
			},
			expectedError: "1 error occurred:\n\t* name \"FOO\" is invalid\n\n",
		},
		{
			name: "invalid deps",
			spec: services.Spec{
				Name: "foo",
				Container: services.Container{
					Entrypoint: "foo",
				},
				Depends: []services.Dependency{
					{},
					{
						Path: "./somefile",
					},
					{
						Network: []nethelpers.Status{
							0,
						},
					},
					{
						Network: []nethelpers.Status{
							nethelpers.StatusAddresses,
						},
						Path: "/foo",
					},
				},
				Restart: services.RestartAlways,
			},
			expectedError: "4 errors occurred:\n\t* no dependency specified\n\t* path is not absolute: \"./somefile\"\n\t* invalid network dependency: Status(0)\n\t* more than a single dependency is set\n\n",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.spec.Validate()
			assert.EqualError(t, err, tt.expectedError)
		})
	}
}
