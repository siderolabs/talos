// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/suite"
	"github.com/talos-systems/go-retry/retry"

	clusterctrl "github.com/talos-systems/talos/internal/app/machined/pkg/controllers/cluster"
	v1alpha1runtime "github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/machinery/resources/cluster"
	"github.com/talos-systems/talos/pkg/machinery/resources/files"
	runtimeres "github.com/talos-systems/talos/pkg/machinery/resources/runtime"
	"github.com/talos-systems/talos/pkg/machinery/resources/v1alpha1"
)

type NodeIdentitySuite struct {
	ClusterSuite

	statePath string
}

func (suite *NodeIdentitySuite) TestContainerMode() {
	suite.statePath = suite.T().TempDir()
	suite.startRuntime()

	suite.Require().NoError(suite.runtime.RegisterController(&clusterctrl.NodeIdentityController{
		StatePath:    suite.statePath,
		V1Alpha1Mode: v1alpha1runtime.ModeContainer,
	}))

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		suite.assertResource(*cluster.NewIdentity(cluster.NamespaceName, cluster.LocalIdentity).Metadata(), func(_ resource.Resource) error {
			return nil
		}),
	))
}

func (suite *NodeIdentitySuite) TestDefault() {
	suite.statePath = suite.T().TempDir()
	suite.startRuntime()

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
		suite.assertResource(*cluster.NewIdentity(cluster.NamespaceName, cluster.LocalIdentity).Metadata(), func(_ resource.Resource) error {
			return nil
		}),
	))

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		suite.assertResource(*files.NewEtcFileSpec(files.NamespaceName, "machine-id").Metadata(), func(_ resource.Resource) error {
			return nil
		}),
	))
}

func (suite *NodeIdentitySuite) TestLoad() {
	suite.statePath = suite.T().TempDir()
	suite.startRuntime()

	suite.Require().NoError(suite.runtime.RegisterController(&clusterctrl.NodeIdentityController{
		StatePath:    suite.statePath,
		V1Alpha1Mode: v1alpha1runtime.ModeMetal,
	}))

	// using verbatim data here to make sure nodeId representation is supported in future version fo Talos
	suite.Require().NoError(os.WriteFile(filepath.Join(suite.statePath, constants.NodeIdentityFilename), []byte("nodeId: gvqfS27LxD58lPlASmpaueeRVzuof16iXoieRgEvBWaE\n"), 0o600))

	stateMount := runtimeres.NewMountStatus(v1alpha1.NamespaceName, constants.StatePartitionLabel)

	suite.Assert().NoError(suite.state.Create(suite.ctx, stateMount))

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		suite.assertResource(*cluster.NewIdentity(cluster.NamespaceName, cluster.LocalIdentity).Metadata(), func(r resource.Resource) error {
			suite.Assert().Equal("gvqfS27LxD58lPlASmpaueeRVzuof16iXoieRgEvBWaE", r.(*cluster.Identity).TypedSpec().NodeID)

			return nil
		}),
	))

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		suite.assertResource(*files.NewEtcFileSpec(files.NamespaceName, "machine-id").Metadata(), func(r resource.Resource) error {
			suite.Assert().Equal("8d2c0de2408fa2a178bad7f45d9aa8fb", string(r.(*files.EtcFileSpec).TypedSpec().Contents))

			return nil
		}),
	))
}

func TestNodeIdentitySuite(t *testing.T) {
	suite.Run(t, new(NodeIdentitySuite))
}
