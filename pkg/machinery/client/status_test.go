// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package client_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/hashicorp/go-multierror"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/talos/pkg/machinery/client"
)

func TestStatus(t *testing.T) {
	for _, tt := range []struct {
		name      string
		err       error
		nilStatus bool
		message   string
		code      codes.Code
	}{
		{
			name:      "nil",
			err:       nil,
			nilStatus: true,
			code:      codes.OK,
		},
		{
			name:      "not status",
			err:       errors.New("some error"),
			nilStatus: true,
			code:      codes.Unknown,
		},
		{
			name:    "status",
			err:     status.Error(codes.AlreadyExists, "file already exists"),
			message: "file already exists",
			code:    codes.AlreadyExists,
		},
		{
			name:    "status wrapped",
			err:     multierror.Append(nil, status.Error(codes.AlreadyExists, "file already exists")).ErrorOrNil(),
			message: "file already exists",
			code:    codes.AlreadyExists,
		},
		{
			name:    "multiple wrapped",
			err:     multierror.Append(nil, status.Error(codes.FailedPrecondition, "can't be zero"), status.Error(codes.AlreadyExists, "file already exists")).ErrorOrNil(),
			message: "can't be zero",
			code:    codes.FailedPrecondition,
		},
		{
			name:    "double wrapped",
			err:     multierror.Append(nil, fmt.Errorf("127.0.0.1: %w", status.Error(codes.AlreadyExists, "file already exists"))).ErrorOrNil(),
			message: "file already exists",
			code:    codes.AlreadyExists,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			st := client.Status(tt.err)
			if tt.nilStatus {
				assert.Nil(t, st)
			} else {
				assert.Equal(t, st.Message(), tt.message)
				assert.Equal(t, st.Code(), tt.code)
			}

			assert.Equal(t, client.StatusCode(tt.err), tt.code)
		})
	}
}
