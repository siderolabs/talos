// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package container_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/types/container"
)

func TestValidateCapabilityName(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name        string
		field       string
		capability  string
		expectedErr string
	}{
		{
			name:       "valid capability",
			field:      "capabilities.add",
			capability: "NET_ADMIN",
		},
		{
			name:        "empty capability",
			field:       "capabilities.add",
			capability:  "",
			expectedErr: "capabilities.add: capability name must not be empty",
		},
		{
			name:        "CAP_ prefix",
			field:       "capabilities.add",
			capability:  "CAP_NET_ADMIN",
			expectedErr: `capabilities.add: capability "CAP_NET_ADMIN" must be given without the CAP_ prefix`,
		},
		{
			name:        "lowercase",
			field:       "capabilities.drop",
			capability:  "net_admin",
			expectedErr: `capabilities.drop: capability "net_admin" must be upper case`,
		},
		{
			name:        "mixed case",
			field:       "capabilities.add",
			capability:  "Net_Admin",
			expectedErr: "must be upper case",
		},
		{
			name:        "invalid character digit",
			field:       "capabilities.add",
			capability:  "NET_ADMIN2",
			expectedErr: `capability "NET_ADMIN2" may only contain upper case letters and underscores`,
		},
		{
			name:        "invalid character hyphen",
			field:       "capabilities.add",
			capability:  "NET-ADMIN",
			expectedErr: "may only contain upper case letters and underscores",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := container.ValidateCapabilityName(test.field, test.capability)

			if test.expectedErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.expectedErr)
			}
		})
	}
}

func TestContainerSecurityReadOnlyRootFS(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		security string
		expected bool
	}{
		{
			name:     "restricted by default",
			expected: true,
		},
		{
			name: "restricted explicit",
			security: `security:
  profile: restricted
`,
			expected: true,
		},
		{
			name: "privileged default",
			security: `security:
  profile: privileged
`,
			expected: false,
		},
		{
			name: "restricted with writable rootfs",
			security: `security:
  readOnlyRootFS: false
`,
			expected: false,
		},
		{
			name: "privileged with read-only rootfs",
			security: `security:
  profile: privileged
  readOnlyRootFS: true
`,
			expected: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := load(t, `apiVersion: v1alpha1
kind: ContainerConfig
name: web
image: docker.io/library/nginx:1.27
`+test.security)

			_, err := cfg.Validate(validationMode{})
			require.NoError(t, err)

			assert.Equal(t, test.expected, cfg.Security().ReadOnlyRootFS())
		})
	}
}
