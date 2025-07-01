// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/cosi-project/runtime/pkg/state/registry"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

func TestRegisterResource(t *testing.T) {
	ctx := t.Context()

	resources := state.WrapCore(namespaced.NewState(inmem.Build))
	resourceRegistry := registry.NewResourceRegistry(resources)

	for _, resource := range []meta.ResourceWithRD{
		&runtime.DevicesStatus{},
		&runtime.Diagnostic{},
		&runtime.EventSinkConfig{},
		&runtime.ExtensionStatus{},
		&runtime.KernelCmdline{},
		&runtime.KernelModuleSpec{},
		&runtime.KernelParamSpec{},
		&runtime.KernelParamStatus{},
		&runtime.KmsgLogConfig{},
		&runtime.MachineStatus{},
		&runtime.MachineResetSignal{},
		&runtime.MaintenanceServiceConfig{},
		&runtime.MaintenanceServiceRequest{},
		&runtime.MetaKey{},
		&runtime.MetaLoaded{},
		&runtime.MountStatus{},
		&runtime.PlatformMetadata{},
		&runtime.SecurityState{},
		&runtime.UniqueMachineToken{},
		&runtime.WatchdogTimerConfig{},
		&runtime.WatchdogTimerStatus{},
	} {
		assert.NoError(t, resourceRegistry.Register(ctx, resource))
	}
}
