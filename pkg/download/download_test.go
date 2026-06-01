// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package download_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/download"
)

type flipper struct {
	srv   *httptest.Server
	val   int
	sleep time.Duration
}

func (f *flipper) EndpointFunc() func(context.Context) (string, error) {
	return func(context.Context) (string, error) {
		time.Sleep(f.sleep)

		f.val++

		if f.val%2 == 1 {
			return f.srv.URL + "/404", nil
		}

		return f.srv.URL + "/data", nil
	}
}

func TestDownload(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/empty":
			w.WriteHeader(http.StatusOK)
		case "/data":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("data")) //nolint:errcheck
		case "/base64":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ZGF0YQ==")) //nolint:errcheck
		case "/400":
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintln(w, "bad request")
		case "/404":
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintln(w, "not found")
		case "/204":
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	t.Cleanup(srv.Close)

	flip := flipper{
		srv: srv,
	}

	sleepingFlip := flipper{
		srv:   srv,
		sleep: 100 * time.Millisecond,
	}

	for _, test := range []struct {
		name string
		path string
		opts []download.Option

		expected      string
		expectedError string
	}{
		{
			name:     "empty download",
			path:     "/empty",
			expected: "",
		},
		{
			name:     "empty download with 204",
			path:     "/204",
			expected: "",
		},
		{
			name:     "some data",
			path:     "/data",
			expected: "data",
		},
		{
			name:     "base64",
			path:     "/base64",
			opts:     []download.Option{download.WithFormat("base64")},
			expected: "data",
		},
		{
			name:          "empty error",
			path:          "/empty",
			opts:          []download.Option{download.WithErrorOnEmptyResponse(errors.New("empty response"))},
			expectedError: "empty response",
		},
		{
			name:          "empty error by 204",
			path:          "/204",
			opts:          []download.Option{download.WithErrorOnEmptyResponse(errors.New("empty response"))},
			expectedError: "empty response",
		},
		{
			name:          "not found error",
			path:          "/404",
			opts:          []download.Option{download.WithErrorOnNotFound(errors.New("gone forever"))},
			expectedError: "gone forever",
		},
		{
			name:          "bad request error",
			path:          "/400",
			opts:          []download.Option{download.WithErrorOnBadRequest(errors.New("bad req"))},
			expectedError: "bad req",
		},
		{
			name:          "failure 404",
			path:          "/404",
			opts:          []download.Option{download.WithTimeout(2 * time.Second)},
			expectedError: "failed to download config, status code 404, body \"not found\\n\"",
		},
		{
			name:          "failure 400",
			path:          "/400",
			opts:          []download.Option{download.WithTimeout(2 * time.Second)},
			expectedError: "failed to download config, status code 400, body \"bad request\\n\"",
		},
		{
			name: "retry endpoint change",
			opts: []download.Option{
				download.WithTimeout(2 * time.Second),
				download.WithEndpointFunc(flip.EndpointFunc()),
			},
			expected: "data",
		},
		{
			name: "retry with attempt timeout",
			opts: []download.Option{
				download.WithTimeout(2 * time.Second),
				download.WithEndpointFunc(sleepingFlip.EndpointFunc()),
				download.WithRetryOptions(retry.WithAttemptTimeout(200 * time.Millisecond)),
			},
			expected: "data",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()

			b, err := download.Download(ctx, srv.URL+test.path, test.opts...)

			if test.expectedError != "" {
				assert.ErrorContains(t, err, test.expectedError)
			} else {
				require.NoError(t, err)

				assert.Equal(t, test.expected, string(b))
			}
		})
	}
}
