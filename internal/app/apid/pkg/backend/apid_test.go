// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package backend_test

import (
	"context"
	"crypto/tls"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"github.com/talos-systems/grpc-proxy/proxy"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"

	"github.com/talos-systems/talos/internal/app/apid/pkg/backend"
	"github.com/talos-systems/talos/pkg/grpc/middleware/authz"
	"github.com/talos-systems/talos/pkg/machinery/api/common"
	"github.com/talos-systems/talos/pkg/machinery/proto"
	"github.com/talos-systems/talos/pkg/machinery/role"
)

func TestAPIDInterfaces(t *testing.T) {
	assert.Implements(t, (*proxy.Backend)(nil), new(backend.APID))
}

type APIDSuite struct {
	suite.Suite

	b *backend.APID
}

func (suite *APIDSuite) SetupSuite() {
	var err error
	suite.b, err = backend.NewAPID("127.0.0.1", credentials.NewTLS(&tls.Config{}))
	suite.Require().NoError(err)
}

func (suite *APIDSuite) TestGetConnection() {
	md1 := metadata.New(nil)
	md1.Set(":authority", "127.0.0.2")
	md1.Set("nodes", "127.0.0.1")
	md1.Set("key", "value1", "value2")
	ctx1 := metadata.NewIncomingContext(authz.ContextWithRoles(context.Background(), role.MakeSet(role.Admin)), md1)

	outCtx1, conn1, err1 := suite.b.GetConnection(ctx1)
	suite.Require().NoError(err1)
	suite.Assert().NotNil(conn1)
	suite.Assert().Equal(role.MakeSet(role.Admin), authz.GetRoles(outCtx1))

	mdOut1, ok1 := metadata.FromOutgoingContext(outCtx1)
	suite.Require().True(ok1)
	suite.Assert().Equal([]string{"value1", "value2"}, mdOut1.Get("key"))
	suite.Assert().Equal([]string{"127.0.0.2"}, mdOut1.Get("proxyfrom"))
	suite.Assert().Equal([]string{"os:admin"}, mdOut1.Get("talos-role"))

	suite.Run("Same context", func() {
		ctx2 := ctx1
		outCtx2, conn2, err2 := suite.b.GetConnection(ctx2)
		suite.Require().NoError(err2)
		suite.Assert().Equal(conn1, conn2) // connection is cached
		suite.Assert().Equal(role.MakeSet(role.Admin), authz.GetRoles(outCtx2))

		mdOut2, ok2 := metadata.FromOutgoingContext(outCtx2)
		suite.Require().True(ok2)
		suite.Assert().Equal([]string{"value1", "value2"}, mdOut2.Get("key"))
		suite.Assert().Equal([]string{"127.0.0.2"}, mdOut2.Get("proxyfrom"))
		suite.Assert().Equal([]string{"os:admin"}, mdOut2.Get("talos-role"))
	})

	suite.Run("Other context", func() {
		md3 := metadata.New(nil)
		md3.Set(":authority", "127.0.0.2")
		md3.Set("nodes", "127.0.0.1")
		md3.Set("key", "value3", "value4")
		ctx3 := metadata.NewIncomingContext(authz.ContextWithRoles(context.Background(), role.MakeSet(role.Reader)), md3)

		outCtx3, conn3, err3 := suite.b.GetConnection(ctx3)
		suite.Require().NoError(err3)
		suite.Assert().Equal(conn1, conn3) // connection is cached
		suite.Assert().Equal(role.MakeSet(role.Reader), authz.GetRoles(outCtx3))

		mdOut3, ok3 := metadata.FromOutgoingContext(outCtx3)
		suite.Require().True(ok3)
		suite.Assert().Equal([]string{"value3", "value4"}, mdOut3.Get("key"))
		suite.Assert().Equal([]string{"127.0.0.2"}, mdOut3.Get("proxyfrom"))
		suite.Assert().Equal([]string{"os:reader"}, mdOut3.Get("talos-role"))
	})
}

func (suite *APIDSuite) TestAppendInfoUnary() {
	reply := &common.DataResponse{
		Messages: []*common.Data{
			{
				Bytes: []byte("foobar"),
			},
		},
	}

	resp, err := proto.Marshal(reply)
	suite.Require().NoError(err)

	newResp, err := suite.b.AppendInfo(false, resp)
	suite.Require().NoError(err)

	var newReply common.DataResponse
	err = proto.Unmarshal(newResp, &newReply)
	suite.Require().NoError(err)

	suite.Assert().EqualValues([]byte("foobar"), newReply.Messages[0].Bytes)
	suite.Assert().Equal(suite.b.String(), newReply.Messages[0].Metadata.Hostname)
	suite.Assert().Empty(newReply.Messages[0].Metadata.Error)
}

func (suite *APIDSuite) TestAppendInfoStreaming() {
	response := &common.Data{
		Bytes: []byte("foobar"),
	}

	resp, err := proto.Marshal(response)
	suite.Require().NoError(err)

	newResp, err := suite.b.AppendInfo(true, resp)
	suite.Require().NoError(err)

	var newResponse common.Data
	err = proto.Unmarshal(newResp, &newResponse)
	suite.Require().NoError(err)

	suite.Assert().EqualValues([]byte("foobar"), newResponse.Bytes)
	suite.Assert().Equal(suite.b.String(), newResponse.Metadata.Hostname)
	suite.Assert().Empty(newResponse.Metadata.Error)
}

func (suite *APIDSuite) TestAppendInfoStreamingMetadata() {
	// this tests the case when metadata field is appended twice
	// to the message, but protobuf merges definitions
	response := &common.Data{
		Metadata: &common.Metadata{
			Error: "something went wrong",
		},
	}

	resp, err := proto.Marshal(response)
	suite.Require().NoError(err)

	newResp, err := suite.b.AppendInfo(true, resp)
	suite.Require().NoError(err)

	var newResponse common.Data
	err = proto.Unmarshal(newResp, &newResponse)
	suite.Require().NoError(err)

	suite.Assert().Nil(newResponse.Bytes)
	suite.Assert().Equal(suite.b.String(), newResponse.Metadata.Hostname)
	suite.Assert().Equal("something went wrong", newResponse.Metadata.Error)
}

func (suite *APIDSuite) TestBuildErrorUnary() {
	resp, err := suite.b.BuildError(false, errors.New("some error"))
	suite.Require().NoError(err)

	var reply common.DataResponse
	err = proto.Unmarshal(resp, &reply)
	suite.Require().NoError(err)

	suite.Assert().Nil(reply.Messages[0].Bytes)
	suite.Assert().Equal(suite.b.String(), reply.Messages[0].Metadata.Hostname)
	suite.Assert().Equal("some error", reply.Messages[0].Metadata.Error)
}

func (suite *APIDSuite) TestBuildErrorStreaming() {
	resp, err := suite.b.BuildError(true, errors.New("some error"))
	suite.Require().NoError(err)

	var response common.Data
	err = proto.Unmarshal(resp, &response)
	suite.Require().NoError(err)

	suite.Assert().Nil(response.Bytes)
	suite.Assert().Equal(suite.b.String(), response.Metadata.Hostname)
	suite.Assert().Equal("some error", response.Metadata.Error)
}

func TestAPIDSuite(t *testing.T) {
	suite.Run(t, new(APIDSuite))
}
