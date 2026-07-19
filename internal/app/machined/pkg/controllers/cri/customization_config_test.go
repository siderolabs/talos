// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cri_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	crictrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/cri"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	criconfig "github.com/siderolabs/talos/pkg/machinery/config/types/cri"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	configres "github.com/siderolabs/talos/pkg/machinery/resources/config"
	crires "github.com/siderolabs/talos/pkg/machinery/resources/cri"
)

type CustomizationConfigSuite struct {
	ctest.DefaultSuite
}

func TestCustomizationConfigSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &CustomizationConfigSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(s *ctest.DefaultSuite) {
				s.Require().NoError(s.Runtime().RegisterController(&crictrl.CustomizationConfigController{}))
			},
		},
	})
}

func (suite *CustomizationConfigSuite) TestProjection() {
	legacy := &v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		MachineConfig: &v1alpha1.MachineConfig{
			MachineType: "worker",
			MachineFiles: []*v1alpha1.MachineFile{ //nolint:staticcheck // test deprecated compatibility
				{
					FilePath:    filepath.Join("/etc", constants.CRICustomizationConfigPart),
					FileContent: "legacy",
				},
			},
		},
	}

	document := criconfig.NewCRICustomizationConfigV1Alpha1("document")
	document.CustomizationContent = "document"

	ctr, err := container.New(legacy, document)
	suite.Require().NoError(err)

	machineConfig := configres.NewMachineConfig(ctr)
	suite.Create(machineConfig)

	ctest.AssertResource(suite, "customization", func(r *crires.CustomizationConfig, a *assert.Assertions) {
		a.Equal("legacy", r.TypedSpec().Content)
	})
	ctest.AssertResource(suite, "document", func(r *crires.CustomizationConfig, a *assert.Assertions) {
		a.Equal("document", r.TypedSpec().Content)
	})

	// now remove one document, only its resource should be cleaned up
	ctr, err = container.New(legacy)
	suite.Require().NoError(err)

	withoutDocument := configres.NewMachineConfig(ctr)
	withoutDocument.Metadata().SetVersion(machineConfig.Metadata().Version())
	suite.Update(withoutDocument)

	ctest.AssertResource(suite, "customization", func(r *crires.CustomizationConfig, a *assert.Assertions) {
		a.Equal("legacy", r.TypedSpec().Content)
	})
	ctest.AssertNoResource[*crires.CustomizationConfig](suite, "document")

	suite.Destroy(machineConfig)

	ctest.AssertNoResource[*crires.CustomizationConfig](suite, "customization")
	ctest.AssertNoResource[*crires.CustomizationConfig](suite, "document")
}
