// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package services_test

import (
	_ "embed"
	"testing"
	"time"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/talos/pkg/machinery/extensions/services"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
)

//go:embed "testdata/hello.yaml"
var helloYAML []byte

//go:embed "testdata/hello-host.yaml"
var helloHostYAML []byte

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
	assert.Equal(t, services.RunnerModeContainer, spec.RunnerMode)
}

func TestUnmarshalHostRunnerMode(t *testing.T) {
	var spec services.Spec

	require.NoError(t, yaml.Unmarshal(helloHostYAML, &spec))

	assert.Equal(t, services.RunnerModeHost, spec.RunnerMode)
	assert.Equal(t, &services.Command{
		Entrypoint: "/usr/local/bin/hello-shutdown",
		Args:       []string{"--graceful"},
		Timeout:    30 * time.Second,
	}, spec.PreShutdown)
	assert.NoError(t, spec.Validate())
}

func TestInvalidRunnerModeUnmarshal(t *testing.T) {
	var spec services.Spec

	err := yaml.Unmarshal([]byte("name: foo\nrunnerMode: bogus\ncontainer:\n  entrypoint: foo\nrestart: always\n"), &spec)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bogus")
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
			name: "invalid runner mode",
			spec: services.Spec{
				Name: "foo",
				Container: services.Container{
					Entrypoint: "foo",
				},
				RunnerMode: services.RunnerMode(100),
				Restart:    services.RestartAlways,
			},
			expectedError: "1 error occurred:\n\t* runner mode is invalid: RunnerMode(100)\n\n",
		},
		{
			name: "host runner with mounts",
			spec: services.Spec{
				Name: "foo",
				Container: services.Container{
					Entrypoint: "/usr/local/bin/foo",
					Mounts:     []specs.Mount{{Source: "/source", Destination: "/destination"}},
				},
				RunnerMode: services.RunnerModeHost,
				Restart:    services.RestartAlways,
			},
			expectedError: "1 error occurred:\n\t* container mounts are not supported in host runner mode\n\n",
		},
		{
			name: "host runner with security options",
			spec: services.Spec{
				Name: "foo",
				Container: services.Container{
					Entrypoint: "/usr/local/bin/foo",
					Security: services.Security{
						WriteableRootfs: true,
					},
				},
				RunnerMode: services.RunnerModeHost,
				Restart:    services.RestartAlways,
			},
			expectedError: "1 error occurred:\n\t* container security options are not supported in host runner mode\n\n",
		},
		{
			name: "host runner with empty security paths",
			spec: services.Spec{
				Name: "foo",
				Container: services.Container{
					Entrypoint: "/usr/local/bin/foo",
					Security: services.Security{
						MaskedPaths:   []string{},
						ReadonlyPaths: []string{},
					},
				},
				RunnerMode: services.RunnerModeHost,
				Restart:    services.RestartAlways,
			},
		},
		{
			name: "host runner with relative entrypoint",
			spec: services.Spec{
				Name: "foo",
				Container: services.Container{
					Entrypoint: "usr/local/bin/foo",
				},
				RunnerMode: services.RunnerModeHost,
				Restart:    services.RestartAlways,
			},
			expectedError: "1 error occurred:\n\t* container entrypoint must be an absolute host path in host runner mode: \"usr/local/bin/foo\"\n\n",
		},
		{
			name: "container runner with pre-shutdown hook",
			spec: services.Spec{
				Name: "foo",
				Container: services.Container{
					Entrypoint: "foo",
				},
				Restart: services.RestartAlways,
				PreShutdown: &services.Command{
					Entrypoint: "/usr/local/bin/foo-shutdown",
					Timeout:    time.Minute,
				},
			},
			expectedError: "1 error occurred:\n\t* pre-shutdown hook is only supported in host runner mode\n\n",
		},
		{
			name: "pre-shutdown hook with relative entrypoint",
			spec: services.Spec{
				Name: "foo",
				Container: services.Container{
					Entrypoint: "/usr/local/bin/foo",
				},
				RunnerMode: services.RunnerModeHost,
				Restart:    services.RestartAlways,
				PreShutdown: &services.Command{
					Entrypoint: "usr/local/bin/foo-shutdown",
					Timeout:    time.Minute,
				},
			},
			expectedError: "1 error occurred:\n\t* pre-shutdown entrypoint must be an absolute host path: \"usr/local/bin/foo-shutdown\"\n\n",
		},
		{
			name: "pre-shutdown hook without timeout",
			spec: services.Spec{
				Name: "foo",
				Container: services.Container{
					Entrypoint: "/usr/local/bin/foo",
				},
				RunnerMode: services.RunnerModeHost,
				Restart:    services.RestartAlways,
				PreShutdown: &services.Command{
					Entrypoint: "/usr/local/bin/foo-shutdown",
				},
			},
			expectedError: "1 error occurred:\n\t* pre-shutdown timeout must be positive\n\n",
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
			if tt.expectedError == "" {
				assert.NoError(t, err)

				return
			}

			assert.EqualError(t, err, tt.expectedError)
		})
	}
}
