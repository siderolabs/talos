// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package client_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"

	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"github.com/siderolabs/talos/pkg/machinery/client"
)

func TestFilterMessages(t *testing.T) {
	reply := &common.DataResponse{
		Messages: []*common.Data{
			{
				Metadata: &common.Metadata{ //nolint:staticcheck // testing legacy behavior
					Hostname: "host1", //nolint:staticcheck // testing legacy behavior
				},
				Bytes: []byte("abc"),
			},
			{
				Metadata: &common.Metadata{ //nolint:staticcheck // testing legacy behavior
					Hostname: "host2",           //nolint:staticcheck // testing legacy behavior
					Error:    "something wrong", //nolint:staticcheck // testing legacy behavior
				},
			},
			{
				Bytes: []byte("def"),
			},
			{
				Metadata: &common.Metadata{ //nolint:staticcheck // testing legacy behavior
					Hostname: "host4",           //nolint:staticcheck // testing legacy behavior
					Error:    "even more wrong", //nolint:staticcheck // testing legacy behavior
				},
			},
		},
	}

	filtered, err := client.FilterMessages(reply, nil)
	assert.EqualError(t, err, "2 errors occurred:\n\t* host2: something wrong\n\t* host4: even more wrong\n\n")
	assert.Equal(t, filtered,
		&common.DataResponse{
			Messages: []*common.Data{
				{
					Metadata: &common.Metadata{ //nolint:staticcheck // testing legacy behavior
						Hostname: "host1", //nolint:staticcheck // testing legacy behavior
					},
					Bytes: []byte("abc"),
				},
				{
					Bytes: []byte("def"),
				},
			},
		})
}

func TestFilterMessagesNil(t *testing.T) {
	e := errors.New("wrong")

	filtered, err := client.FilterMessages((*common.DataResponse)(nil), e)
	assert.Nil(t, filtered)
	assert.Equal(t, e, err)
}

func TestFilterMessagesOnlyErrors(t *testing.T) {
	reply := &common.DataResponse{
		Messages: []*common.Data{
			{
				Metadata: &common.Metadata{ //nolint:staticcheck // testing legacy behavior
					Hostname: "host2",           //nolint:staticcheck // testing legacy behavior
					Error:    "something wrong", //nolint:staticcheck // testing legacy behavior
				},
			},
			{
				Metadata: &common.Metadata{ //nolint:staticcheck // testing legacy behavior
					Hostname: "host4",           //nolint:staticcheck // testing legacy behavior
					Error:    "even more wrong", //nolint:staticcheck // testing legacy behavior
				},
			},
		},
	}

	filtered, err := client.FilterMessages(reply, nil)
	assert.EqualError(t, err, "2 errors occurred:\n\t* host2: something wrong\n\t* host4: even more wrong\n\n")
	assert.Nil(t, filtered)
}

func TestFilterMessagesGRPCStatus(t *testing.T) {
	reply := &common.DataResponse{
		Messages: []*common.Data{
			{
				Metadata: &common.Metadata{ //nolint:staticcheck // testing legacy behavior
					Hostname: "host2",             //nolint:staticcheck // testing legacy behavior
					Error:    "should be ignored", //nolint:staticcheck // testing legacy behavior
					Status: &status.Status{ //nolint:staticcheck // testing legacy behavior
						Code:    int32(codes.Aborted),
						Message: "something aborted",
					},
				},
			},
			{
				Metadata: &common.Metadata{ //nolint:staticcheck // testing legacy behavior
					Hostname: "host4",             //nolint:staticcheck // testing legacy behavior
					Error:    "should be ignored", //nolint:staticcheck // testing legacy behavior
					Status: &status.Status{ //nolint:staticcheck // testing legacy behavior
						Code:    int32(codes.Unknown),
						Message: "something went wrong",
					},
				},
			},
		},
	}

	filtered, err := client.FilterMessages(reply, nil)
	assert.EqualError(t, err, "2 errors occurred:\n\t* host2: rpc error: code = Aborted desc = something aborted\n\t* host4: rpc error: code = Unknown desc = something went wrong\n\n")
	assert.Nil(t, filtered)
}
