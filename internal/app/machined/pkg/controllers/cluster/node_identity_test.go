// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster_test

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/suite"
	"github.com/talos-systems/go-retry/retry"

	clusterctrl "github.com/talos-systems/talos/internal/app/machined/pkg/controllers/cluster"
	v1alpha1runtime "github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/pkg/logging"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/resources/cluster"
	runtimeres "github.com/talos-systems/talos/pkg/resources/runtime"
	"github.com/talos-systems/talos/pkg/resources/v1alpha1"
)

type NodeIdentitySuite struct {
	suite.Suite

	state state.State

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context
	ctxCancel context.CancelFunc

	statePath string
}

func (suite *NodeIdentitySuite) SetupTest() {
	suite.statePath = suite.T().TempDir()

	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	suite.runtime, err = runtime.NewRuntime(suite.state, logging.Wrap(log.Writer()))
	suite.Require().NoError(err)

	suite.startRuntime()
}

func (suite *NodeIdentitySuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
}

func (suite *NodeIdentitySuite) assertNodeIdentities(expected []string) error {
	resources, err := suite.state.List(suite.ctx, resource.NewMetadata(cluster.NamespaceName, cluster.IdentityType, "", resource.VersionUndefined))
	if err != nil {
		return err
	}

	ids := make([]string, 0, len(resources.Items))

	for _, res := range resources.Items {
		ids = append(ids, res.Metadata().ID())
	}

	if !reflect.DeepEqual(expected, ids) {
		return retry.ExpectedError(fmt.Errorf("expected %q, got %q", expected, ids))
	}

	return nil
}

func (suite *NodeIdentitySuite) TestContainerMode() {
	suite.Require().NoError(suite.runtime.RegisterController(&clusterctrl.NodeIdentityController{
		StatePath:    suite.statePath,
		V1Alpha1Mode: v1alpha1runtime.ModeContainer,
	}))

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertNodeIdentities([]string{cluster.LocalIdentity})
		},
	))
}

func (suite *NodeIdentitySuite) TestDefault() {
	suite.Require().NoError(suite.runtime.RegisterController(&clusterctrl.NodeIdentityController{
		StatePath:    suite.statePath,
		V1Alpha1Mode: v1alpha1runtime.ModeMetal,
	}))

	time.Sleep(500 * time.Millisecond)

	_, err := suite.state.Get(suite.ctx, cluster.NewIdentity(cluster.NamespaceName, cluster.LocalIdentity).Metadata())
	suite.Assert().True(state.IsNotFoundError(err))

	stateMount := runtimeres.NewMountStatus(v1alpha1.NamespaceName, constants.StatePartitionLabel)

	suite.Assert().NoError(suite.state.Create(suite.ctx, stateMount))

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertNodeIdentities([]string{cluster.LocalIdentity})
		},
	))
}

func (suite *NodeIdentitySuite) TestLoad() {
	suite.Require().NoError(suite.runtime.RegisterController(&clusterctrl.NodeIdentityController{
		StatePath:    suite.statePath,
		V1Alpha1Mode: v1alpha1runtime.ModeMetal,
	}))

	// using verbatim data here to make sure nodeId representation is supported in future version fo Talos
	suite.Require().NoError(os.WriteFile(filepath.Join(suite.statePath, constants.NodeIdentityFilename), []byte("nodeId: gvqfS27LxD58lPlASmpaueeRVzuof16iXoieRgEvBWaE\n"), 0o600))

	stateMount := runtimeres.NewMountStatus(v1alpha1.NamespaceName, constants.StatePartitionLabel)

	suite.Assert().NoError(suite.state.Create(suite.ctx, stateMount))

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertNodeIdentities([]string{cluster.LocalIdentity})
		},
	))

	r, err := suite.state.Get(suite.ctx, cluster.NewIdentity(cluster.NamespaceName, cluster.LocalIdentity).Metadata())
	suite.Require().NoError(err)

	suite.Assert().Equal("gvqfS27LxD58lPlASmpaueeRVzuof16iXoieRgEvBWaE", r.(*cluster.Identity).TypedSpec().NodeID)
}

func (suite *NodeIdentitySuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.wg.Wait()

	// trigger updates in resources to stop watch loops
	suite.Assert().NoError(suite.state.Create(context.Background(), runtimeres.NewMountStatus(v1alpha1.NamespaceName, "-")))
}

func TestNodeIdentitySuite(t *testing.T) {
	suite.Run(t, new(NodeIdentitySuite))
}
