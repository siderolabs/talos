// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hcloud_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network/operator/vip/internal/hcloud"
)

type recordedRequest struct {
	header http.Header
	method string
	path   string
	query  string
	body   string
}

type fakeAPI struct {
	handler func(w http.ResponseWriter, r *http.Request, body string)

	mu       sync.Mutex
	requests []recordedRequest
}

func (f *fakeAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body) //nolint:errcheck

	f.mu.Lock()
	f.requests = append(f.requests, recordedRequest{
		header: r.Header.Clone(),
		method: r.Method,
		path:   r.URL.Path,
		query:  r.URL.RawQuery,
		body:   string(body),
	})
	f.mu.Unlock()

	f.handler(w, r, string(body))
}

func (f *fakeAPI) recorded() []recordedRequest {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]recordedRequest(nil), f.requests...)
}

func newClient(t *testing.T, handler func(w http.ResponseWriter, r *http.Request, body string)) (*hcloud.Client, *fakeAPI) {
	t.Helper()

	api := &fakeAPI{handler: handler}

	srv := httptest.NewServer(api)
	t.Cleanup(srv.Close)

	return hcloud.NewClient(
		"secret-token",
		hcloud.WithEndpoint(srv.URL+"/v1"),
		hcloud.WithUserAgent("talos/test"),
		hcloud.WithBackoff(func(int) time.Duration { return 0 }),
	), api
}

func writeJSON(w http.ResponseWriter, status int, v string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write([]byte(v)) //nolint:errcheck
}

func TestAllServersPagination(t *testing.T) {
	t.Parallel()

	client, api := newClient(t, func(w http.ResponseWriter, r *http.Request, _ string) {
		switch r.URL.Query().Get("page") {
		case "1":
			writeJSON(w, http.StatusOK, `{"servers":[{"id":1,"private_net":[{"network":10,"ip":"10.0.0.2","alias_ips":["10.0.0.100"]}]}],
				"meta":{"pagination":{"page":1,"per_page":50,"next_page":2,"last_page":2}}}`)
		case "2":
			writeJSON(w, http.StatusOK, `{"servers":[{"id":2,"private_net":[]}],
				"meta":{"pagination":{"page":2,"per_page":50,"next_page":null,"last_page":2}}}`)
		default:
			writeJSON(w, http.StatusBadRequest, `{"error":{"code":"invalid_input","message":"bad page"}}`)
		}
	})

	servers, err := client.AllServers(t.Context())
	require.NoError(t, err)

	require.Len(t, servers, 2)
	assert.Equal(t, int64(1), servers[0].ID)
	assert.Equal(t, []hcloud.ServerPrivateNet{{Network: 10, IP: "10.0.0.2", AliasIPs: []string{"10.0.0.100"}}}, servers[0].PrivateNet)
	assert.Equal(t, int64(2), servers[1].ID)

	requests := api.recorded()
	require.Len(t, requests, 2)

	for i, req := range requests {
		assert.Equal(t, http.MethodGet, req.method)
		assert.Equal(t, "/v1/servers", req.path)
		assert.Equal(t, "page="+strconv.Itoa(i+1)+"&per_page=50", req.query)
		assert.Equal(t, "Bearer secret-token", req.header.Get("Authorization"))
		assert.Equal(t, "talos/test", req.header.Get("User-Agent"))
	}
}

func TestAllFloatingIPsNoMeta(t *testing.T) {
	t.Parallel()

	client, _ := newClient(t, func(w http.ResponseWriter, _ *http.Request, _ string) {
		writeJSON(w, http.StatusOK, `{"floating_ips":[
			{"id":5,"ip":"1.2.3.4","type":"ipv4","server":7},
			{"id":6,"ip":"2001:db8:1::/64","type":"ipv6","server":null}
		]}`)
	})

	floatingIPs, err := client.AllFloatingIPs(t.Context())
	require.NoError(t, err)

	require.Len(t, floatingIPs, 2)
	assert.Equal(t, int64(5), floatingIPs[0].ID)
	require.NotNil(t, floatingIPs[0].Server)
	assert.Equal(t, int64(7), *floatingIPs[0].Server)
	assert.Nil(t, floatingIPs[1].Server)
}

