// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.
package runtime_test

import (
	"fmt"
	"reflect"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/talos-systems/go-retry/retry"

	runtimecontrollers "github.com/talos-systems/talos/internal/app/machined/pkg/controllers/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/services"
)

type ExtensionServiceSuite struct {
	RuntimeSuite
}

type serviceMock struct {
	mu       sync.Mutex
	services map[string]system.Service
}

func (mock *serviceMock) Load(services ...system.Service) []string {
	mock.mu.Lock()
	defer mock.mu.Unlock()

	ids := []string{}

	for _, svc := range services {
		mock.services[svc.ID(nil)] = svc
		ids = append(ids, svc.ID(nil))
	}

	return ids
}

func (mock *serviceMock) Start(serviceIDs ...string) error {
	return nil
}

func (mock *serviceMock) getIDs() []string {
	mock.mu.Lock()
	defer mock.mu.Unlock()

	ids := []string{}

	for id := range mock.services {
		ids = append(ids, id)
	}

	sort.Strings(ids)

	return ids
}

func (mock *serviceMock) get(id string) system.Service {
	mock.mu.Lock()
	defer mock.mu.Unlock()

	return mock.services[id]
}

func (suite *ExtensionServiceSuite) TestReconcile() {
	svcMock := &serviceMock{
		services: map[string]system.Service{},
	}

	suite.Require().NoError(suite.runtime.RegisterController(&runtimecontrollers.ExtensionServiceController{
		V1Alpha1Services: svcMock,
		ConfigPath:       "testdata/extservices/",
	}))

	suite.startRuntime()

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			ids := svcMock.getIDs()

			if !reflect.DeepEqual(ids, []string{"ext-hello-world"}) {
				return retry.ExpectedError(fmt.Errorf("services registered: %q", ids))
			}

			return nil
		},
	))

	helloSvc := svcMock.get("ext-hello-world")
	suite.Require().IsType(&services.Extension{}, helloSvc)

	suite.Assert().Equal("./hello-world", helloSvc.(*services.Extension).Spec.Container.Entrypoint)
}

func TestExtensionServiceSuite(t *testing.T) {
	suite.Run(t, new(ExtensionServiceSuite))
}
