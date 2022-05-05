// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubeaccess_test

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/suite"
	"github.com/talos-systems/go-retry/retry"

	"github.com/talos-systems/talos/pkg/logging"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/talos-systems/talos/pkg/machinery/resources/config"
)

type KubeaccessSuite struct {
	suite.Suite

	state state.State

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

func (suite *KubeaccessSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	logger := logging.Wrap(log.Writer())

	suite.runtime, err = runtime.NewRuntime(suite.state, logger)
	suite.Require().NoError(err)
}

func (suite *KubeaccessSuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
}

func (suite *KubeaccessSuite) assertResource(md resource.Metadata, check func(res resource.Resource) error) func() error {
	return func() error {
		r, err := suite.state.Get(suite.ctx, md)
		if err != nil {
			if state.IsNotFoundError(err) {
				return retry.ExpectedError(err)
			}

			return err
		}

		return check(r)
	}
}

func (suite *KubeaccessSuite) assertNoResource(md resource.Metadata) func() error {
	return func() error {
		_, err := suite.state.Get(suite.ctx, md)
		if err == nil {
			return retry.ExpectedErrorf("resource %s still exists", md)
		}

		if state.IsNotFoundError(err) {
			return nil
		}

		return err
	}
}

func (suite *KubeaccessSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.wg.Wait()

	// trigger updates in resources to stop watch loops
	err := suite.state.Create(
		context.Background(), config.NewMachineConfig(
			&v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{},
			},
		),
	)
	if state.IsConflictError(err) {
		err = suite.state.Destroy(context.Background(), config.NewMachineConfig(nil).Metadata())
	}

	suite.Assert().NoError(err)
}
