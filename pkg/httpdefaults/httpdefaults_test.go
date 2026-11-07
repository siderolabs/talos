// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package httpdefaults_test

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/httpdefaults"
)

func TestLazyTransport(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)

	var built atomic.Int32

	client := &http.Client{
		Transport: httpdefaults.LazyTransport(func() *http.Transport {
			built.Add(1)

			return &http.Transport{}
		}),
	}

	// closing idle connections before first use doesn't build the transport
	client.CloseIdleConnections()
	assert.EqualValues(t, 0, built.Load())

	var wg sync.WaitGroup

	for range 10 {
		wg.Go(func() {
			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL, nil)
			if !assert.NoError(t, err) {
				return
			}

			resp, err := client.Do(req)
			if !assert.NoError(t, err) {
				return
			}

			assert.NoError(t, resp.Body.Close())
			assert.Equal(t, http.StatusNoContent, resp.StatusCode)
		})
	}

	wg.Wait()

	require.EqualValues(t, 1, built.Load())

	client.CloseIdleConnections()
	assert.EqualValues(t, 1, built.Load())
}
