// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.
package runtime_test

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/suite"

	runtimecontrollers "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/services"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

type ExtensionServiceSuite struct {
	RuntimeSuite
}

type serviceMock struct {
	mu           sync.Mutex
	services     map[string]system.Service
	running      map[string]bool
	timesStarted map[string]int
	timesStopped map[string]int
}

func (mock *serviceMock) Load(services ...system.Service) []string {
	mock.mu.Lock()
	defer mock.mu.Unlock()

	ids := make([]string, 0, len(services))

	for _, svc := range services {
		mock.services[svc.ID(nil)] = svc
		ids = append(ids, svc.ID(nil))
	}

	return ids
}

func (mock *serviceMock) Start(serviceIDs ...string) error {
	mock.mu.Lock()
	defer mock.mu.Unlock()

	for _, id := range serviceIDs {
		mock.running[id] = true
		mock.timesStarted[id]++
	}

	return nil
}

func (mock *serviceMock) IsRunning(id string) (system.Service, bool, error) {
	mock.mu.Lock()
	defer mock.mu.Unlock()

	svc, exists := mock.services[id]
	if !exists {
		return nil, false, fmt.Errorf("service %q not found", id)
	}

	_, running := mock.running[id]

	return svc, running, nil
}

func (mock *serviceMock) Stop(ctx context.Context, serviceIDs ...string) error {
	mock.mu.Lock()
	defer mock.mu.Unlock()

	for _, id := range serviceIDs {
		mock.running[id] = false
		mock.timesStopped[id]++
	}

	return nil
}

func (mock *serviceMock) getIDs() []string {
	mock.mu.Lock()
	defer mock.mu.Unlock()

	ids := make([]string, 0, len(mock.services))

	for id := range mock.services {
		ids = append(ids, id)
	}

	slices.Sort(ids)

	return ids
}

type serviceStartStopInfo struct {
	started int
	stopped int
}

func (mock *serviceMock) getTimesStartedStopped() map[string]serviceStartStopInfo {
	mock.mu.Lock()
	defer mock.mu.Unlock()

	result := map[string]serviceStartStopInfo{}

	for id := range mock.services {
		result[id] = serviceStartStopInfo{
			started: mock.timesStarted[id],
			stopped: mock.timesStopped[id],
		}
	}

	return result
}

func (mock *serviceMock) get(id string) system.Service {
	mock.mu.Lock()
	defer mock.mu.Unlock()

	return mock.services[id]
}

func (suite *ExtensionServiceSuite) TestReconcile() {
	svcMock := &serviceMock{
		services:     map[string]system.Service{},
		running:      map[string]bool{},
		timesStarted: map[string]int{},
		timesStopped: map[string]int{},
	}

	suite.Require().NoError(suite.runtime.RegisterController(&runtimecontrollers.ExtensionServiceController{
		V1Alpha1Services: svcMock,
		ConfigPath:       "testdata/extservices/",
	}))

	suite.startRuntime()

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			ids := svcMock.getIDs()

			if !slices.Equal(ids, []string{"ext-frr", "ext-hello-world"}) {
				return retry.ExpectedErrorf("services registered: %q", ids)
			}

			return nil
		},
	))

	helloSvc := svcMock.get("ext-hello-world")
	suite.Require().IsType(&services.Extension{}, helloSvc)

	suite.Assert().Equal("./hello-world", helloSvc.(*services.Extension).Spec.Container.Entrypoint)

	suite.Assert().Equal(
		map[string]serviceStartStopInfo{
			"ext-hello-world": {
				started: 1,
			},
			"ext-frr": {
				started: 1,
			},
		},
		svcMock.getTimesStartedStopped(),
	)

	helloConfig := runtime.NewExtensionServiceConfigStatusSpec(runtime.NamespaceName, "hello-world")
	helloConfig.TypedSpec().SpecVersion = "1"
	suite.Require().NoError(suite.state.Create(suite.ctx, helloConfig))

	assertTimesStartedStopped := func(expected map[string]serviceStartStopInfo) {
		suite.Assert().NoError(retry.Constant(5*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				actual := svcMock.getTimesStartedStopped()

				if !reflect.DeepEqual(actual, expected) {
					return retry.ExpectedErrorf("services restart status expected %v, actual %v", expected, actual)
				}

				return nil
			},
		))
	}

	// specVersion is 1, and ext-hello-world is already started, so it should not be restarted
	assertTimesStartedStopped(map[string]serviceStartStopInfo{
		"ext-hello-world": {
			started: 1,
			stopped: 0,
		},
		"ext-frr": {
			started: 1,
		},
	})

	unexpectedConfig := runtime.NewExtensionServiceConfigStatusSpec(runtime.NamespaceName, "unexpected")
	unexpectedConfig.TypedSpec().SpecVersion = "1"
	suite.Require().NoError(suite.state.Create(suite.ctx, unexpectedConfig))

	assertTimesStartedStopped(map[string]serviceStartStopInfo{
		"ext-hello-world": {
			started: 1,
			stopped: 0,
		},
		"ext-frr": {
			started: 1,
		},
	})

	// update config for hello service
	helloConfig.TypedSpec().SpecVersion = "2"
	suite.Require().NoError(suite.state.Update(suite.ctx, helloConfig))

	assertTimesStartedStopped(map[string]serviceStartStopInfo{
		"ext-hello-world": {
			started: 2,
			stopped: 1,
		},
		"ext-frr": {
			started: 1,
		},
	})

	// destroy config for hello service
	suite.Require().NoError(suite.state.Destroy(suite.ctx, helloConfig.Metadata()))

	assertTimesStartedStopped(map[string]serviceStartStopInfo{
		"ext-hello-world": {
			started: 3,
			stopped: 2,
		},
		"ext-frr": {
			started: 1,
		},
	})
}

func TestExtensionServiceSuite(t *testing.T) {
	suite.Run(t, new(ExtensionServiceSuite))
}
