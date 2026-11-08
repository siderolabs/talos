// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	runtimecontrollers "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	runtimeconfig "github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

type SELinuxBooleanSuite struct {
	ctest.DefaultSuite

	booleans, commit string
}

func TestSELinuxBooleanSuite(t *testing.T) {
	s := &SELinuxBooleanSuite{}

	s.DefaultSuite = ctest.DefaultSuite{
		AfterSetup: func(suite *ctest.DefaultSuite) {
			// a selinuxfs whose booleans hold "current pending", as the kernel shows them
			dir := suite.T().TempDir()
			s.booleans, s.commit = filepath.Join(dir, "booleans"), filepath.Join(dir, "commit_pending_bools")

			suite.Require().NoError(os.Mkdir(s.booleans, 0o755))
			suite.Require().NoError(os.WriteFile(filepath.Join(s.booleans, "pod_use_ebpf"), []byte("1 1"), 0o644))
			suite.Require().NoError(os.WriteFile(filepath.Join(s.booleans, "pod_use_network_fs"), []byte("1 1"), 0o644))
			suite.Require().NoError(os.WriteFile(filepath.Join(s.booleans, "ext_agent_clients"), []byte("0 0"), 0o644))

			suite.Require().NoError(suite.Runtime().RegisterController(&runtimecontrollers.SELinuxBooleanController{
				BooleansPath: s.booleans,
				CommitPath:   s.commit,
			}))
		},
	}

	suite.Run(t, s)
}

func (suite *SELinuxBooleanSuite) applyBooleans(booleans map[string]bool) {
	profile := runtimeconfig.NewSecurityProfileConfigV1Alpha1()
	profile.SELinuxBooleansConfig = booleans

	cntr, err := container.New(profile)
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(cntr)

	if existing, err := suite.State().Get(suite.Ctx(), cfg.Metadata()); err == nil {
		cfg.Metadata().SetVersion(existing.Metadata().Version())
		suite.Update(cfg)
	} else {
		suite.Create(cfg)
	}
}

func (suite *SELinuxBooleanSuite) assertBoolean(name string, want runtimeres.SELinuxBooleanStatusSpec) {
	rtestutils.AssertResource(suite.Ctx(), suite.T(), suite.State(), name, func(res *runtimeres.SELinuxBooleanStatus, asrt *assert.Assertions) {
		asrt.Equal(want, *res.TypedSpec())
	})
}

func (suite *SELinuxBooleanSuite) TestDefaults() {
	suite.assertBoolean("pod_use_ebpf", runtimeres.SELinuxBooleanStatusSpec{Value: true, Default: true})
	suite.assertBoolean("ext_agent_clients", runtimeres.SELinuxBooleanStatusSpec{})
}

func (suite *SELinuxBooleanSuite) TestConfigured() {
	suite.applyBooleans(map[string]bool{"pod_use_ebpf": false, "ext_agent_clients": true, "pod_nonexistent": true})

	suite.assertBoolean("pod_use_ebpf", runtimeres.SELinuxBooleanStatusSpec{Default: true, Configured: true})
	suite.assertBoolean("ext_agent_clients", runtimeres.SELinuxBooleanStatusSpec{Value: true, Configured: true})
	suite.assertBoolean("pod_use_network_fs", runtimeres.SELinuxBooleanStatusSpec{Value: true, Default: true})
	suite.assertBoolean("pod_nonexistent", runtimeres.SELinuxBooleanStatusSpec{Configured: true, Unknown: true})

	commit, err := os.ReadFile(suite.commit)
	suite.Require().NoError(err)
	suite.Assert().Equal("1", string(commit))

	// a boolean left out of the machine config goes back to its default
	suite.applyBooleans(map[string]bool{"ext_agent_clients": true})

	suite.assertBoolean("pod_use_ebpf", runtimeres.SELinuxBooleanStatusSpec{Value: true, Default: true})
	rtestutils.AssertNoResource[*runtimeres.SELinuxBooleanStatus](suite.Ctx(), suite.T(), suite.State(), "pod_nonexistent")
}
