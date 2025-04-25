// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	runtimectrls "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/types/siderolink"
	"github.com/siderolabs/talos/pkg/machinery/meta"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

type UniqueMachineTokenSuite struct {
	ctest.DefaultSuite
}

func TestUniqueMachineTokenSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &UniqueMachineTokenSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&runtimectrls.UniqueMachineTokenController{}))
			},
		},
	})
}

func (suite *UniqueMachineTokenSuite) TestReconcileNoConfig() {
	ctest.AssertNoResource[*runtime.UniqueMachineToken](suite, runtime.UniqueMachineTokenID)

	suite.Create(runtime.NewMetaLoaded())

	ctest.AssertResource(suite, runtime.UniqueMachineTokenID, func(token *runtime.UniqueMachineToken, asrt *assert.Assertions) {
		asrt.Empty(token.TypedSpec().Token)
	})

	metaKey := runtime.NewMetaKey(runtime.NamespaceName, runtime.MetaKeyTagToID(meta.UniqueMachineToken))
	metaKey.TypedSpec().Value = "token1"
	suite.Create(metaKey)

	ctest.AssertResource(suite, runtime.UniqueMachineTokenID, func(token *runtime.UniqueMachineToken, asrt *assert.Assertions) {
		asrt.Equal("token1", token.TypedSpec().Token)
	})
}

func (suite *UniqueMachineTokenSuite) TestReconcileWithConfig() {
	sideroLinkConfig := siderolink.NewConfigV1Alpha1()
	sideroLinkConfig.UniqueTokenConfig = "token2"

	ctr, err := container.New(sideroLinkConfig)
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(ctr)
	suite.Create(cfg)

	ctest.AssertNoResource[*runtime.UniqueMachineToken](suite, runtime.UniqueMachineTokenID)

	suite.Create(runtime.NewMetaLoaded())

	ctest.AssertResource(suite, runtime.UniqueMachineTokenID, func(token *runtime.UniqueMachineToken, asrt *assert.Assertions) {
		asrt.Equal("token2", token.TypedSpec().Token)
	})

	metaKey := runtime.NewMetaKey(runtime.NamespaceName, runtime.MetaKeyTagToID(meta.UniqueMachineToken))
	metaKey.TypedSpec().Value = "token1"
	suite.Create(metaKey)

	ctest.AssertResource(suite, runtime.UniqueMachineTokenID, func(token *runtime.UniqueMachineToken, asrt *assert.Assertions) {
		asrt.Equal("token1", token.TypedSpec().Token)
	})
}
