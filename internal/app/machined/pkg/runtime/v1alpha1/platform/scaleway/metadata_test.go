// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package scaleway //nolint:testpackage // Exercise endpoint failover and parsing without exporting test-only APIs.

import (
	"bytes"
	"context"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/scaleway/scaleway-sdk-go/api/instance/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	platformerrors "github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	"github.com/siderolabs/talos/pkg/download"
)

func TestDownloadAlternatingNoConfigReturnsImmediately(t *testing.T) {
	for _, tt := range []struct {
		name   string
		status int
	}{
		{name: "not found", status: http.StatusNotFound},
		{name: "no content", status: http.StatusNoContent},
		{name: "empty", status: http.StatusOK},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var ipv4Requests, ipv6Requests atomic.Int32

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/v4":
					ipv4Requests.Add(1)
					w.WriteHeader(tt.status)
				case "/v6":
					ipv6Requests.Add(1)

					_, err := w.Write([]byte("user data"))
					assert.NoError(t, err)
				}
			}))
			t.Cleanup(server.Close)

			data, err := (&Scaleway{}).downloadAlternating(t.Context(), server.URL+"/v4", server.URL+"/v6",
				download.WithErrorOnNotFound(platformerrors.ErrNoConfigSource),
				download.WithErrorOnEmptyResponse(platformerrors.ErrNoConfigSource),
			)

			assert.Nil(t, data)
			assert.Equal(t, platformerrors.ErrNoConfigSource, err)
			assert.EqualValues(t, 1, ipv4Requests.Load())
			assert.Zero(t, ipv6Requests.Load())
		})
	}
}

func TestDownloadAlternatingFallsBackAndReusesSuccessfulFamily(t *testing.T) {
	var ipv4Requests, ipv6Requests atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v4":
			ipv4Requests.Add(1)
			w.WriteHeader(http.StatusBadRequest)
		case "/v6":
			ipv6Requests.Add(1)

			_, err := w.Write([]byte("metadata"))
			assert.NoError(t, err)
		}
	}))
	t.Cleanup(server.Close)

	s := &Scaleway{}
	options := []download.Option{download.WithErrorOnBadRequest(errors.New("bad request"))}

	data, err := s.downloadAlternating(t.Context(), server.URL+"/v4", server.URL+"/v6", options...)
	require.NoError(t, err)
	assert.Equal(t, "metadata", string(data))
	assert.EqualValues(t, 1, ipv4Requests.Load())
	assert.EqualValues(t, 1, ipv6Requests.Load())

	data, err = s.downloadAlternating(t.Context(), server.URL+"/v4", server.URL+"/v6", options...)
	require.NoError(t, err)
	assert.Equal(t, "metadata", string(data))
	assert.EqualValues(t, 1, ipv4Requests.Load())
	assert.EqualValues(t, 2, ipv6Requests.Load())
}

func TestDownloadAlternatingErrorsNameAttemptedEndpoints(t *testing.T) {
	var logs bytes.Buffer

	previousLogWriter := log.Writer()

	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(previousLogWriter) })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	t.Cleanup(server.Close)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	ipv4Endpoint := server.URL + "/actual-v4"
	ipv6Endpoint := server.URL + "/actual-v6"
	_, err := (&Scaleway{}).downloadAlternating(ctx, ipv4Endpoint, ipv6Endpoint,
		download.WithErrorOnBadRequest(errors.New("bad request")),
	)
	require.Error(t, err)

	assert.ErrorContains(t, err, ipv4Endpoint)
	assert.ErrorContains(t, err, ipv6Endpoint)
	assert.Contains(t, logs.String(), ipv4Endpoint)
	assert.Contains(t, logs.String(), ipv6Endpoint)
}

func TestDownloadAlternatingBoundsUnresponsiveEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v4" {
			<-r.Context().Done()

			return
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	ctx, cancel := context.WithTimeout(t.Context(), 3*endpointAttemptTimeout)
	defer cancel()

	_, err := (&Scaleway{}).downloadAlternating(ctx, server.URL+"/v4", server.URL+"/v6",
		download.WithErrorOnEmptyResponse(platformerrors.ErrNoConfigSource),
	)
	assert.Equal(t, platformerrors.ErrNoConfigSource, err)
	assert.NoError(t, ctx.Err())
}

func TestDownloadAlternatingCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		cancel()
		<-r.Context().Done()
	}))
	t.Cleanup(server.Close)

	defer cancel()

	_, err := (&Scaleway{}).downloadAlternating(ctx, server.URL, server.URL)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestParseIPv6MetadataIPValidation(t *testing.T) {
	for _, tt := range []struct {
		name    string
		ip      instance.MetadataIP
		wantErr bool
	}{
		{name: "IPv6 prefix", ip: instance.MetadataIP{Address: "2001:db8::1", Netmask: "64", Gateway: "fe80::1"}},
		{name: "IPv6 mask", ip: instance.MetadataIP{Address: "2001:db8::1", Netmask: "ffff:ffff:ffff:ffff::"}},
		{name: "empty address", ip: instance.MetadataIP{Netmask: "64"}, wantErr: true},
		{name: "empty mask", ip: instance.MetadataIP{Address: "2001:db8::1"}, wantErr: true},
		{name: "negative prefix", ip: instance.MetadataIP{Address: "2001:db8::1", Netmask: "-1"}, wantErr: true},
		{name: "oversized prefix", ip: instance.MetadataIP{Address: "2001:db8::1", Netmask: "129"}, wantErr: true},
		{name: "wrong address family", ip: instance.MetadataIP{Address: "192.0.2.1", Netmask: "32"}, wantErr: true},
		{name: "noncontiguous mask", ip: instance.MetadataIP{Address: "2001:db8::1", Netmask: "ffff:0:ffff::"}, wantErr: true},
		{name: "mapped address", ip: instance.MetadataIP{Address: "::ffff:192.0.2.1", Netmask: "128"}, wantErr: true},
		{name: "wrong mask family", ip: instance.MetadataIP{Address: "2001:db8::1", Netmask: "255.255.255.0"}, wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			prefix, _, err := parseIPv6MetadataIP(tt.ip)
			if tt.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.True(t, prefix.IsValid())
		})
	}
}