func TestGetters(t *testing.T) {
	t.Parallel()

	client, _ := newClient(t, func(w http.ResponseWriter, r *http.Request, _ string) {
		switch r.URL.Path {
		case "/v1/servers/42":
			writeJSON(w, http.StatusOK, `{"server":{"id":42,"private_net":[{"network":10,"ip":"10.0.0.2","alias_ips":[]}]}}`)
		case "/v1/networks/10":
			writeJSON(w, http.StatusOK, `{"network":{"id":10,"ip_range":"10.0.0.0/16"}}`)
		case "/v1/floating_ips/5":
			writeJSON(w, http.StatusOK, `{"floating_ip":{"id":5,"ip":"1.2.3.4","type":"ipv4","server":42}}`)
		default:
			writeJSON(w, http.StatusNotFound, `{"error":{"code":"not_found","message":"resource not found"}}`)
		}
	})

	server, err := client.GetServer(t.Context(), 42)
	require.NoError(t, err)
	assert.Equal(t, int64(42), server.ID)
	require.Len(t, server.PrivateNet, 1)
	assert.Equal(t, int64(10), server.PrivateNet[0].Network)

	network, err := client.GetNetwork(t.Context(), 10)
	require.NoError(t, err)
	assert.Equal(t, "10.0.0.0/16", network.IPRange)

	floatingIP, err := client.GetFloatingIP(t.Context(), 5)
	require.NoError(t, err)
	require.NotNil(t, floatingIP.Server)
	assert.Equal(t, int64(42), *floatingIP.Server)

	_, err = client.GetServer(t.Context(), 43)
	require.EqualError(t, err, "resource not found (not_found)")
}

func TestActions(t *testing.T) {
	t.Parallel()

	client, api := newClient(t, func(w http.ResponseWriter, _ *http.Request, _ string) {
		writeJSON(w, http.StatusCreated, `{"action":{"id":100,"command":"some_action","status":"running"}}`)
	})

	action, err := client.ChangeAliasIPs(t.Context(), 42, 10, []string{"10.0.0.100"})
	require.NoError(t, err)
	assert.Equal(t, "running", action.Status)

	_, err = client.ChangeAliasIPs(t.Context(), 43, 10, nil)
	require.NoError(t, err)

	_, err = client.AssignFloatingIP(t.Context(), 5, 42)
	require.NoError(t, err)

	requests := api.recorded()
	require.Len(t, requests, 3)

	for _, req := range requests {
		assert.Equal(t, http.MethodPost, req.method)
		assert.Equal(t, "application/json", req.header.Get("Content-Type"))
	}

	assert.Equal(t, "/v1/servers/42/actions/change_alias_ips", requests[0].path)
	assert.JSONEq(t, `{"network":10,"alias_ips":["10.0.0.100"]}`, requests[0].body)

	// clearing alias IPs sends an empty list, not null
	assert.Equal(t, "/v1/servers/43/actions/change_alias_ips", requests[1].path)
	assert.JSONEq(t, `{"network":10,"alias_ips":[]}`, requests[1].body)

	assert.Equal(t, "/v1/floating_ips/5/actions/assign", requests[2].path)
	assert.JSONEq(t, `{"server":42}`, requests[2].body)
}

func TestRetries(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		failures   []func(w http.ResponseWriter)
		failAlways func(w http.ResponseWriter)

		expectedRequests int
		expectedError    string
	}{
		{
			name: "conflict and rate limit are retried",
			failures: []func(w http.ResponseWriter){
				func(w http.ResponseWriter) {
					writeJSON(w, http.StatusConflict, `{"error":{"code":"conflict","message":"resource is locked"}}`)
				},
				func(w http.ResponseWriter) {
					writeJSON(w, http.StatusTooManyRequests, `{"error":{"code":"rate_limit_exceeded","message":"slow down"}}`)
				},
			},
			expectedRequests: 3,
		},
		{
			name: "gateway errors without API error are retried",
			failures: []func(w http.ResponseWriter){
				func(w http.ResponseWriter) { w.WriteHeader(http.StatusBadGateway) },
				func(w http.ResponseWriter) { w.WriteHeader(http.StatusGatewayTimeout) },
			},
			expectedRequests: 3,
		},
		{
			name: "invalid input is not retried",
			failAlways: func(w http.ResponseWriter) {
				writeJSON(w, http.StatusBadRequest, `{"error":{"code":"invalid_input","message":"invalid input in field 'network'"}}`)
			},
			expectedRequests: 1,
			expectedError:    "invalid input in field 'network' (invalid_input)",
		},
		{
			name: "internal server error is not retried",
			failAlways: func(w http.ResponseWriter) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			expectedRequests: 1,
			expectedError:    "server responded with status code 500 ()",
		},
		{
			name: "retries are limited",
			failAlways: func(w http.ResponseWriter) {
				writeJSON(w, http.StatusConflict, `{"error":{"code":"conflict","message":"resource is locked"}}`)
			},
			expectedRequests: 6,
			expectedError:    "resource is locked (conflict)",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var (
				mu    sync.Mutex
				count int
			)

			client, api := newClient(t, func(w http.ResponseWriter, _ *http.Request, _ string) {
				mu.Lock()
				attempt := count
				count++
				mu.Unlock()

				switch {
				case test.failAlways != nil:
					test.failAlways(w)
				case attempt < len(test.failures):
					test.failures[attempt](w)
				default:
					writeJSON(w, http.StatusCreated, `{"action":{"id":100,"command":"assign_floating_ip","status":"success"}}`)
				}
			})

			action, err := client.AssignFloatingIP(t.Context(), 5, 42)

			requests := api.recorded()
			assert.Len(t, requests, test.expectedRequests)

			// the request body is re-sent on each retry
			for _, req := range requests {
				assert.JSONEq(t, `{"server":42}`, req.body)
			}

			if test.expectedError != "" {
				require.EqualError(t, err, test.expectedError)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, "success", action.Status)
		})
	}
}

