// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package director_test

import (
	"context"
	"regexp"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/talos-systems/grpc-proxy/proxy"
	"google.golang.org/grpc/metadata"

	"github.com/talos-systems/talos/internal/app/apid/pkg/director"
)

type DirectorSuite struct {
	suite.Suite

	router *director.Router
}

func (suite *DirectorSuite) SetupSuite() {
	suite.router = director.NewRouter(mockBackendFactory, &mockBackend{})
}

func (suite *DirectorSuite) TestStreamedDetector() {
	suite.Assert().False(suite.router.StreamedDetector("/service.Service/someMethod"))

	suite.router.RegisterStreamedRegex("^" + regexp.QuoteMeta("/service.Service/someMethod") + "$")

	suite.Assert().True(suite.router.StreamedDetector("/service.Service/someMethod"))
	suite.Assert().False(suite.router.StreamedDetector("/service.Service/someMethod2"))
	suite.Assert().False(suite.router.StreamedDetector("/servicexService/someMethod"))

	suite.router.RegisterStreamedRegex("Stream$")

	suite.Assert().True(suite.router.StreamedDetector("/service.Service/getStream"))
	suite.Assert().False(suite.router.StreamedDetector("/service.Service/getStreamItem"))
}

func (suite *DirectorSuite) TestDirectorAggregate() {
	ctx := context.Background()

	md := metadata.New(nil)
	md.Set("nodes", "127.0.0.1", "127.0.0.2")
	mode, backends, err := suite.router.Director(metadata.NewIncomingContext(ctx, md), "/service.Service/method")
	suite.Assert().Equal(proxy.One2Many, mode)
	suite.Assert().Len(backends, 2)
	suite.Assert().Equal("127.0.0.1", backends[0].(*mockBackend).target)
	suite.Assert().Equal("127.0.0.2", backends[1].(*mockBackend).target)
	suite.Assert().NoError(err)

	md = metadata.New(nil)
	md.Set("nodes", "127.0.0.1")
	mode, backends, err = suite.router.Director(metadata.NewIncomingContext(ctx, md), "/service.Service/method")
	suite.Assert().Equal(proxy.One2Many, mode)
	suite.Assert().Len(backends, 1)
	suite.Assert().Equal("127.0.0.1", backends[0].(*mockBackend).target)
	suite.Assert().NoError(err)
}

func TestDirectorSuite(t *testing.T) {
	suite.Run(t, new(DirectorSuite))
}
