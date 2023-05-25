// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package k8s_test

import (
	"context"
	"fmt"
	"log"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/suite"

	k8sctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/k8s"
	"github.com/siderolabs/talos/pkg/logging"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

type NodeLabelsSuite struct {
	suite.Suite

	state state.State

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	//nolint:containedctx
	ctx       context.Context
	ctxCancel context.CancelFunc
}

func (suite *NodeLabelsSuite) createAndStartRuntime() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	suite.runtime, err = runtime.NewRuntime(suite.state, logging.Wrap(log.Writer()))
	suite.Require().NoError(err)

	suite.Require().NoError(suite.runtime.RegisterController(&k8sctrl.NodeLabelSpecController{}))

	suite.startRuntime()

	suite.setupMachineType()

	suite.createNodename()
}

func (suite *NodeLabelsSuite) SetupTest() {
	suite.createAndStartRuntime()
}

func (suite *NodeLabelsSuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
}

func (suite *NodeLabelsSuite) assertResource(
	md resource.Metadata,
	check func(res resource.Resource) error,
) func() error {
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

func (suite *NodeLabelsSuite) setupMachineType() {
	machineType := config.NewMachineType()
	machineType.SetMachineType(machine.TypeControlPlane)

	suite.Require().NoError(suite.state.Create(suite.ctx, machineType))
}

func mcWithNodeLabels(labels map[string]string) *config.MachineConfig {
	return config.NewMachineConfig(
		container.NewV1Alpha1(
			&v1alpha1.Config{
				MachineConfig: &v1alpha1.MachineConfig{
					MachineNodeLabels: labels,
				},
			}))
}

func (suite *NodeLabelsSuite) createNodeLabelsConfig(labels map[string]string) {
	mc := mcWithNodeLabels(labels)

	suite.Require().NoError(suite.state.Create(suite.ctx, mc))
}

func (suite *NodeLabelsSuite) createNodename() {
	nodeName := k8s.NewNodename(k8s.NamespaceName, k8s.NodenameID)
	suite.Require().NoError(suite.state.Create(suite.ctx, nodeName))
}

func (suite *NodeLabelsSuite) changeNodeLabelsConfig(labels map[string]string) {
	mc := mcWithNodeLabels(labels)

	oldCfg, err := suite.state.Get(suite.ctx, mc.Metadata())
	if err != nil {
		if state.IsNotFoundError(err) {
			suite.Require().NoError(
				suite.state.Create(suite.ctx, mc),
			)

			return
		}

		suite.Require().NoError(err)
	}

	mc.Metadata().SetVersion(oldCfg.Metadata().Version())

	suite.Require().NoError(
		suite.state.Update(suite.ctx, mc),
	)
}

func (suite *NodeLabelsSuite) assertInexistentLabel(expectedLabel string) {
	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				md := resource.NewMetadata(
					k8s.NamespaceName,
					k8s.NodeLabelSpecType,
					expectedLabel,
					resource.VersionUndefined,
				)

				_, err := suite.state.Get(suite.ctx, md)
				if err == nil {
					return retry.ExpectedError(fmt.Errorf("resource should be destroyed: %v", md))
				}

				if !state.IsNotFoundError(err) {
					return err
				}

				return nil
			},
		),
	)
}

func (suite *NodeLabelsSuite) assertLabel(expectedLabel, oldValue, expectedValue string) {
	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertResource(
					resource.NewMetadata(
						k8s.NamespaceName,
						k8s.NodeLabelSpecType,
						expectedLabel,
						resource.VersionUndefined,
					),
					func(res resource.Resource) error {
						spec := res.(*k8s.NodeLabelSpec).TypedSpec()

						suite.Assert().Equal(
							expectedLabel,
							spec.Key,
						)

						if oldValue != "" && spec.Value == oldValue {
							return retry.ExpectedError(fmt.Errorf("old value still set: %q", oldValue))
						}

						suite.Assert().Equal(
							expectedValue,
							spec.Value,
						)

						return nil
					},
				)()
			},
		),
	)
}

func (suite *NodeLabelsSuite) TestAddLabel() {
	// given
	expectedLabel := "expectedLabel"
	expectedValue := "expectedValue"

	// when
	suite.createNodeLabelsConfig(map[string]string{
		expectedLabel: expectedValue,
	})

	// then
	suite.assertLabel(expectedLabel, "", expectedValue)
}

func (suite *NodeLabelsSuite) TestChangeLabel() {
	// given
	expectedLabel := "someLabel"
	oldValue := "oldValue"
	expectedValue := "newValue"

	// when
	suite.createNodeLabelsConfig(map[string]string{
		expectedLabel: oldValue,
	})

	suite.assertLabel(expectedLabel, "", oldValue)

	suite.changeNodeLabelsConfig(map[string]string{
		expectedLabel: expectedValue,
	})

	// then
	suite.assertLabel(expectedLabel, oldValue, expectedValue)
}

func (suite *NodeLabelsSuite) TestDeleteLabel() {
	// given
	expectedLabel := "label"
	expectedValue := "labelValue"

	// when
	suite.createNodeLabelsConfig(map[string]string{
		expectedLabel: expectedValue,
	})
	suite.assertLabel(expectedLabel, "", expectedValue)

	suite.changeNodeLabelsConfig(map[string]string{})

	// then
	suite.assertInexistentLabel(expectedLabel)
}

func (suite *NodeLabelsSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.wg.Wait()
}

func TestNodeLabelsSuite(t *testing.T) {
	suite.Run(t, new(NodeLabelsSuite))
}
