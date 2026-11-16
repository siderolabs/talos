// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package container_test

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/types/container"
	"github.com/siderolabs/talos/pkg/machinery/extensions"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

func TestContainerConfigRuntime(t *testing.T) {
	t.Parallel()

	cfg := load(t, "apiVersion: v1alpha1\nkind: ContainerConfig\nname: nginx\nimage: nginx\n")
	assert.Equal(t, config.ContainerRuntimeRunc, cfg.Runtime())

	cfg = load(t, "apiVersion: v1alpha1\nkind: ContainerConfig\nname: nginx\nimage: nginx\nruntime: runsc\n")
	assert.Equal(t, config.ContainerRuntimeRunsc, cfg.Runtime())

	_, err := cfg.Validate(validationMode{})
	require.NoError(t, err)
}

func TestContainerConfigRuntimeValidate(t *testing.T) {
	t.Parallel()

	const missing = `container "nginx": runtime runsc requires the "gvisor" system extension, which is not installed`

	for _, test := range []struct {
		name string

		runtime    config.ContainerRuntime
		extensions []string
		// stage to seed MachineStatus with; unset leaves MachineStatus out.
		stage *runtime.MachineStage

		expectedWarning string
		expectedError   string
	}{
		{
			name:       "runc needs no extension",
			runtime:    config.ContainerRuntimeRunc,
			extensions: nil,
			stage:      new(runtime.MachineStageRunning),
		},
		{
			name:       "runsc with gvisor installed",
			runtime:    config.ContainerRuntimeRunsc,
			extensions: []string{"iscsi-tools", "gvisor"},
			stage:      new(runtime.MachineStageRunning),
		},
		{
			name:          "runsc without gvisor on a running machine",
			runtime:       config.ContainerRuntimeRunsc,
			extensions:    []string{"iscsi-tools"},
			stage:         new(runtime.MachineStageRunning),
			expectedError: missing,
		},
		{
			name:          "runsc without any extension in maintenance",
			runtime:       config.ContainerRuntimeRunsc,
			stage:         new(runtime.MachineStageMaintenance),
			expectedError: missing,
		},
		{
			name:            "runsc without gvisor while booting",
			runtime:         config.ContainerRuntimeRunsc,
			stage:           new(runtime.MachineStageBooting),
			expectedWarning: missing,
		},
		{
			name:            "runsc without gvisor before the machine status exists",
			runtime:         config.ContainerRuntimeRunsc,
			expectedWarning: missing,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctx := t.Context()
			st := state.WrapCore(inmem.NewState())

			for _, name := range test.extensions {
				ext := runtime.NewExtensionStatus(runtime.NamespaceName, "000."+name)
				ext.TypedSpec().Metadata = extensions.Metadata{Name: name}
				require.NoError(t, st.Create(ctx, ext))
			}

			if test.stage != nil {
				machineStatus := runtime.NewMachineStatus()
				machineStatus.TypedSpec().Stage = *test.stage
				require.NoError(t, st.Create(ctx, machineStatus))
			}

			cfg := container.NewContainerConfigV1Alpha1()
			cfg.MetaName = "nginx"
			cfg.ContainerImage = "nginx"
			cfg.ContainerRuntimeName = test.runtime

			warnings, err := cfg.RuntimeValidate(ctx, st, validationMode{})

			if test.expectedError == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, test.expectedError)
			}

			if test.expectedWarning == "" {
				assert.Empty(t, warnings)
			} else {
				assert.Equal(t, []string{test.expectedWarning}, warnings)
			}
		})
	}
}
