// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package basic_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/talos/pkg/grpc/middleware/auth/basic"
)

func TestTokenInterceptor(t *testing.T) {
	t.Parallel()

	const validToken = "valid-token"

	creds := basic.NewTokenCredentials(validToken)

	interceptor := creds.UnaryInterceptor()

	for _, test := range []struct {
		name string

		md      metadata.MD
		wantErr bool
	}{
		{
			name: "valid token",

			md: metadata.MD{
				"token": []string{validToken},
			},
			wantErr: false,
		},
		{
			name: "invalid token",

			md: metadata.MD{
				"token": []string{"invalid-token"},
			},
			wantErr: true,
		},
		{
			name:    "missing token",
			md:      metadata.MD{},
			wantErr: true,
		},
		{
			name:    "no metadata",
			md:      nil,
			wantErr: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctx := t.Context()

			if test.md != nil {
				ctx = metadata.NewIncomingContext(ctx, test.md)
			}

			_, err := interceptor(ctx, nil, nil, func(context.Context, any) (any, error) {
				return nil, nil
			})

			if test.wantErr {
				require.Error(t, err)
				assert.Equal(t, codes.Unauthenticated, status.Code(err))
			} else {
				require.NoError(t, err)
			}
		})
	}
}
