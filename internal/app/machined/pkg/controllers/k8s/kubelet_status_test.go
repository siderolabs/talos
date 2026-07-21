// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	k8sctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

type KubeletStatusControllerSuite struct {
	ctest.DefaultSuite
}

func (suite *KubeletStatusControllerSuite) TestReconcile() {
	kubeletSpec := k8s.NewKubeletSpec(k8s.NamespaceName, k8s.KubeletID)
	kubeletSpec.TypedSpec().Image = "ghcr.io/siderolabs/kubelet:v1.34.0"
	kubeletSpec.TypedSpec().ExpectedNodename = "example-node"

	suite.Create(kubeletSpec)

	ctest.AssertResource(suite, k8s.KubeletID, func(status *k8s.KubeletStatus, asrt *assert.Assertions) {
		asrt.Equal("ghcr.io/siderolabs/kubelet:v1.34.0", status.TypedSpec().Image)
	})

	ctest.UpdateWithConflicts(suite, kubeletSpec, func(spec *k8s.KubeletSpec) error {
		spec.TypedSpec().Image = "ghcr.io/siderolabs/kubelet:v1.35.0"

		return nil
	})

	ctest.AssertResource(suite, k8s.KubeletID, func(status *k8s.KubeletStatus, asrt *assert.Assertions) {
		asrt.Equal("ghcr.io/siderolabs/kubelet:v1.35.0", status.TypedSpec().Image)
	})

	suite.Require().NoError(suite.State().Destroy(suite.Ctx(), kubeletSpec.Metadata()))

	ctest.AssertNoResource[*k8s.KubeletStatus](suite, k8s.KubeletID)
}

func TestKubeletStatusControllerSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &KubeletStatusControllerSuite{
		DefaultSuite: ctest.DefaultSuite{
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(k8sctrl.NewKubeletStatusController()))
			},
		},
	})
}
