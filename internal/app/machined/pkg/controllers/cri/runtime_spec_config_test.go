// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cri_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	crictrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/cri"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	criconfig "github.com/siderolabs/talos/pkg/machinery/config/types/cri"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	configres "github.com/siderolabs/talos/pkg/machinery/resources/config"
	crires "github.com/siderolabs/talos/pkg/machinery/resources/cri"
)

type RuntimeSpecConfigSuite struct {
	ctest.DefaultSuite
}

func TestRuntimeSpecConfigSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &RuntimeSpecConfigSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(s *ctest.DefaultSuite) {
				s.Require().NoError(s.Runtime().RegisterController(&crictrl.RuntimeSpecConfigController{}))
			},
		},
	})
}

func (suite *RuntimeSpecConfigSuite) TestProjection() {
	ctest.AssertResource(suite, crires.BaseRuntimeSpecDefaultID, func(r *crires.BaseRuntimeSpecConfig, a *assert.Assertions) {
		process, ok := r.TypedSpec().Object["process"].(map[string]any)
		if !a.True(ok) {
			return
		}

		a.NotContains(process, "rlimits")
	})
	ctest.AssertNoResource[*crires.BaseRuntimeSpecConfig](suite, crires.BaseRuntimeSpecOverridesID)

	legacy := &v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		MachineConfig: &v1alpha1.MachineConfig{
			MachineType: "worker",
			MachineBaseRuntimeSpecOverrides: meta.Unstructured{ //nolint:staticcheck // test deprecated compatibility
				Object: map[string]any{
					"process": map[string]any{
						"cwd":             "/legacy",
						"noNewPrivileges": false,
						"rlimits":         []any{map[string]any{"type": "RLIMIT_NOFILE", "hard": 512, "soft": 512}},
					},
				},
			},
		},
	}

	document := criconfig.NewCRIBaseRuntimeSpecConfigV1Alpha1()
	document.OverridesConfig.Object = map[string]any{
		"process": map[string]any{
			"noNewPrivileges": true,
			"rlimits":         []any{map[string]any{"type": "RLIMIT_NOFILE", "hard": 1024, "soft": 1024}},
		},
	}

	ctr, err := container.New(legacy)
	suite.Require().NoError(err)

	machineConfig := configres.NewMachineConfig(ctr)
	suite.Create(machineConfig)

	ctest.AssertResource(suite, crires.BaseRuntimeSpecOverridesID, func(r *crires.BaseRuntimeSpecConfig, a *assert.Assertions) {
		a.Equal(legacy.MachineConfig.MachineBaseRuntimeSpecOverrides.Object, r.TypedSpec().Object) //nolint:staticcheck // test deprecated compatibility
	})

	ctr, err = container.New(document)
	suite.Require().NoError(err)

	newMachineConfig := configres.NewMachineConfig(ctr)
	newMachineConfig.Metadata().SetVersion(machineConfig.Metadata().Version())
	suite.Update(newMachineConfig)

	ctest.AssertResource(suite, crires.BaseRuntimeSpecOverridesID, func(r *crires.BaseRuntimeSpecConfig, a *assert.Assertions) {
		a.Equal(document.Overrides(), r.TypedSpec().Object)
	})

	suite.Destroy(machineConfig)

	ctest.AssertResource(suite, crires.BaseRuntimeSpecDefaultID, func(r *crires.BaseRuntimeSpecConfig, a *assert.Assertions) {
		a.NotEmpty(r.TypedSpec().Object)
	})
	ctest.AssertNoResource[*crires.BaseRuntimeSpecConfig](suite, crires.BaseRuntimeSpecOverridesID)
}
