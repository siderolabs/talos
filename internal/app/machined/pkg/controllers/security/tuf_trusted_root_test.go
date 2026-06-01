// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package security_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	securityctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/security"
	"github.com/siderolabs/talos/pkg/machinery/resources/security"
)

type TUFTrustedRootSuite struct {
	ctest.DefaultSuite
}

func (suite *TUFTrustedRootSuite) TestReconcileNoConfig() {
	ctest.AssertNoResource[*security.TUFTrustedRoot](suite, security.TrustedRootID)
}

func (suite *TUFTrustedRootSuite) TestReconcileNoKeylessRules() {
	rule := security.NewImageVerificationRule("0000")
	rule.TypedSpec().ImagePattern = "ghcr.io/myorg/*"
	suite.Create(rule)

	ctest.AssertNoResource[*security.TUFTrustedRoot](suite, security.TrustedRootID)
}

func (suite *TUFTrustedRootSuite) TestReconcileWithRules() {
	rule := security.NewImageVerificationRule("0000")
	rule.TypedSpec().ImagePattern = "ghcr.io/myorg/*"
	rule.TypedSpec().KeylessVerifier = &security.ImageKeylessVerifierSpec{
		Issuer:       "https://token.actions.githubusercontent.com",
		SubjectRegex: "https://github.com/myorg/.*",
	}
	suite.Create(rule)

	ctest.AssertResource(suite, security.TrustedRootID, func(root *security.TUFTrustedRoot, asrt *assert.Assertions) {
		asrt.NotEmpty(root.TypedSpec().JSONData)
	})

	suite.Destroy(rule)

	ctest.AssertNoResource[*security.TUFTrustedRoot](suite, security.TrustedRootID)
}

func TestTUFTrustedRootSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &TUFTrustedRootSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(s *ctest.DefaultSuite) {
				s.Require().NoError(s.Runtime().RegisterController(&securityctrl.TUFTrustedRootController{}))
			},
		},
	})
}
