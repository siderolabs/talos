// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	secretsctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/secrets"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/types/security"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/files"
)

func TestTrustedRootsSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &TrustedRootsSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 10 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&secretsctrl.TrustedRootsController{}))
			},
		},
	})
}

type TrustedRootsSuite struct {
	ctest.DefaultSuite
}

func (suite *TrustedRootsSuite) TestReconcileDefault() {
	ctest.AssertResources(suite, []string{constants.DefaultTrustedRelativeCAFile}, func(r *files.EtcFileSpec, asrt *assert.Assertions) {
		asrt.EqualValues(0o644, r.TypedSpec().Mode)
		asrt.Contains(string(r.TypedSpec().Contents), "Bundle of CA Root Certificates")
	})
}

func (suite *TrustedRootsSuite) TestReconcileExtraCAs() {
	trustedRoot1 := security.NewTrustedRootsConfigV1Alpha1()
	trustedRoot1.MetaName = "root1"
	trustedRoot1.Certificates = "-- BEGIN1 --"

	trustedRoot2 := security.NewTrustedRootsConfigV1Alpha1()
	trustedRoot2.MetaName = "root2"
	trustedRoot2.Certificates = "-- BEGIN2 --"

	cfg, err := container.New(trustedRoot1, trustedRoot2)
	suite.Require().NoError(err)

	mc := config.NewMachineConfig(cfg)
	suite.Require().NoError(suite.State().Create(suite.Ctx(), mc))

	ctest.AssertResources(suite, []string{constants.DefaultTrustedRelativeCAFile}, func(r *files.EtcFileSpec, asrt *assert.Assertions) {
		asrt.EqualValues(0o644, r.TypedSpec().Mode)

		asrt.Contains(string(r.TypedSpec().Contents), "Bundle of CA Root Certificates")

		for _, contains := range []string{
			trustedRoot1.MetaName,
			trustedRoot1.Certificates,
			trustedRoot2.MetaName,
			trustedRoot2.Certificates,
		} {
			asrt.Contains(string(r.TypedSpec().Contents), contains)
		}
	})
}
