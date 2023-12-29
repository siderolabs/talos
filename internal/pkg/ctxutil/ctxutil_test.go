// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package ctxutil_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/ctxutil"
)

func TestStartFn(t *testing.T) {
	ctx := ctxutil.MonitorFn(context.Background(), func() error { return nil })

	<-ctx.Done()

	require.Equal(t, context.Canceled, ctx.Err())
	require.Nil(t, ctxutil.Cause(ctx))

	myErr := errors.New("my error")

	ctx = ctxutil.MonitorFn(context.Background(), func() error { return myErr })

	<-ctx.Done()

	require.Equal(t, context.Canceled, ctx.Err())
	require.Equal(t, myErr, ctxutil.Cause(ctx))
}
