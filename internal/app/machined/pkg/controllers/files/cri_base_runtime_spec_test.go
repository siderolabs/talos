// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package files_test

import (
	"encoding/json"
	"io/fs"
	"testing"
	"time"

	"github.com/containerd/containerd/v2/pkg/oci"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	filesctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/files"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/cri"
	"github.com/siderolabs/talos/pkg/machinery/resources/files"
)

type CRIBaseRuntimeSpecSuite struct {
	ctest.DefaultSuite
}

func (suite *CRIBaseRuntimeSpecSuite) TestDefaults() {
	suite.createRuntimeSpecConfig(cri.BaseRuntimeSpecDefaultID, map[string]any{
		"ociVersion": "1.0.2",
		"process": map[string]any{
			"cwd": "/default",
		},
	})

	ctest.AssertResource(suite, constants.CRIBaseRuntimeSpec, func(etcFile *files.EtcFileSpec, asrt *assert.Assertions) {
		contents := etcFile.TypedSpec().Contents

		var ociSpec oci.Spec

		asrt.NoError(json.Unmarshal(contents, &ociSpec))

		asrt.Empty(ociSpec.Process.Rlimits)
		asrt.Equal("/default", ociSpec.Process.Cwd)
		asrt.Equal(fs.FileMode(0o600), etcFile.TypedSpec().Mode)
		asrt.Equal(constants.EtcSelinuxLabel, etcFile.TypedSpec().SelinuxLabel)
	})
}

func (suite *CRIBaseRuntimeSpecSuite) TestOverrides() {
	suite.createRuntimeSpecConfig(cri.BaseRuntimeSpecDefaultID, map[string]any{
		"ociVersion": "1.0.2",
		"process": map[string]any{
			"cwd": "/default",
		},
	})
	suite.createRuntimeSpecConfig(cri.BaseRuntimeSpecOverridesID, map[string]any{
		"process": map[string]any{
			"noNewPrivileges": true,
			"rlimits": []map[string]any{
				{
					"type": "RLIMIT_NOFILE",
					"hard": 1024,
					"soft": 1024,
				},
			},
		},
	})
	ctest.AssertResource(suite, constants.CRIBaseRuntimeSpec, func(etcFile *files.EtcFileSpec, asrt *assert.Assertions) {
		contents := etcFile.TypedSpec().Contents

		var ociSpec oci.Spec

		asrt.NoError(json.Unmarshal(contents, &ociSpec))

		asrt.Len(ociSpec.Process.Rlimits, 1)
		asrt.Equal("RLIMIT_NOFILE", ociSpec.Process.Rlimits[0].Type)
		asrt.Equal(uint64(1024), ociSpec.Process.Rlimits[0].Hard)
		asrt.Equal(uint64(1024), ociSpec.Process.Rlimits[0].Soft)
		asrt.Equal("/default", ociSpec.Process.Cwd)
		asrt.True(ociSpec.Process.NoNewPrivileges)
	})
}

func (suite *CRIBaseRuntimeSpecSuite) createRuntimeSpecConfig(id resource.ID, spec map[string]any) {
	cfg := cri.NewBaseRuntimeSpecConfig(id)
	cfg.TypedSpec().Object = spec

	suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))
}

func TestCRIBaseRuntimeSpecSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &CRIBaseRuntimeSpecSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 10 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&filesctrl.CRIBaseRuntimeSpecController{}))
			},
		},
	})
}