func TestRetryCanceled(t *testing.T) {
	t.Parallel()

	api := &fakeAPI{handler: func(w http.ResponseWriter, _ *http.Request, _ string) {
		writeJSON(w, http.StatusConflict, `{"error":{"code":"conflict","message":"resource is locked"}}`)
	}}

	srv := httptest.NewServer(api)
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithCancel(t.Context())

	client := hcloud.NewClient("token", hcloud.WithEndpoint(srv.URL), hcloud.WithBackoff(func(int) time.Duration {
		cancel()

		return time.Hour
	}))

	_, err := client.GetServer(ctx, 1)
	require.EqualError(t, err, "resource is locked (conflict)")
	assert.Len(t, api.recorded(), 1)
}

func TestFindServerByAliasIP(t *testing.T) {
	t.Parallel()

	servers := []hcloud.Server{
		{ID: 1, PrivateNet: []hcloud.ServerPrivateNet{{Network: 10, AliasIPs: []string{"10.0.0.101"}}}},
		{ID: 2, PrivateNet: []hcloud.ServerPrivateNet{{Network: 20, AliasIPs: []string{"10.0.0.100"}}}},
		{ID: 3, PrivateNet: []hcloud.ServerPrivateNet{{Network: 10, AliasIPs: []string{"invalid", "10.0.0.100"}}}},
	}

	assert.Equal(t, int64(3), hcloud.FindServerByAliasIP(servers, 10, netip.MustParseAddr("10.0.0.100")))
	assert.Equal(t, int64(2), hcloud.FindServerByAliasIP(servers, 20, netip.MustParseAddr("10.0.0.100")))
	assert.Equal(t, int64(0), hcloud.FindServerByAliasIP(servers, 30, netip.MustParseAddr("10.0.0.100")))
	assert.Equal(t, int64(0), hcloud.FindServerByAliasIP(servers, 10, netip.MustParseAddr("10.0.0.102")))
}

func TestFindFloatingIP(t *testing.T) {
	t.Parallel()

	floatingIPs := []hcloud.FloatingIP{
		{ID: 1, IP: "1.2.3.4", Type: "ipv4"},
		{ID: 2, IP: "2001:db8:1::/64", Type: "ipv6"},
		{ID: 3, IP: "garbage", Type: "ipv4"},
	}

	found := hcloud.FindFloatingIP(floatingIPs, netip.MustParseAddr("1.2.3.4"))
	require.NotNil(t, found)
	assert.Equal(t, int64(1), found.ID)

	found = hcloud.FindFloatingIP(floatingIPs, netip.MustParseAddr("2001:db8:1::"))
	require.NotNil(t, found)
	assert.Equal(t, int64(2), found.ID)

	assert.Nil(t, hcloud.FindFloatingIP(floatingIPs, netip.MustParseAddr("2001:db8:1::1")))
	assert.Nil(t, hcloud.FindFloatingIP(floatingIPs, netip.MustParseAddr("5.6.7.8")))
}

func TestNetworkContains(t *testing.T) {
	t.Parallel()

	network := hcloud.Network{IPRange: "10.0.0.0/16"}

	assert.True(t, network.Contains(netip.MustParseAddr("10.0.1.1")))
	assert.False(t, network.Contains(netip.MustParseAddr("10.1.0.1")))
	assert.False(t, (&hcloud.Network{IPRange: "garbage"}).Contains(netip.MustParseAddr("10.0.1.1")))
}
