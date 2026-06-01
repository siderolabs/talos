// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster_test

import (
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	clusterctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/cluster"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/resources/cluster"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
)

type InfoSuite struct {
	ctest.DefaultSuite
}

func (suite *InfoSuite) TestReconcile() {
	cfg := config.NewMachineConfig(container.NewV1Alpha1(&v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		ClusterConfig: &v1alpha1.ClusterConfig{
			ClusterID:   "cluster1",
			ClusterName: "foo",
		},
	}))

	suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{cluster.InfoID},
		func(res *cluster.Info, asrt *assert.Assertions) {
			spec := res.TypedSpec()

			asrt.Equal("cluster1", spec.ClusterID)
			asrt.Equal("foo", spec.ClusterName)
		})

	suite.Require().NoError(suite.State().Destroy(suite.Ctx(), cfg.Metadata()))

	rtestutils.AssertNoResource[*cluster.Config](suite.Ctx(), suite.T(), suite.State(), cluster.ConfigID)
}

func TestInfoSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &InfoSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(clusterctrl.NewInfoController()))
			},
		},
	})
}
