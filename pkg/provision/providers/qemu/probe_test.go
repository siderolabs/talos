// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package qemu_test

import (
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/provision"
	"github.com/siderolabs/talos/pkg/provision/providers/qemu"
)

func TestProbeHTTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/redirect":
			http.Redirect(writer, request, "/success", http.StatusFound)
		case "/large":
			writer.Write([]byte(strings.Repeat("x", provision.HTTPProbeMaxResponseBody+1))) //nolint:errcheck
		default:
			writer.WriteHeader(http.StatusCreated)

			writer.Write([]byte("probe-ok")) //nolint:errcheck
		}
	}))
	t.Cleanup(server.Close)

	address := server.Listener.Addr().(*net.TCPAddr) //nolint:forcetypeassert
	request := provision.HTTPProbeRequest{
		IP:      netip.MustParseAddr(address.IP.String()),
		Port:    uint16(address.Port),
		Path:    "/success",
		Timeout: time.Second,
	}

	response, err := qemu.ProbeHTTPForTest(t.Context(), request)
	require.NoError(t, err)
	require.Empty(t, response.Failure)
	require.Equal(t, http.StatusCreated, response.StatusCode)
	require.Equal(t, []byte("probe-ok"), response.Body)

	request.Path = "/redirect"

	response, err = qemu.ProbeHTTPForTest(t.Context(), request)
	require.NoError(t, err)
	require.Empty(t, response.Failure)
	require.Equal(t, http.StatusFound, response.StatusCode)

	request.Path = "/large"

	response, err = qemu.ProbeHTTPForTest(t.Context(), request)
	require.NoError(t, err)
	require.Len(t, response.Body, provision.HTTPProbeMaxResponseBody)

	server.Close()

	request.Path = "/success"

	response, err = qemu.ProbeHTTPForTest(t.Context(), request)
	require.NoError(t, err)
	require.NotEmpty(t, response.Failure)
	require.Zero(t, response.StatusCode)
}

func TestProbeHTTPValidation(t *testing.T) {
	_, err := qemu.ProbeHTTPForTest(t.Context(), provision.HTTPProbeRequest{})
	require.ErrorContains(t, err, "IP is required")

	_, err = qemu.ProbeHTTPForTest(t.Context(), provision.HTTPProbeRequest{
		IP:   netip.MustParseAddr("127.0.0.1"),
		Port: 80,
		Path: "relative",
	})
	require.ErrorContains(t, err, "path must start")
}
