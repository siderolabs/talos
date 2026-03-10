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
		nameVar        string
		wantHostname   string
		wantDomainname string
	}{
		{
			name:           "clean hostname passes through unchanged",
			nameVar:        "myhost",
			wantHostname:   "myhost",
			wantDomainname: "",
		},
		{
			name:           "FQDN is split on first dot",
			nameVar:        "myhost.example.com",
			wantHostname:   "myhost",
			wantDomainname: "example.com",
		},
		{
			name:           "invalid chars replaced with hyphen",
			nameVar:        "my_host",
			wantHostname:   "my-host",
			wantDomainname: "",
		},
		{
			name:           "leading and trailing hyphens stripped",
			nameVar:        "-myhost-",
			wantHostname:   "myhost",
			wantDomainname: "",
		},
		{
			name:           "per-label hyphen trimming",
			nameVar:        "my-.host",
			wantHostname:   "my",
			wantDomainname: "host",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := minimalContext("NAME = \"" + tc.nameVar + "\"")

			networkConfig, err := o.ParseMetadata(st, ctx)
			require.NoError(t, err)
			require.Len(t, networkConfig.Hostnames, 1)

			assert.Equal(t, tc.wantHostname, networkConfig.Hostnames[0].Hostname)
			assert.Equal(t, tc.wantDomainname, networkConfig.Hostnames[0].Domainname)
		})
	}

	t.Run("empty string produces no hostname entry", func(t *testing.T) {
		t.Parallel()

		ctx := minimalContext("NAME = \"\"")

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
		wantHostname   string
		wantDomainname string
	}{
		{
			name:           "HOSTNAME takes priority",
			vars:           "HOSTNAME = \"fromhostname\"\nSET_HOSTNAME = \"fromsethostname\"\nNAME = \"fromname\"",
			wantHostname:   "fromhostname",
			wantDomainname: "",
		},
		{
			name:           "falls back to SET_HOSTNAME when HOSTNAME is empty",
			vars:           "SET_HOSTNAME = \"fromsethostname\"\nNAME = \"fromname\"",
			wantHostname:   "fromsethostname",
			wantDomainname: "",
		},
		{
			name:           "falls back to NAME when both HOSTNAME and SET_HOSTNAME are empty",
			vars:           "NAME = \"fromname\"",
			wantHostname:   "fromname",
			wantDomainname: "",
		},
		{
			name:           "DNS_HOSTNAME=YES is not used as Domainname",
			vars:           "NAME = \"myhost\"\nDNS_HOSTNAME = \"YES\"",
			wantHostname:   "myhost",
			wantDomainname: "",
		},
		{
			name:           "FQDN in NAME is split into Hostname and Domainname",
			vars:           "NAME = \"myhost.example.com\"",
			wantHostname:   "myhost",
			wantDomainname: "example.com",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := minimalContext(tc.vars)

			networkConfig, err := o.ParseMetadata(st, ctx)
			require.NoError(t, err)
			require.Len(t, networkConfig.Hostnames, 1)

			assert.Equal(t, tc.wantHostname, networkConfig.Hostnames[0].Hostname)
			assert.Equal(t, tc.wantDomainname, networkConfig.Hostnames[0].Domainname)
		})
	}
}
