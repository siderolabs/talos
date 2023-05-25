// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/slices"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	runtimectrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime"
	v1alpha1runtime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	timeres "github.com/siderolabs/talos/pkg/machinery/resources/time"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

func TestMachineStatusSuite(t *testing.T) {
	eventCh := make(chan v1alpha1runtime.EventInfo)

	suite.Run(t, &MachineStatusSuite{
		eventCh: eventCh,
		DefaultSuite: ctest.DefaultSuite{
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&runtimectrl.MachineStatusController{
					V1Alpha1Events: &mockWatcher{eventCh: eventCh},
				}))
			},
		},
	})
}

type mockWatcher struct {
	eventCh chan v1alpha1runtime.EventInfo
}

func (m *mockWatcher) Watch(f v1alpha1runtime.WatchFunc, opt ...v1alpha1runtime.WatchOptionFunc) error {
	f(m.eventCh)

	return nil
}

type MachineStatusSuite struct {
	ctest.DefaultSuite

	eventCh chan v1alpha1runtime.EventInfo
}

func (suite *MachineStatusSuite) assertMachineStatus(stage runtime.MachineStage, ready bool, unmetConditions []string) {
	suite.AssertWithin(10*time.Second, 100*time.Millisecond, ctest.WrapRetry(func(assert *assert.Assertions, require *require.Assertions) {
		machineStatus, err := ctest.Get[*runtime.MachineStatus](suite, runtime.NewMachineStatus().Metadata())
		if err != nil {
			if state.IsNotFoundError(err) {
				assert.NoError(err)
			} else {
				require.NoError(err)
			}

			return
		}

		assert.Equal(stage, machineStatus.TypedSpec().Stage)
		assert.Equal(ready, machineStatus.TypedSpec().Status.Ready)

		assert.Equal(unmetConditions,
			slices.Map(machineStatus.TypedSpec().Status.UnmetConditions, func(c runtime.UnmetCondition) string { return c.Name }))
	}))
}

func (suite *MachineStatusSuite) TestReconcile() {
	suite.assertMachineStatus(runtime.MachineStageUnknown, true, nil)

	suite.eventCh <- v1alpha1runtime.EventInfo{
		Event: v1alpha1runtime.Event{
			Payload: &machineapi.SequenceEvent{
				Sequence: v1alpha1runtime.SequenceInitialize.String(),
				Action:   machineapi.SequenceEvent_START,
			},
		},
	}

	suite.assertMachineStatus(runtime.MachineStageBooting, false, []string{"time", "network", "services"})

	machineType := config.NewMachineType()
	machineType.SetMachineType(machine.TypeControlPlane)
	suite.Require().NoError(suite.State().Create(suite.Ctx(), machineType))

	timeStatus := timeres.NewStatus()
	timeStatus.TypedSpec().Synced = true
	suite.Require().NoError(suite.State().Create(suite.Ctx(), timeStatus))

	suite.eventCh <- v1alpha1runtime.EventInfo{
		Event: v1alpha1runtime.Event{
			Payload: &machineapi.SequenceEvent{
				Sequence: v1alpha1runtime.SequenceBoot.String(),
				Action:   machineapi.SequenceEvent_START,
			},
		},
	}

	suite.assertMachineStatus(runtime.MachineStageBooting, false, []string{"network", "services"})

	suite.eventCh <- v1alpha1runtime.EventInfo{
		Event: v1alpha1runtime.Event{
			Payload: &machineapi.SequenceEvent{
				Sequence: v1alpha1runtime.SequenceBoot.String(),
				Action:   machineapi.SequenceEvent_STOP,
			},
		},
	}

	networkStatus := network.NewStatus(network.NamespaceName, network.StatusID)
	networkStatus.TypedSpec().AddressReady = true
	networkStatus.TypedSpec().ConnectivityReady = true
	networkStatus.TypedSpec().EtcFilesReady = true
	networkStatus.TypedSpec().HostnameReady = true
	suite.Require().NoError(suite.State().Create(suite.Ctx(), networkStatus))

	suite.assertMachineStatus(runtime.MachineStageRunning, false, []string{"services"})

	for _, service := range []string{"apid", "etcd", "kubelet", "machined", "trustd"} {
		serviceStatus := v1alpha1.NewService(service)
		serviceStatus.TypedSpec().Running = true
		serviceStatus.TypedSpec().Healthy = true
		suite.Require().NoError(suite.State().Create(suite.Ctx(), serviceStatus))
	}

	suite.assertMachineStatus(runtime.MachineStageRunning, true, nil)

	suite.eventCh <- v1alpha1runtime.EventInfo{
		Event: v1alpha1runtime.Event{
			Payload: &machineapi.SequenceEvent{
				Sequence: v1alpha1runtime.SequenceReboot.String(),
				Action:   machineapi.SequenceEvent_START,
			},
		},
	}

	suite.assertMachineStatus(runtime.MachineStageRebooting, true, nil)
}
