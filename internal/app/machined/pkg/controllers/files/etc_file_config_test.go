// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package files_test

import (
	"slices"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	filesctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/files"
	configconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	runtimecfg "github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/files"
)

type EtcFileConfigSuite struct {
	ctest.DefaultSuite

	documents []configconfig.Document
}

func (suite *EtcFileConfigSuite) TestEtcFileConfig() {
	suite.PatchMachineConfig(newEtcFileConfig("nfsmount.conf", "[NFSMount_Global_Options]\n"))

	suite.assertEtcFileSpec("nfsmount.conf", "[NFSMount_Global_Options]\n")

	suite.RemoveMachineConfigDocumentsByName(runtimecfg.EtcFileConfigKind, "nfsmount.conf")

	ctest.AssertNoResource[*files.EtcFileSpec](suite, "nfsmount.conf")

	suite.PatchMachineConfig(newEtcFileConfig("nfsmount.conf", "recreated\n"))

	suite.assertEtcFileSpec("nfsmount.conf", "recreated\n")

	suite.RemoveMachineConfigDocumentsByName(runtimecfg.EtcFileConfigKind, "nfsmount.conf")

	ctest.AssertNoResource[*files.EtcFileSpec](suite, "nfsmount.conf")
}

func (suite *EtcFileConfigSuite) assertEtcFileSpec(id resource.ID, contents string) {
	ctest.AssertResource(suite, id, func(r *files.EtcFileSpec, asrt *assert.Assertions) {
		asrt.Equal(resource.PhaseRunning, r.Metadata().Phase())
		asrt.EqualValues(0o644, r.TypedSpec().Mode)
		asrt.Equal(contents, string(r.TypedSpec().Contents))
		asrt.Equal(constants.EtcSelinuxLabel, r.TypedSpec().SelinuxLabel)
	})
}

func newEtcFileConfig(name, contents string) *runtimecfg.EtcFileConfigV1Alpha1 {
	etcFile := runtimecfg.NewEtcFileConfigV1Alpha1(name)
	etcFile.Contents = contents

	return etcFile
}

func (suite *EtcFileConfigSuite) PatchMachineConfig(documents ...configconfig.Document) {
	suite.documents = append(
		slices.DeleteFunc(suite.documents, func(existing configconfig.Document) bool {
			return slices.ContainsFunc(documents, func(document configconfig.Document) bool {
				if existing.Kind() != document.Kind() {
					return false
				}

				existingNamed, existingOK := existing.(configconfig.NamedDocument)
				documentNamed, documentOK := document.(configconfig.NamedDocument)

				if existingOK != documentOK {
					return false
				}

				return !existingOK || existingNamed.Name() == documentNamed.Name()
			})
		}),
		documents...,
	)

	suite.updateMachineConfig()
}

func (suite *EtcFileConfigSuite) RemoveMachineConfigDocumentsByName(docType string, names ...string) {
	suite.documents = slices.DeleteFunc(suite.documents, func(document configconfig.Document) bool {
		if document.Kind() != docType {
			return false
		}

		namedDocument, ok := document.(configconfig.NamedDocument)
		if !ok {
			return false
		}

		return slices.Contains(names, namedDocument.Name())
	})

	suite.updateMachineConfig()
}

func (suite *EtcFileConfigSuite) updateMachineConfig() {
	cfg, err := container.New(suite.documents...)
	suite.Require().NoError(err)

	machineConfig := config.NewMachineConfig(cfg)

	current, err := safe.StateGetByID[*config.MachineConfig](suite.Ctx(), suite.State(), config.ActiveID)
	if err != nil {
		if state.IsNotFoundError(err) {
			suite.Require().NoError(suite.State().Create(suite.Ctx(), machineConfig))

			return
		}

		suite.Require().NoError(err)
	}

	machineConfig.Metadata().SetVersion(current.Metadata().Version())

	suite.Require().NoError(suite.State().Update(suite.Ctx(), machineConfig))
}

func TestEtcFileConfigSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &EtcFileConfigSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 10 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&filesctrl.EtcFileConfigController{}))
			},
		},
	})
}
