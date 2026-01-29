// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package director_test

import (
	"context"
	"regexp"
	"testing"

	"github.com/siderolabs/grpc-proxy/proxy"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/talos/internal/app/apid/pkg/director"
)

type DirectorSuite struct {
	suite.Suite

	localBackend *mockBackend
	router       *director.Router
}

func (suite *DirectorSuite) SetupSuite() {
	suite.localBackend = &mockBackend{}
	suite.router = director.NewRouter(
		mockBackendFactory,
		suite.localBackend,
		&mockLocalAddressProvider{
			local: map[string]struct{}{
				"localhost": {},
			},
		},
	)
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
	mode, backends, err := suite.router.Director(metadata.NewIncomingContext(ctx, md), "/machine.MachineService/method")
	suite.Require().NoError(err)
	suite.Assert().Equal(proxy.One2Many, mode)
	suite.Assert().Len(backends, 2)
	suite.Assert().Equal("127.0.0.1", backends[0].(*mockBackend).target)
	suite.Assert().Equal("127.0.0.2", backends[1].(*mockBackend).target)

	md = metadata.New(nil)
	md.Set("nodes", "127.0.0.1")
	mode, backends, err = suite.router.Director(metadata.NewIncomingContext(ctx, md), "/machine.MachineService/method")
	suite.Require().NoError(err)
	suite.Assert().Equal(proxy.One2Many, mode)
	suite.Assert().Len(backends, 1)
	suite.Assert().Equal("127.0.0.1", backends[0].(*mockBackend).target)
}

func (suite *DirectorSuite) TestDirectorSingleNode() {
	ctx := context.Background()

	md := metadata.New(nil)
	md.Set("node", "127.0.0.1")
	mode, backends, err := suite.router.Director(metadata.NewIncomingContext(ctx, md), "/service.Service/method")
	suite.Assert().Equal(proxy.One2One, mode)
	suite.Assert().Len(backends, 1)
	suite.Assert().Equal("127.0.0.1", backends[0].(*mockBackend).target)
	suite.Assert().NoError(err)

	md = metadata.New(nil)
	md.Set("node", "127.0.0.1", "127.0.0.2")
	_, _, err = suite.router.Director(metadata.NewIncomingContext(ctx, md), "/service.Service/method")
	suite.Assert().Equal(codes.InvalidArgument, status.Code(err))
}

func (suite *DirectorSuite) TestDirectorLocal() {
	ctx := context.Background()

	md := metadata.New(nil)
	mode, backends, err := suite.router.Director(metadata.NewIncomingContext(ctx, md), "/service.Service/method")
	suite.Assert().Equal(proxy.One2One, mode)
	suite.Assert().Len(backends, 1)
	suite.Assert().Equal(suite.localBackend, backends[0])
	suite.Assert().NoError(err)
}

func (suite *DirectorSuite) TestDirectorNoRemoteBackend() {
	// override the router to have no remote backends
	router := director.NewRouter(
		nil,
		suite.localBackend,
		&mockLocalAddressProvider{
			local: map[string]struct{}{
				"localhost": {},
			},
		},
	)

	ctx := context.Background()

	// request forwarding via node/nodes is disabled
	md := metadata.New(nil)
	md.Set("node", "127.0.0.1")
	_, _, err := router.Director(metadata.NewIncomingContext(ctx, md), "/service.Service/method")
	suite.Require().Error(err)
	suite.Assert().Equal(codes.PermissionDenied, status.Code(err))

	md = metadata.New(nil)
	md.Set("nodes", "127.0.0.1", "127.0.0.2")
	_, _, err = router.Director(metadata.NewIncomingContext(ctx, md), "/machine.MachineService/method")
	suite.Require().Error(err)
	suite.Assert().Equal(codes.PermissionDenied, status.Code(err))

	// no request forwarding, allowed
	md = metadata.New(nil)
	mode, backends, err := router.Director(metadata.NewIncomingContext(ctx, md), "/service.Service/method")
	suite.Require().NoError(err)
	suite.Assert().Equal(proxy.One2One, mode)
	suite.Assert().Len(backends, 1)
	suite.Assert().Equal(suite.localBackend, backends[0])

	// request forwarding to local address, allowed
	md = metadata.New(nil)
	md.Set("node", "localhost")
	mode, backends, err = router.Director(metadata.NewIncomingContext(ctx, md), "/service.Service/method")
	suite.Require().NoError(err)
	suite.Assert().Equal(proxy.One2One, mode)
	suite.Assert().Len(backends, 1)
	suite.Assert().Equal(suite.localBackend, backends[0])

	md = metadata.New(nil)
	md.Set("nodes", "localhost")
	mode, backends, err = router.Director(metadata.NewIncomingContext(ctx, md), "/service.Service/method")
	suite.Require().NoError(err)
	suite.Assert().Equal(proxy.One2One, mode)
	suite.Assert().Len(backends, 1)
	suite.Assert().Equal(suite.localBackend, backends[0])
}

func TestDirectorSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(DirectorSuite))
}
