// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package backend_test

import (
	"context"
	"crypto/tls"
	"errors"
	"testing"

	"github.com/AlekSi/pointer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/talos-systems/grpc-proxy/proxy"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	protobuf "google.golang.org/protobuf/proto" //nolint:depguard,gci
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/talos-systems/talos/internal/app/apid/pkg/backend"
	"github.com/talos-systems/talos/pkg/grpc/middleware/authz"
	"github.com/talos-systems/talos/pkg/machinery/api/cluster"
	"github.com/talos-systems/talos/pkg/machinery/api/common"
	"github.com/talos-systems/talos/pkg/machinery/api/inspect"
	"github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/api/resource"
	"github.com/talos-systems/talos/pkg/machinery/api/security"
	"github.com/talos-systems/talos/pkg/machinery/api/storage"
	"github.com/talos-systems/talos/pkg/machinery/api/time"
	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/proto"
	"github.com/talos-systems/talos/pkg/machinery/role"
	"github.com/talos-systems/talos/pkg/version"
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

func TestAPIIdiosyncrasies(t *testing.T) {
	for _, services := range []protoreflect.ServiceDescriptors{
		common.File_common_common_proto.Services(),
		cluster.File_cluster_cluster_proto.Services(),
		inspect.File_inspect_inspect_proto.Services(),
		machine.File_machine_machine_proto.Services(),
		resource.File_resource_resource_proto.Services(),
		// security.File_security_security_proto.Services() is different
		storage.File_storage_storage_proto.Services(),
		time.File_time_time_proto.Services(),
	} {
		for i := 0; i < services.Len(); i++ {
			service := services.Get(i)
			methods := service.Methods()

			for j := 0; j < methods.Len(); j++ {
				method := methods.Get(j)

				t.Run(string(method.FullName()), func(t *testing.T) {
					response := method.Output()
					responseFields := response.Fields()

					if method.IsStreamingServer() {
						metadata := responseFields.Get(0)
						assert.Equal(t, "metadata", metadata.TextName())
						assert.Equal(t, 1, int(metadata.Number()))
					} else {
						require.Equal(t, 1, responseFields.Len(), "unary responses should have exactly one field")

						messages := responseFields.Get(0)
						assert.Equal(t, "messages", messages.TextName())
						assert.Equal(t, 1, int(messages.Number()))

						reply := messages.Message()
						replyFields := reply.Fields()
						require.GreaterOrEqual(t, replyFields.Len(), 1, "unary replies should have at least one field")

						metadata := replyFields.Get(0)
						assert.Equal(t, "metadata", metadata.TextName())
						assert.Equal(t, 1, int(metadata.Number()))
					}
				})
			}
		}
	}
}

//nolint:nakedret,gocyclo,errcheck,forcetypeassert
func getOptions(t *testing.T, descriptor protoreflect.Descriptor) (deprecated bool, version string) {
	switch opts := descriptor.Options().(type) {
	case *descriptorpb.EnumOptions:
		if opts != nil {
			deprecated = pointer.GetBool(opts.Deprecated)
			version = protobuf.GetExtension(opts, common.E_RemoveDeprecatedEnum).(string)
		}
	case *descriptorpb.EnumValueOptions:
		if opts != nil {
			deprecated = pointer.GetBool(opts.Deprecated)
			version = protobuf.GetExtension(opts, common.E_RemoveDeprecatedEnumValue).(string)
		}
	case *descriptorpb.MessageOptions:
		if opts != nil {
			deprecated = pointer.GetBool(opts.Deprecated)
			version = protobuf.GetExtension(opts, common.E_RemoveDeprecatedMessage).(string)
		}
	case *descriptorpb.FieldOptions:
		if opts != nil {
			deprecated = pointer.GetBool(opts.Deprecated)
			version = protobuf.GetExtension(opts, common.E_RemoveDeprecatedField).(string)
		}
	case *descriptorpb.ServiceOptions:
		if opts != nil {
			deprecated = pointer.GetBool(opts.Deprecated)
			version = protobuf.GetExtension(opts, common.E_RemoveDeprecatedService).(string)
		}
	case *descriptorpb.MethodOptions:
		if opts != nil {
			deprecated = pointer.GetBool(opts.Deprecated)
			version = protobuf.GetExtension(opts, common.E_RemoveDeprecatedMethod).(string)
		}

	default:
		t.Fatalf("unhandled %T", opts)
	}

	return
}

func testDeprecated(t *testing.T, descriptor protoreflect.Descriptor, currentVersion *config.VersionContract) {
	deprecated, version := getOptions(t, descriptor)

	assert.Equal(t, deprecated, version != "",
		"%s: `deprecated` and `remove_deprecated_XXX_in` options should be used together", descriptor.FullName())

	if !deprecated || version == "" {
		return
	}

	v, err := config.ParseContractFromVersion(version)
	require.NoError(t, err, "%s", descriptor.FullName())

	assert.True(t, v.Greater(currentVersion), "%s should be removed in this version", descriptor.FullName())
}

func testEnum(t *testing.T, enum protoreflect.EnumDescriptor, currentVersion *config.VersionContract) {
	testDeprecated(t, enum, currentVersion)

	values := enum.Values()
	for i := 0; i < values.Len(); i++ {
		testDeprecated(t, values.Get(i), currentVersion)
	}
}

func testMessage(t *testing.T, message protoreflect.MessageDescriptor, currentVersion *config.VersionContract) {
	testDeprecated(t, message, currentVersion)

	fields := message.Fields()
	for i := 0; i < fields.Len(); i++ {
		testDeprecated(t, fields.Get(i), currentVersion)
	}

	oneofs := message.Oneofs()
	for i := 0; i < oneofs.Len(); i++ {
		testDeprecated(t, oneofs.Get(i), currentVersion)
	}

	enums := message.Enums()
	for i := 0; i < enums.Len(); i++ {
		testEnum(t, enums.Get(i), currentVersion)
	}

	// test nested messages
	messages := message.Messages()
	for i := 0; i < messages.Len(); i++ {
		testMessage(t, messages.Get(i), currentVersion)
	}
}

func TestDeprecatedAPIs(t *testing.T) {
	currentVersion, err := config.ParseContractFromVersion(version.Tag)
	require.NoError(t, err)

	for _, file := range []protoreflect.FileDescriptor{
		common.File_common_common_proto,
		cluster.File_cluster_cluster_proto,
		inspect.File_inspect_inspect_proto,
		machine.File_machine_machine_proto,
		resource.File_resource_resource_proto,
		security.File_security_security_proto,
		storage.File_storage_storage_proto,
		time.File_time_time_proto,
	} {
		enums := file.Enums()
		for i := 0; i < enums.Len(); i++ {
			testEnum(t, enums.Get(i), currentVersion)
		}

		messages := file.Messages()
		for i := 0; i < messages.Len(); i++ {
			testMessage(t, messages.Get(i), currentVersion)
		}

		services := file.Services()
		for i := 0; i < services.Len(); i++ {
			service := services.Get(i)
			testDeprecated(t, service, currentVersion)

			methods := service.Methods()
			for j := 0; j < methods.Len(); j++ {
				method := methods.Get(j)
				testDeprecated(t, method, currentVersion)

				message := method.Input()
				testMessage(t, message, currentVersion)

				message = method.Output()
				testMessage(t, message, currentVersion)
			}
		}
	}
}
