// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/rest"

	"github.com/siderolabs/talos/pkg/cluster"
)

func TestApplyKubernetesEndpointOverride(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		endpoint string

		expectedHost  string
		expectedError string
	}{
		{endpoint: "", expectedHost: "https://original:6443"},
		// naked ips
		{endpoint: "10.5.0.2", expectedHost: "https://10.5.0.2:6443"},
		{endpoint: "2001:db8::1", expectedHost: "https://[2001:db8::1]:6443"},
		{endpoint: "fe80::1%eth0", expectedHost: "https://[fe80::1%25eth0]:6443"},
		{endpoint: "10.5.0.2:7443", expectedHost: "https://10.5.0.2:7443"},
		// default port added
		{endpoint: "[2001:db8::1]", expectedHost: "https://[2001:db8::1]:6443"},
		{endpoint: "https://myhost", expectedHost: "https://myhost:6443"},
		{endpoint: "myhost/", expectedHost: "https://myhost:6443"},
		// valid
		{endpoint: "https://myhost1:6443", expectedHost: "https://myhost1:6443"},
		{endpoint: "https://myhost2:6443/", expectedHost: "https://myhost2:6443"},
		{endpoint: "myhost:7443", expectedHost: "https://myhost:7443"},
		{endpoint: "https://[2001:db8::1]:7443/", expectedHost: "https://[2001:db8::1]:7443"},
		{endpoint: "https://[fe80::1%25eth0]:7443/", expectedHost: "https://[fe80::1%25eth0]:7443"},
		// invalid
		{endpoint: "http://myhost:6443", expectedError: "only https"},
		{endpoint: "http://myhost", expectedError: "only https"},
		{endpoint: "my host", expectedError: "invalid character"},
		{endpoint: "https://", expectedError: "host is empty"},
		{endpoint: "/", expectedError: "host is empty"},
		{endpoint: ":6443", expectedError: "host is empty"},
		{endpoint: "https://myhost:abc", expectedError: "invalid port"},
		{endpoint: "https://myhost:0", expectedError: "invalid port"},
		{endpoint: "https://myhost:65536", expectedError: "invalid port"},
		{endpoint: "https://myhost:6443//", expectedError: "not allowed"},
		{endpoint: "https://myhost:6443/api", expectedError: "not allowed"},
		{endpoint: "https://user@myhost:6443/", expectedError: "not allowed"},
		{endpoint: "https://myhost:6443/?test=123", expectedError: "not allowed"},
		{endpoint: "https://myhost:6443/#frag123", expectedError: "not allowed"},
	} {
		t.Run(tt.endpoint, func(t *testing.T) {
			t.Parallel()

			config := &rest.Config{Host: "https://original:6443"}

			err := cluster.ApplyKubernetesEndpointOverride(config, tt.endpoint)

			if tt.expectedError != "" {
				require.ErrorContains(t, err, tt.expectedError)
				assert.Equal(t, "https://original:6443", config.Host)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedHost, config.Host)

			if tt.endpoint != "" {
				assert.NoError(t, cluster.ValidateKubernetesEndpoint(tt.endpoint))
			}
		})
	}
}
