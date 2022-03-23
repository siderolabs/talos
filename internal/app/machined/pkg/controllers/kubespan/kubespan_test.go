// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubespan_test

import (
	"context"
	"log"
	"reflect"
	"sort"
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

type KubeSpanSuite struct {
	suite.Suite

	state state.State

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

func (suite *KubeSpanSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	logger := logging.Wrap(log.Writer())

	suite.runtime, err = runtime.NewRuntime(suite.state, logger)
	suite.Require().NoError(err)
}

func (suite *KubeSpanSuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
}

func (suite *KubeSpanSuite) assertResourceIDs(md resource.Metadata, expectedIDs []resource.ID) func() error {
	return func() error {
		l, err := suite.state.List(suite.ctx, md)
		if err != nil {
			return err
		}

		actualIDs := make([]resource.ID, 0, len(l.Items))

		for _, r := range l.Items {
			actualIDs = append(actualIDs, r.Metadata().ID())
		}

		sort.Strings(expectedIDs)

		if !reflect.DeepEqual(actualIDs, expectedIDs) {
			return retry.ExpectedErrorf("ids do no match expected %v != actual %v", expectedIDs, actualIDs)
		}

		return nil
	}
}

func (suite *KubeSpanSuite) assertNoResource(md resource.Metadata) func() error {
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

func (suite *KubeSpanSuite) assertNoResourceType(md resource.Metadata) func() error {
	return func() error {
		list, err := suite.state.List(suite.ctx, md)
		if err != nil {
			return err
		}

		if len(list.Items) > 0 {
			return retry.ExpectedErrorf("resource list is not empty: %d items", len(list.Items))
		}

		return nil
	}
}

func (suite *KubeSpanSuite) assertResource(md resource.Metadata, check func(res resource.Resource) error) func() error {
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

func (suite *KubeSpanSuite) TearDownTest() {
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
