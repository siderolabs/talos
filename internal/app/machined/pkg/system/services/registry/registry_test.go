// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package registry_test

import (
	"cmp"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/siderolabs/gen/xiter"
	"github.com/siderolabs/gen/xtesting/check"
	"github.com/siderolabs/go-pointer"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/talos/internal/app/machined/pkg/system/services/registry"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

func TestRegistry(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	logger := zaptest.NewLogger(t)
	svc := registry.NewService(registry.NewMultiPathFS(xiter.Single("test")), logger)

	var wg sync.WaitGroup

	wg.Add(1)

	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)

	go func() {
		defer wg.Done()

		cancel(cmp.Or(svc.Run(ctx), errors.New("service exited")))
	}()

	defer wg.Wait()

	time.Sleep(100 * time.Millisecond)

	tests := []struct {
		name         string
		path         string
		method       string
		body         io.Reader
		check        check.Check
		expectedCode int
	}{
		{
			name:         "HEAD /v2/",
			path:         "/v2/",
			method:       http.MethodHead,
			check:        check.NoError(),
			expectedCode: http.StatusOK,
		},
		{
			name:         "GET /v2/",
			path:         "/v2/",
			method:       http.MethodGet,
			check:        check.NoError(),
			expectedCode: http.StatusOK,
		},
		{
			name:         "HEAD /healthz",
			path:         "/healthz",
			method:       http.MethodHead,
			check:        check.NoError(),
			expectedCode: http.StatusOK,
		},
		{
			name:         "GET /healthz",
			path:         "/healthz",
			method:       http.MethodGet,
			check:        check.NoError(),
			expectedCode: http.StatusOK,
		},
		{
			name:         "GET /v2/alpine/manifests/3.20.3",
			path:         "/v2/alpine/manifests/3.20.3",
			method:       http.MethodGet,
			check:        check.NoError(),
			expectedCode: http.StatusBadRequest,
		},
		{
			name:         "GET /v2/alpine/manifests/3.20.3?ns=docker.io",
			path:         "/v2/alpine/manifests/3.20.3?ns=docker.io",
			method:       http.MethodGet,
			check:        check.NoError(),
			expectedCode: http.StatusNotFound,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req, err := http.NewRequestWithContext(ctx, test.method, "http://"+constants.RegistrydListenAddress+test.path, test.body)
			require.NoError(t, err)

			resp, err := http.DefaultClient.Do(req)
			test.check(t, err)

			if resp != nil {
				defer resp.Body.Close() //nolint:errcheck
			}

			require.Equal(t, test.expectedCode, pointer.SafeDeref(resp).StatusCode, "unexpected status code, body is %s", readAll(t, resp))
		})
	}

	cancel(nil)
	wg.Wait()

	err := ctx.Err()
	if err == context.Canceled || err == context.DeadlineExceeded {
		err = nil
	}

	require.NoError(t, err)
}

func readAll(t *testing.T, resp *http.Response) string {
	if resp == nil || resp.Body == nil {
		return "<no response>"
	}

	var builder strings.Builder

	_, err := io.Copy(&builder, resp.Body)
	require.NoError(t, err)

	if builder.String() == "" {
		return "<empty response>"
	}

	return builder.String()
}
