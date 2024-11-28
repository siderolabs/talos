// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package files_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/containerd/containerd/v2/pkg/oci"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	filesctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/files"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/files"
)

type CRIBaseRuntimeSpecSuite struct {
	ctest.DefaultSuite
}

func (suite *CRIBaseRuntimeSpecSuite) TestDefaults() {
	cfg := config.NewMachineConfig(
		container.NewV1Alpha1(
			&v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "controlplane",
				},
			},
		),
	)

	suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))

	ctest.AssertResource(suite, constants.CRIBaseRuntimeSpec, func(etcFile *files.EtcFileSpec, asrt *assert.Assertions) {
		contents := etcFile.TypedSpec().Contents

		var ociSpec oci.Spec

		asrt.NoError(json.Unmarshal(contents, &ociSpec))

		asrt.Empty(ociSpec.Process.Rlimits)
	})
}

func (suite *CRIBaseRuntimeSpecSuite) TestOverrides() {
	cfg := config.NewMachineConfig(
		container.NewV1Alpha1(
			&v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "controlplane",
					MachineBaseRuntimeSpecOverrides: v1alpha1.Unstructured{
						Object: map[string]any{
							"process": map[string]any{
								"rlimits": []map[string]any{
									{
										"type": "RLIMIT_NOFILE",
										"hard": 1024,
										"soft": 1024,
									},
								},
							},
						},
					},
				},
			},
		),
	)

	suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))

	ctest.AssertResource(suite, constants.CRIBaseRuntimeSpec, func(etcFile *files.EtcFileSpec, asrt *assert.Assertions) {
		contents := etcFile.TypedSpec().Contents

		var ociSpec oci.Spec

		asrt.NoError(json.Unmarshal(contents, &ociSpec))

		asrt.NotEmpty(ociSpec.Process.Rlimits)
		asrt.Equal("RLIMIT_NOFILE", ociSpec.Process.Rlimits[0].Type)
		asrt.Equal(uint64(1024), ociSpec.Process.Rlimits[0].Hard)
		asrt.Equal(uint64(1024), ociSpec.Process.Rlimits[0].Soft)
	})
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
