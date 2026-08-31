// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"fmt"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/meta"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// MetaSuite ...
type MetaSuite struct {
	base.APISuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *MetaSuite) SuiteName() string {
	return "api.MetaSuite"
}

// SetupTest ...
func (suite *MetaSuite) SetupTest() {
	if !suite.Capabilities().SupportsMETA {
		suite.T().Skip("META APIs not supported on this node")
	}

	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 15*time.Second)
}

// TearDownTest ...
func (suite *MetaSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestMetaWriteDelete verifies META APIs.
func (suite *MetaSuite) TestMetaWriteDelete() {
	node := suite.RandomDiscoveredNodeInternalIP()
	ctx := client.WithNode(suite.ctx, node)

	const (
		metaValue  = "test-value"
		metaValue2 = "test-value-2"
	)

	suite.Run("regular operations", func() {
		// no value in the initial state
		rtestutils.AssertNoResource[*runtime.MetaKey](ctx, suite.T(), suite.Client.COSI, runtime.MetaKeyTagToID(meta.UserReserved1))

		suite.Require().NoError(suite.Client.MetaWrite(ctx, meta.UserReserved1, []byte(metaValue)))

		// value should appear after write
		rtestutils.AssertResource(ctx, suite.T(), suite.Client.COSI, runtime.MetaKeyTagToID(meta.UserReserved1), func(mt *runtime.MetaKey, asrt *assert.Assertions) {
			asrt.Equal(metaValue, mt.TypedSpec().Value)
		})

		// overwrite value
		suite.Require().NoError(suite.Client.MetaWrite(ctx, meta.UserReserved1, []byte(metaValue2)))

		rtestutils.AssertResource(ctx, suite.T(), suite.Client.COSI, runtime.MetaKeyTagToID(meta.UserReserved1), func(mt *runtime.MetaKey, asrt *assert.Assertions) {
			asrt.Equal(metaValue2, mt.TypedSpec().Value)
		})

		// delete value
		suite.Require().NoError(suite.Client.MetaDelete(ctx, meta.UserReserved1))

		rtestutils.AssertNoResource[*runtime.MetaKey](ctx, suite.T(), suite.Client.COSI, runtime.MetaKeyTagToID(meta.UserReserved1))
	})

	suite.Run("invalid key", func() {
		const invalidKey = 0x100 // above uint8

		// using direct client here as the wrapper limits to uint8
		_, err := suite.Client.MachineClient.MetaWrite(ctx, &machine.MetaWriteRequest{Key: invalidKey, Value: []byte(metaValue)})
		suite.Require().Error(err)
		suite.Assert().Equal(codes.InvalidArgument, status.Code(err))

		_, err = suite.Client.MachineClient.MetaDelete(ctx, &machine.MetaDeleteRequest{Key: invalidKey})
		suite.Require().Error(err)
		suite.Assert().Equal(codes.InvalidArgument, status.Code(err))
	})

	suite.Run("disallowed keys", func() {
		for metaKey := range 256 {
			metaKey := uint8(metaKey)

			if meta.IsAPIWriteable(metaKey) {
				continue
			}

			suite.Run(fmt.Sprintf("key=%02x", metaKey), func() {
				err := suite.Client.MetaWrite(ctx, metaKey, []byte(metaValue))
				suite.Require().Error(err)
				suite.Assert().Equal(codes.PermissionDenied, status.Code(err))

				err = suite.Client.MetaDelete(ctx, metaKey)
				suite.Require().Error(err)
				suite.Assert().Equal(codes.PermissionDenied, status.Code(err))
			})
		}
	})
}

func init() {
	allSuites = append(allSuites, new(MetaSuite))
}
