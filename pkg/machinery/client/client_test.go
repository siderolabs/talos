// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package client_test

import (
	"context"
	"io"
	"runtime"
	"testing"
	"time"

	"github.com/siderolabs/gen/xtesting/must"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/siderolabs/talos/pkg/machinery/client"
)

func TestNew(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	fCalled := make(chan struct{})

	prev := client.SetClientFinalizer(func(closer io.Closer) error {
		defer close(fCalled)

		require.NoError(t, closer.Close())

		return nil
	})
	defer client.SetClientFinalizer(prev)

	c := must.Value(
		client.New(
			ctx,
			client.WithUnixSocket("/path/to/socket"),
			client.WithGRPCDialOptions(
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			),
		),
	)(t)

	runtime.SetFinalizer(c, func(c *client.Client) { t.Log("client finalizer set and called") })

	for {
		select {
		case <-fCalled:
			t.Log("client conn finalized")

			return
		case <-ctx.Done():
			require.Fail(t, "client finalizer not called")
		default:
			runtime.GC()
		}
	}
}
