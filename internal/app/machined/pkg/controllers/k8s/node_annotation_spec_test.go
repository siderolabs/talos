// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s_test

import (
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	k8sctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/k8s"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/extensions"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

type NodeAnnotationsSuite struct {
	ctest.DefaultSuite
}

func TestNodeAnnotationsSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &NodeAnnotationsSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(s *ctest.DefaultSuite) {
				s.Require().NoError(s.Runtime().RegisterController(&k8sctrl.NodeAnnotationSpecController{}))
			},
		},
	})
}

func (suite *NodeAnnotationsSuite) updateMachineConfig(annotations map[string]string) {
	cfg, err := safe.StateGetByID[*config.MachineConfig](suite.Ctx(), suite.State(), config.ActiveID)
	if err != nil && !state.IsNotFoundError(err) {
		suite.Require().NoError(err)
	}

	if cfg == nil {
		cfg = config.NewMachineConfig(container.NewV1Alpha1(&v1alpha1.Config{
			MachineConfig: &v1alpha1.MachineConfig{
				MachineType:            "controlplane",
				MachineNodeAnnotations: annotations,
			},
		}))

		suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))
	} else {
		cfg.Container().RawV1Alpha1().MachineConfig.MachineNodeAnnotations = annotations
		suite.Require().NoError(suite.State().Update(suite.Ctx(), cfg))
	}
}

func (suite *NodeAnnotationsSuite) TestChangeLabel() {
	// given
	expectedAnnotation := "some/annotation"
	oldValue := "oldValue"
	expectedValue := "newValue"

	// when
	suite.updateMachineConfig(map[string]string{
		expectedAnnotation: oldValue,
	})

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []string{expectedAnnotation},
		func(labelSpec *k8s.NodeAnnotationSpec, asrt *assert.Assertions) {
			asrt.Equal(oldValue, labelSpec.TypedSpec().Value)
		})

	suite.updateMachineConfig(map[string]string{
		expectedAnnotation: expectedValue,
	})

	// then
	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []string{expectedAnnotation},
		func(labelSpec *k8s.NodeAnnotationSpec, asrt *assert.Assertions) {
			asrt.Equal(expectedValue, labelSpec.TypedSpec().Value)
		})
}

func (suite *NodeAnnotationsSuite) TestExtensionAnnotations() {
	ext1 := runtime.NewExtensionStatus(runtime.NamespaceName, "0")
	ext1.TypedSpec().Metadata = extensions.Metadata{
		Name:    "zfs",
		Version: "2.2.4",
	}

	ext2 := runtime.NewExtensionStatus(runtime.NamespaceName, "1")
	ext2.TypedSpec().Metadata = extensions.Metadata{
		Name:    "drbd",
		Version: "9.2.8-v1.7.5",
	}

	ext3 := runtime.NewExtensionStatus(runtime.NamespaceName, "2")
	ext3.TypedSpec().Metadata = extensions.Metadata{
		Name:    "schematic",
		Version: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
	}

	suite.Require().NoError(suite.State().Create(suite.Ctx(), ext1))
	suite.Require().NoError(suite.State().Create(suite.Ctx(), ext2))
	suite.Require().NoError(suite.State().Create(suite.Ctx(), ext3))

	rtestutils.AssertNoResource[*k8s.NodeAnnotationSpec](suite.Ctx(), suite.T(), suite.State(), "extensions.talos.dev/zfs")
	rtestutils.AssertNoResource[*k8s.NodeAnnotationSpec](suite.Ctx(), suite.T(), suite.State(), "extensions.talos.dev/drbd")

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []string{"extensions.talos.dev/schematic"},
		func(labelSpec *k8s.NodeAnnotationSpec, asrt *assert.Assertions) {
			asrt.Equal("e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", labelSpec.TypedSpec().Value)
		})
}
