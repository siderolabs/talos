// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package director_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/talos-systems/grpc-proxy/proxy"
	"google.golang.org/grpc/metadata"

	"github.com/talos-systems/talos/internal/app/routerd/pkg/director"
)

type DirectorSuite struct {
	suite.Suite

	router *director.Router
}

func (suite *DirectorSuite) SetupSuite() {
	suite.router = director.NewRouter()
}

func (suite *DirectorSuite) TestRegisterLocalBackend() {
	suite.router.RegisterLocalBackend("a.A", &mockBackend{})
	suite.router.RegisterLocalBackend("b.B", &mockBackend{})

	suite.Require().Panics(func() { suite.router.RegisterLocalBackend("a.A", &mockBackend{}) })
}

func (suite *DirectorSuite) TestDirectorLocal() {
	ctx := context.Background()

	mode, backends, err := suite.router.Director(ctx, "/service.Service/method")
	suite.Assert().Equal(proxy.One2One, mode)
	suite.Assert().Nil(backends)
	suite.Assert().EqualError(err, "rpc error: code = Unknown desc = service service.Service is not defined")

	suite.router.RegisterLocalBackend("service.Service", &mockBackend{target: "local"})

	mode, backends, err = suite.router.Director(ctx, "/service.Service/method")
	suite.Assert().Equal(proxy.One2One, mode)
	suite.Assert().Len(backends, 1)
	suite.Assert().Equal("local", backends[0].(*mockBackend).target)
	suite.Assert().NoError(err)

	ctxProxyFrom := metadata.NewIncomingContext(ctx, metadata.Pairs("proxyfrom", "127.0.0.1"))
	mode, backends, err = suite.router.Director(ctxProxyFrom, "/service.Service/method")
	suite.Assert().Equal(proxy.One2One, mode)
	suite.Assert().Len(backends, 1)
	suite.Assert().Equal("local", backends[0].(*mockBackend).target)
	suite.Assert().NoError(err)

	ctxNoTargets := metadata.NewIncomingContext(ctx, metadata.Pairs(":authority", "127.0.0.1"))
	mode, backends, err = suite.router.Director(ctxNoTargets, "/service.Service/method")
	suite.Assert().Equal(proxy.One2One, mode)
	suite.Assert().Len(backends, 1)
	suite.Assert().Equal("local", backends[0].(*mockBackend).target)
	suite.Assert().NoError(err)
}

func TestDirectorSuite(t *testing.T) {
	suite.Run(t, new(DirectorSuite))
}
