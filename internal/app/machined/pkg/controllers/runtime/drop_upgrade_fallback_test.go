// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime"
	machineruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/pkg/meta"
	metaconsts "github.com/siderolabs/talos/pkg/machinery/meta"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

type DropUpgradeFallbackControllerSuite struct {
	ctest.DefaultSuite

	meta *meta.Meta
}

type metaProvider struct {
	meta *meta.Meta
}

func (m metaProvider) Meta() machineruntime.Meta {
	return m.meta
}

func TestUpgradeFallbackControllerSuite(t *testing.T) {
	tmpDir := t.TempDir()

	path := filepath.Join(tmpDir, "meta")

	f, err := os.Create(path)
	require.NoError(t, err)
	require.NoError(t, f.Truncate(1024*1024))
	require.NoError(t, f.Close())

	st := state.WrapCore(namespaced.NewState(inmem.Build))

	m, err := meta.New(t.Context(), st, meta.WithFixedPath(path))
	require.NoError(t, err)

	suite.Run(t, &DropUpgradeFallbackControllerSuite{
		meta: m,
		DefaultSuite: ctest.DefaultSuite{
			AfterSetup: func(s *ctest.DefaultSuite) {
				s.Require().NoError(s.Runtime().RegisterController(&runtime.DropUpgradeFallbackController{
					MetaProvider: metaProvider{meta: m},
				}))
			},
		},
	})
}

func (suite *DropUpgradeFallbackControllerSuite) TestDropUpgradeFallback() {
	_, err := suite.meta.SetTag(suite.Ctx(), metaconsts.Upgrade, "A")
	suite.Require().NoError(err)

	machineStatus := runtimeres.NewMachineStatus()
	machineStatus.TypedSpec().Stage = runtimeres.MachineStageBooting
	machineStatus.TypedSpec().Status.Ready = false
	suite.Require().NoError(suite.State().Create(suite.Ctx(), machineStatus))

	time.Sleep(time.Second)

	// controller should not remove the tag
	val, ok := suite.meta.ReadTag(metaconsts.Upgrade)
	suite.Require().True(ok)
	suite.Require().Equal("A", val)

	// update machine status to ready
	machineStatus.TypedSpec().Status.Ready = true
	machineStatus.TypedSpec().Stage = runtimeres.MachineStageRunning
	suite.Require().NoError(suite.State().Update(suite.Ctx(), machineStatus))

	suite.AssertWithin(time.Second, 10*time.Millisecond, func() error {
		_, ok = suite.meta.ReadTag(metaconsts.Upgrade)
		if ok {
			return retry.ExpectedErrorf("tag is still present")
		}

		return nil
	})
}
