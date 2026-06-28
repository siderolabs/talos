// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	runtimecontrollers "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/services"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

type mockUdevServiceManager struct {
	mu      sync.Mutex
	actions []string
	service system.Service
	running bool
}

func (m *mockUdevServiceManager) IsRunning(id string) (system.Service, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.actions = append(m.actions, "is-running "+id)

	if m.service == nil {
		return nil, false, fmt.Errorf("service %q not defined", id)
	}

	return m.service, m.running, nil
}

func (m *mockUdevServiceManager) Load(svcs ...system.Service) []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	ids := make([]string, 0, len(svcs))

	for _, svc := range svcs {
		m.actions = append(m.actions, "load "+svc.ID(nil))
		m.service = svc
		ids = append(ids, svc.ID(nil))
	}

	return ids
}

func (m *mockUdevServiceManager) Start(serviceIDs ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, id := range serviceIDs {
		m.actions = append(m.actions, "start "+id)
	}

	m.running = true

	return nil
}

func (m *mockUdevServiceManager) waitForUdevd(context.Context, string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.actions = append(m.actions, "wait udevd")

	return nil
}

func (m *mockUdevServiceManager) snapshot() ([]string, system.Service) {
	m.mu.Lock()
	defer m.mu.Unlock()

	return append([]string(nil), m.actions...), m.service
}

func TestUdevServiceControllerStartsUdevdOnceWithKernelCmdlineSettleTime(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		serviceManager := &mockUdevServiceManager{}
		suite := &ctest.DefaultSuite{
			AfterSetup: func(s *ctest.DefaultSuite) {
				s.Require().NoError(s.Runtime().RegisterController(&runtimecontrollers.UdevServiceController{
					V1Alpha1Services: serviceManager,
					WaitForUdevd:     serviceManager.waitForUdevd,
				}))
			},
		}
		suite.SetT(t)
		suite.SetupTest()

		defer suite.TearDownTest()

		kernelCmdline := runtimeres.NewKernelCmdline()
		kernelCmdline.TypedSpec().Cmdline = constants.KernelParamDeviceSettleTime + "=3s"
		suite.Create(kernelCmdline)

		synctest.Wait()

		actions, svc := serviceManager.snapshot()
		require.Equal(t, []string{
			"load udevd",
			"is-running udevd",
			"start udevd",
			"wait udevd",
		}, actions)

		udevd, ok := svc.(*services.Udevd)
		require.True(t, ok)
		assert.Equal(t, 3*time.Second, udevd.ExtraSettleTime)

		ctest.UpdateWithConflicts(suite, kernelCmdline, func(cmdline *runtimeres.KernelCmdline) error {
			cmdline.TypedSpec().Cmdline = constants.KernelParamDeviceSettleTime + "=5s"

			return nil
		})

		synctest.Wait()

		actions, svc = serviceManager.snapshot()
		assert.Equal(t, []string{
			"load udevd",
			"is-running udevd",
			"start udevd",
			"wait udevd",
		}, actions)

		udevd, ok = svc.(*services.Udevd)
		require.True(t, ok)
		assert.Equal(t, 3*time.Second, udevd.ExtraSettleTime)
	})
}
