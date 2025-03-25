// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package oauth2_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/siderolabs/go-procfs/procfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/metal/oauth2"
)

func TestNewConfig(t *testing.T) { //nolint:tparallel
	t.Parallel()

	for _, test := range []struct {
		name string

		cmdline  string
		expected *oauth2.Config
	}{
		{
			name: "no config",
		},
		{
			name:    "only client ID",
			cmdline: `talos.config.oauth.client_id=device_client_id`,
			expected: &oauth2.Config{
				ClientID:      "device_client_id",
				TokenURL:      "https://example.com/token",
				DeviceAuthURL: "https://example.com/device/code",
			},
		},
		{
			name:    "client ID and custom URLs",
			cmdline: `talos.config.oauth.client_id=device_client_id talos.config.oauth.token_url=https://google.com/token talos.config.oauth.device_auth_url=https://google.com/device/code`,
			expected: &oauth2.Config{
				ClientID:      "device_client_id",
				TokenURL:      "https://google.com/token",
				DeviceAuthURL: "https://google.com/device/code",
			},
		},
		{
			name: "complete config",
			cmdline: `talos.config.oauth.client_id=device_client_id talos.config.oauth.client_secret=device_secret ` +
				`talos.config.oauth.token_url=https://google.com/token talos.config.oauth.device_auth_url=https://google.com/device/code ` +
				`talos.config.oauth.scope=foo talos.config.oauth.scope=bar talos.config.oauth.audience=world ` +
				`talos.config.oauth.extra_variable=uuid talos.config.oauth.extra_variable=mac`,
			expected: &oauth2.Config{
				ClientID:       "device_client_id",
				ClientSecret:   "device_secret",
				Audience:       "world",
				Scopes:         []string{"foo", "bar"},
				ExtraVariables: []string{"uuid", "mac"},
				TokenURL:       "https://google.com/token",
				DeviceAuthURL:  "https://google.com/device/code",
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg, err := oauth2.NewConfig(procfs.NewCmdline(test.cmdline), "https://example.com/my/config")
			if test.expected == nil {
				require.Error(t, err)
				assert.True(t, os.IsNotExist(err))

				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.expected, cfg)
		})
	}
}

func TestDeviceAuthFlow(t *testing.T) {
	t.Parallel()

	cfg := &oauth2.Config{
		ClientID: "device_client_id",
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close() //nolint:errcheck

		t.Logf("received request: %s %s", r.Method, r.RequestURI)

		switch r.Method + r.RequestURI {
		case "POST/device/code":
			w.Header().Add("Content-Type", "application/json")
			w.Write([]byte(`{"device_code":"abcd", "user_code":"1234", "verification_uri":"https://example.com/verify","verification_uri_complete":"https://example.com/verify/1234","interval":1,"expires_in":36000}`)) //nolint:errcheck,lll
		case "POST/token":
			w.Header().Add("Content-Type", "application/json")
			w.Write([]byte(`{"access_token":"abcd","token_type":"bearer","expires_in":3600,"refresh_token":"efgh","id_token":"ijkl"}`)) //nolint:errcheck
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(ts.Close)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	t.Cleanup(cancel)

	cfg.DeviceAuthURL = ts.URL + "/device/code"
	cfg.TokenURL = ts.URL + "/token"

	require.NoError(t, cfg.DeviceAuthFlow(ctx, nil))
	assert.Equal(t, map[string]string{"Authorization": "Bearer abcd"}, cfg.ExtraHeaders())
}
