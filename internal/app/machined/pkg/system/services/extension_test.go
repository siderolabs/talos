// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package services_test

import (
	"context"
	"os"
	"testing"

	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	"github.com/containerd/containerd/snapshots"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/internal/app/machined/pkg/system/services"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/services/mocks"
	extservices "github.com/siderolabs/talos/pkg/machinery/extensions/services"
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

		return oci.GenerateSpec(namespaces.WithNamespace(context.Background(), "testNamespace"), &mockClient, &containers.Container{}, ociOpts...)
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
