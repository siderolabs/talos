// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package client_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/talos-systems/talos/api/common"
	"github.com/talos-systems/talos/cmd/osctl/pkg/client"
)

func TestFilterResponse(t *testing.T) {
	reply := &common.DataReply{
		Response: []*common.DataResponse{
			{
				Metadata: &common.ResponseMetadata{
					Hostname: "host1",
				},
				Bytes: []byte("abc"),
			},
			{
				Metadata: &common.ResponseMetadata{
					Hostname: "host2",
					Error:    "something wrong",
				},
			},
			{
				Bytes: []byte("def"),
			},
			{
				Metadata: &common.ResponseMetadata{
					Hostname: "host4",
					Error:    "even more wrong",
				},
			},
		},
	}

	filtered, err := client.FilterReply(reply, nil)
	assert.EqualError(t, err, "2 errors occurred:\n\t* host2: something wrong\n\t* host4: even more wrong\n\n")
	assert.Equal(t, filtered,
		&common.DataReply{
			Response: []*common.DataResponse{
				{
					Metadata: &common.ResponseMetadata{
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

func TestFilterResponseNil(t *testing.T) {
	e := errors.New("wrong")

	filtered, err := client.FilterReply(nil, e)
	assert.Nil(t, filtered)
	assert.Equal(t, e, err)

	filtered, err = client.FilterReply((*common.DataReply)(nil), e)
	assert.Nil(t, filtered)
	assert.Equal(t, e, err)
}

func TestFilterResponseOnlyErrors(t *testing.T) {
	reply := &common.DataReply{
		Response: []*common.DataResponse{
			{
				Metadata: &common.ResponseMetadata{
					Hostname: "host2",
					Error:    "something wrong",
				},
			},
			{
				Metadata: &common.ResponseMetadata{
					Hostname: "host4",
					Error:    "even more wrong",
				},
			},
		},
	}

	filtered, err := client.FilterReply(reply, nil)
	assert.EqualError(t, err, "2 errors occurred:\n\t* host2: something wrong\n\t* host4: even more wrong\n\n")
	assert.Nil(t, filtered)
}
