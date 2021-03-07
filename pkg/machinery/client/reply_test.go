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

	"github.com/talos-systems/talos/pkg/machinery/api/common"
	"github.com/talos-systems/talos/pkg/machinery/client"
)

func TestFilterMessages(t *testing.T) {
	reply := &common.DataResponse{
		Messages: []*common.Data{
			{
				Metadata: &common.Metadata{
					Hostname: "host1",
				},
				Bytes: []byte("abc"),
			},
			{
				Metadata: &common.Metadata{
					Hostname: "host2",
					Error:    "something wrong",
				},
			},
			{
				Bytes: []byte("def"),
			},
			{
				Metadata: &common.Metadata{
					Hostname: "host4",
					Error:    "even more wrong",
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
					Metadata: &common.Metadata{
						Hostname: "host1",
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

	filtered, err := client.FilterMessages(nil, e)
	assert.Nil(t, filtered)
	assert.Equal(t, e, err)

	filtered, err = client.FilterMessages((*common.Data)(nil), e)
	assert.Nil(t, filtered)
	assert.Equal(t, e, err)
}

func TestFilterMessagesOnlyErrors(t *testing.T) {
	reply := &common.DataResponse{
		Messages: []*common.Data{
			{
				Metadata: &common.Metadata{
					Hostname: "host2",
					Error:    "something wrong",
				},
			},
			{
				Metadata: &common.Metadata{
					Hostname: "host4",
					Error:    "even more wrong",
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
				Metadata: &common.Metadata{
					Hostname: "host2",
					Error:    "should be ignored",
					Status: &status.Status{
						Code:    int32(codes.Aborted),
						Message: "something aborted",
					},
				},
			},
			{
				Metadata: &common.Metadata{
					Hostname: "host4",
					Error:    "should be ignored",
					Status: &status.Status{
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
