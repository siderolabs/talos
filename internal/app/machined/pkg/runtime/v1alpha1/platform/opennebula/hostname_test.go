// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package opennebula_test

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/opennebula"
)

// minimalContext returns the minimum context bytes needed to exercise hostname
// parsing without triggering ETH* processing.
func minimalContext(vars string) []byte {
	return []byte("ETH0_MAC = \"02:00:c0:a8:01:5c\"\nETH0_IP = \"10.0.0.1\"\nETH0_MASK = \"255.255.255.0\"\n" + vars)
}

func TestSanitizeHostname(t *testing.T) {
	t.Parallel()

	o := &opennebula.OpenNebula{}
	st := state.WrapCore(namespaced.NewState(inmem.Build))

	for _, tc := range []struct {
		name           string
		setHostname    string
		wantHostname   string
		wantDomainname string
	}{
		{
			name:           "clean hostname passes through unchanged",
			setHostname:    "myhost",
			wantHostname:   "myhost",
			wantDomainname: "",
		},
		{
			name:           "FQDN is split on first dot",
			setHostname:    "myhost.example.com",
			wantHostname:   "myhost",
			wantDomainname: "example.com",
		},
		{
			name:           "invalid chars replaced with hyphen",
			setHostname:    "my_host",
			wantHostname:   "my-host",
			wantDomainname: "",
		},
		{
			name:           "leading and trailing hyphens stripped",
			setHostname:    "-myhost-",
			wantHostname:   "myhost",
			wantDomainname: "",
		},
		{
			name:           "per-label hyphen trimming",
			setHostname:    "my-.host",
			wantHostname:   "my",
			wantDomainname: "host",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := minimalContext("SET_HOSTNAME = \"" + tc.setHostname + "\"")

			networkConfig, err := o.ParseMetadata(st, ctx)
			require.NoError(t, err)
			require.Len(t, networkConfig.Hostnames, 1)

			assert.Equal(t, tc.wantHostname, networkConfig.Hostnames[0].Hostname)
			assert.Equal(t, tc.wantDomainname, networkConfig.Hostnames[0].Domainname)
		})
	}

	t.Run("empty SET_HOSTNAME produces no hostname entry", func(t *testing.T) {
		t.Parallel()

		ctx := minimalContext("SET_HOSTNAME = \"\"")

		networkConfig, err := o.ParseMetadata(st, ctx)
		require.NoError(t, err)
		assert.Empty(t, networkConfig.Hostnames)
	})
}

func TestParseMetadataHostname(t *testing.T) {
	t.Parallel()

	o := &opennebula.OpenNebula{}
	st := state.WrapCore(namespaced.NewState(inmem.Build))

	for _, tc := range []struct {
		name           string
		vars           string
		wantHostnames  int
		wantHostname   string
		wantDomainname string
	}{
		{
			name:          "SET_HOSTNAME is used as hostname",
			vars:          "SET_HOSTNAME = \"myhost\"",
			wantHostnames: 1,
			wantHostname:  "myhost",
		},
		{
			name:           "FQDN in SET_HOSTNAME is split into Hostname and Domainname",
			vars:           "SET_HOSTNAME = \"myhost.example.com\"",
			wantHostnames:  1,
			wantHostname:   "myhost",
			wantDomainname: "example.com",
		},
		{
			name:          "HOSTNAME variable is ignored",
			vars:          "HOSTNAME = \"fromhostname\"",
			wantHostnames: 0,
		},
		{
			name:          "NAME variable is ignored",
			vars:          "NAME = \"fromname\"",
			wantHostnames: 0,
		},
		{
			name:          "SET_HOSTNAME takes precedence over HOSTNAME and NAME",
			vars:          "SET_HOSTNAME = \"correct\"\nHOSTNAME = \"wrong\"\nNAME = \"alsowrong\"",
			wantHostnames: 1,
			wantHostname:  "correct",
		},
		{
			name:          "DNS_HOSTNAME=YES is not used as a hostname value",
			vars:          "DNS_HOSTNAME = \"YES\"",
			wantHostnames: 0,
		},
		{
			name:          "absent SET_HOSTNAME produces no hostname entry",
			vars:          "",
			wantHostnames: 0,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := minimalContext(tc.vars)

			networkConfig, err := o.ParseMetadata(st, ctx)
			require.NoError(t, err)
			require.Len(t, networkConfig.Hostnames, tc.wantHostnames)

			if tc.wantHostnames > 0 {
				assert.Equal(t, tc.wantHostname, networkConfig.Hostnames[0].Hostname)
				assert.Equal(t, tc.wantDomainname, networkConfig.Hostnames[0].Domainname)
			}
		})
	}
}
