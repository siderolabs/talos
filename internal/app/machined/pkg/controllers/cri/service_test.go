// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cri_test

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	crictrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/cri"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/files"
)

type mockCRIServiceManager struct {
	mu sync.Mutex

	loaded    system.Service
	running   bool
	calls     []string
	stopHook  func() error
	startHook func() error
}

func (mock *mockCRIServiceManager) IsRunning(string) (system.Service, bool, error) {
	mock.mu.Lock()
	defer mock.mu.Unlock()

	if mock.loaded == nil {
		return nil, false, fmt.Errorf("service is not loaded")
	}

	return mock.loaded, mock.running, nil
}

func (mock *mockCRIServiceManager) Load(services ...system.Service) []string {
	mock.mu.Lock()
	defer mock.mu.Unlock()

	mock.loaded = services[0]
	mock.calls = append(mock.calls, "load")

	return []string{"cri"}
}

func (mock *mockCRIServiceManager) Stop(context.Context, ...string) error {
	mock.mu.Lock()

	mock.running = false
	mock.calls = append(mock.calls, "stop")
	hook := mock.stopHook

	mock.mu.Unlock()

	if hook != nil {
		return hook()
	}

	return nil
}

func (mock *mockCRIServiceManager) Start(...string) error {
	mock.mu.Lock()

	mock.running = true
	mock.calls = append(mock.calls, "start")
	hook := mock.startHook

	mock.mu.Unlock()

	if hook != nil {
		return hook()
	}

	return nil
}

func (mock *mockCRIServiceManager) reset() {
	mock.mu.Lock()
	defer mock.mu.Unlock()

	mock.loaded = nil
	mock.running = false
	mock.calls = nil
	mock.stopHook = nil
	mock.startHook = nil
}

func (mock *mockCRIServiceManager) snapshot() []string {
	mock.mu.Lock()
	defer mock.mu.Unlock()

	return append([]string(nil), mock.calls...)
}

type CRIServiceSuite struct {
	ctest.DefaultSuite

	serviceManager *mockCRIServiceManager
}

func TestCRIServiceSuite(t *testing.T) {
	t.Parallel()

	serviceManager := &mockCRIServiceManager{}

	suite.Run(t, &CRIServiceSuite{
		serviceManager: serviceManager,
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 10 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&crictrl.ServiceController{
					V1Alpha1Services: serviceManager,
				}))
			},
		},
	})
}

func (suite *CRIServiceSuite) TestStartAndRestart() {
	suite.serviceManager.reset()

	configStatus := files.NewEtcFileStatus(files.NamespaceName, constants.CRIConfig)
	configStatus.TypedSpec().SpecVersion = "1"
	suite.Create(configStatus)

	suite.Never(func() bool { return len(suite.serviceManager.snapshot()) > 0 }, 200*time.Millisecond, 10*time.Millisecond)

	baseSpecStatus := files.NewEtcFileStatus(files.NamespaceName, constants.CRIBaseRuntimeSpec)
	baseSpecStatus.TypedSpec().SpecVersion = "1"
	suite.Create(baseSpecStatus)

	suite.Eventually(func() bool {
		return slices.Equal([]string{"load", "start"}, suite.serviceManager.snapshot())
	}, time.Second, 10*time.Millisecond)

	ctest.UpdateWithConflicts(suite, configStatus, func(*files.EtcFileStatus) error { return nil })
	suite.Never(func() bool { return len(suite.serviceManager.snapshot()) > 2 }, 200*time.Millisecond, 10*time.Millisecond)

	ctest.UpdateWithConflicts(suite, configStatus, func(status *files.EtcFileStatus) error {
		status.TypedSpec().SpecVersion = "2"

		return nil
	})

	suite.Eventually(func() bool {
		return slices.Equal([]string{"load", "start", "stop", "start"}, suite.serviceManager.snapshot())
	}, time.Second, 10*time.Millisecond)

	ctest.UpdateWithConflicts(suite, baseSpecStatus, func(status *files.EtcFileStatus) error {
		status.TypedSpec().SpecVersion = "2"

		return nil
	})

	suite.Eventually(func() bool {
		return slices.Equal([]string{"load", "start", "stop", "start", "stop", "start"}, suite.serviceManager.snapshot())
	}, time.Second, 10*time.Millisecond)
}
