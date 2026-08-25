// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package services_test

import (
	"os"
	"testing"

	"github.com/containerd/containerd/v2/core/containers"
	"github.com/containerd/containerd/v2/core/snapshots"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/containerd/containerd/v2/pkg/oci"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/system/services"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/services/mocks"
	extservices "github.com/siderolabs/talos/pkg/machinery/extensions/services"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

type MockClient struct {
	controller *gomock.Controller
}

func (c *MockClient) SnapshotService(snapshotterName string) snapshots.Snapshotter {
	return mocks.NewMockSnapshotter(c.controller)
}

func TestGetOCIOptions(t *testing.T) {
	mockClient := MockClient{
		controller: gomock.NewController(t),
	}
	defer mockClient.controller.Finish()

	generateOCISpec := func(svc *services.Extension) (*oci.Spec, error) {
		ociOpts, err := svc.GetOCIOptions()
		if err != nil {
			return nil, err
		}

		return oci.GenerateSpec(namespaces.WithNamespace(t.Context(), "testNamespace"), &mockClient, &containers.Container{}, ociOpts...)
	}

	t.Run("default configurations are cleared away if user passes empty arrays for MaskedPaths and ReadonlyPaths", func(t *testing.T) {
		// given
		svc := &services.Extension{
			Spec: extservices.Spec{
				Container: extservices.Container{
					Security: extservices.Security{
						MaskedPaths:   []string{},
						ReadonlyPaths: []string{},
					},
				},
			},
		}

		// when
		spec, err := generateOCISpec(svc)

		// then
		assert.NoError(t, err)
		assert.Equal(t, []string{}, spec.Linux.MaskedPaths)
		assert.Equal(t, []string{}, spec.Linux.ReadonlyPaths)
	})

	t.Run("default configuration applies if user passes nil for MaskedPaths and ReadonlyPaths", func(t *testing.T) {
		// given
		svc := &services.Extension{
			Spec: extservices.Spec{
				Container: extservices.Container{
					Security: extservices.Security{
						MaskedPaths:   nil,
						ReadonlyPaths: nil,
					},
				},
			},
		}

		// when
		spec, err := generateOCISpec(svc)

		// then
		assert.NoError(t, err)
		assert.Equal(t, []string{
			"/proc/acpi",
			"/proc/asound",
			"/proc/kcore",
			"/proc/keys",
			"/proc/latency_stats",
			"/proc/timer_list",
			"/proc/timer_stats",
			"/proc/sched_debug",
			"/sys/firmware",
			"/sys/devices/virtual/powercap",
			"/proc/scsi",
		}, spec.Linux.MaskedPaths)
		assert.Equal(t, []string{
			"/proc/bus",
			"/proc/fs",
			"/proc/irq",
			"/proc/sys",
			"/proc/sysrq-trigger",
		}, spec.Linux.ReadonlyPaths)
	})

	t.Run("root fs is readonly unless explicitly enabled", func(t *testing.T) {
		// given
		svc := &services.Extension{
			Spec: extservices.Spec{
				Container: extservices.Container{
					Security: extservices.Security{
						WriteableRootfs: true,
					},
				},
			},
		}

		// when
		spec, err := generateOCISpec(svc)

		// then
		assert.NoError(t, err)
		assert.Equal(t, false, spec.Root.Readonly)
	})

	t.Run("root fs is readonly by default", func(t *testing.T) {
		// given
		svc := &services.Extension{
			Spec: extservices.Spec{
				Container: extservices.Container{
					Security: extservices.Security{},
				},
			},
		}

		// when
		spec, err := generateOCISpec(svc)

		// then
		assert.NoError(t, err)
		assert.Equal(t, true, spec.Root.Readonly)
	})

	t.Run("allows setting extra env vars", func(t *testing.T) {
		// given
		svc := &services.Extension{
			Spec: extservices.Spec{
				Container: extservices.Container{
					Environment: []string{
						"FOO=BAR",
					},
				},
			},
		}

		// when
		spec, err := generateOCISpec(svc)

		// then
		assert.NoError(t, err)
		assert.Equal(t, []string{"FOO=BAR"}, spec.Process.Env)
	})

	t.Run("allows setting extra envFile", func(t *testing.T) {
		tempDir := t.TempDir()
		envFile := tempDir + "/envfile"

		assert.NoError(t, os.WriteFile(envFile, []byte("FOO=BARFROMENVFILE"), 0o644))

		// given
		svc := &services.Extension{
			Spec: extservices.Spec{
				Container: extservices.Container{
					EnvironmentFile: envFile,
				},
			},
		}

		// when
		spec, err := generateOCISpec(svc)

		// then
		assert.NoError(t, err)
		assert.Equal(t, []string{"FOO=BARFROMENVFILE"}, spec.Process.Env)
	})
}

func TestExtensionHostRunnerMode(t *testing.T) {
	svc := &services.Extension{
		Spec: extservices.Spec{
			Name:       "hello",
			RunnerMode: extservices.RunnerModeHost,
			Container: extservices.Container{
				Entrypoint: "/usr/local/bin/hello",
				Args:       []string{"--log=debug"},
			},
			Depends: []extservices.Dependency{{Service: "networkd"}},
		},
	}

	args, err := svc.HostProcessArgs()
	require.NoError(t, err)

	assert.Equal(t, "ext-hello", args.ID)
	assert.Equal(t, []string{"/usr/local/bin/hello", "--log=debug"}, args.ProcessArgs)
	assert.Equal(t, []string{"networkd"}, svc.DependsOn(nil))
	assert.NoError(t, svc.PostFunc(nil, 0))
}

func TestExtensionHostRunnerConfig(t *testing.T) {
	svc := &services.Extension{
		Spec: extservices.Spec{
			Name:       "hello",
			RunnerMode: extservices.RunnerModeHost,
		},
	}

	mounts, env, err := svc.ApplyExtensionServiceConfig(&runtimeres.ExtensionServiceConfigSpec{
		Environment: []string{"FROM_CONFIG=true"},
	}, nil, []string{"FROM_MANIFEST=true"})
	require.NoError(t, err)
	assert.Empty(t, mounts)
	assert.Equal(t, []string{"FROM_MANIFEST=true", "FROM_CONFIG=true"}, env)

	_, _, err = svc.ApplyExtensionServiceConfig(&runtimeres.ExtensionServiceConfigSpec{
		Files: []runtimeres.ExtensionServiceConfigFile{{MountPath: "/etc/hello.conf"}},
	}, nil, nil)
	assert.EqualError(t, err, "extension service config files are not supported in host runner mode")
}

func TestExtensionContainerRunnerModeDefault(t *testing.T) {
	svc := &services.Extension{}

	assert.Equal(t, []string{"containerd"}, svc.DependsOn(nil))
}

func TestExtensionHostRunnerRejectsRelativeEntrypoint(t *testing.T) {
	svc := &services.Extension{
		Spec: extservices.Spec{
			Name:       "hello",
			RunnerMode: extservices.RunnerModeHost,
			Container: extservices.Container{
				Entrypoint: "usr/local/bin/hello",
			},
		},
	}

	_, err := svc.HostProcessArgs()
	assert.EqualError(t, err, "host runner entrypoint must be an absolute path: \"usr/local/bin/hello\"")
}
