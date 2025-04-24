// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package compatibility_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/compatibility"
)

type talosVersionTest struct {
	host          string
	target        string
	expectedError string
}

func runTalosVersionTest(t *testing.T, tt talosVersionTest) {
	t.Run(tt.host+" -> "+tt.target, func(t *testing.T) {
		host, err := compatibility.ParseTalosVersion(&machine.VersionInfo{
			Tag: tt.host,
		})
		require.NoError(t, err)

		target, err := compatibility.ParseTalosVersion(&machine.VersionInfo{
			Tag: tt.target,
		})
		require.NoError(t, err)

		err = target.UpgradeableFrom(host)
		if tt.expectedError != "" {
			require.EqualError(t, err, tt.expectedError)
		} else {
			require.NoError(t, err)
		}
	})
}

func TestTalosUpgradeCompatibility13(t *testing.T) {
	for _, tt := range []talosVersionTest{
		{
			host:   "1.2.0",
			target: "1.3.0",
		},
		{
			host:   "1.0.0-alpha.0",
			target: "1.3.0",
		},
		{
			host:   "1.2.0-alpha.0",
			target: "1.3.0-alpha.0",
		},
		{
			host:   "1.3.0",
			target: "1.3.1",
		},
		{
			host:   "1.3.0-beta.0",
			target: "1.3.0",
		},
		{
			host:   "1.4.5",
			target: "1.3.3",
		},
		{
			host:          "0.14.3",
			target:        "1.3.0",
			expectedError: `host version 0.14.3 is too old to upgrade to Talos 1.3.0`,
		},
		{
			host:          "1.5.0-alpha.0",
			target:        "1.3.0",
			expectedError: `host version 1.5.0-alpha.0 is too new to downgrade to Talos 1.3.0`,
		},
	} {
		runTalosVersionTest(t, tt)
	}
}

func TestTalosUpgradeCompatibility14(t *testing.T) {
	for _, tt := range []talosVersionTest{
		{
			host:   "1.3.0",
			target: "1.4.0",
		},
		{
			host:   "1.0.0-alpha.0",
			target: "1.4.0",
		},
		{
			host:   "1.2.0-alpha.0",
			target: "1.4.0-alpha.0",
		},
		{
			host:   "1.4.0",
			target: "1.4.1",
		},
		{
			host:   "1.4.0-beta.0",
			target: "1.4.0",
		},
		{
			host:   "1.5.5",
			target: "1.4.3",
		},
		{
			host:          "0.14.3",
			target:        "1.4.0",
			expectedError: `host version 0.14.3 is too old to upgrade to Talos 1.4.0`,
		},
		{
			host:          "1.6.0-alpha.0",
			target:        "1.4.0",
			expectedError: `host version 1.6.0-alpha.0 is too new to downgrade to Talos 1.4.0`,
		},
	} {
		runTalosVersionTest(t, tt)
	}
}

func TestTalosUpgradeCompatibility15(t *testing.T) {
	for _, tt := range []talosVersionTest{
		{
			host:   "1.3.0",
			target: "1.5.0",
		},
		{
			host:   "1.2.0-alpha.0",
			target: "1.5.0",
		},
		{
			host:   "1.2.0",
			target: "1.5.0-alpha.0",
		},
		{
			host:   "1.5.0",
			target: "1.5.1",
		},
		{
			host:   "1.5.0-beta.0",
			target: "1.5.0",
		},
		{
			host:   "1.6.5",
			target: "1.5.3",
		},
		{
			host:          "1.1.0",
			target:        "1.5.0",
			expectedError: `host version 1.1.0 is too old to upgrade to Talos 1.5.0`,
		},
		{
			host:          "1.7.0-alpha.0",
			target:        "1.5.0",
			expectedError: `host version 1.7.0-alpha.0 is too new to downgrade to Talos 1.5.0`,
		},
	} {
		runTalosVersionTest(t, tt)
	}
}

func TestTalosUpgradeCompatibility16(t *testing.T) {
	for _, tt := range []talosVersionTest{
		{
			host:   "1.4.0",
			target: "1.6.0",
		},
		{
			host:   "1.3.0-alpha.0",
			target: "1.6.0",
		},
		{
			host:   "1.3.0",
			target: "1.6.0-alpha.0",
		},
		{
			host:   "1.6.0",
			target: "1.6.1",
		},
		{
			host:   "1.6.0-beta.0",
			target: "1.6.0",
		},
		{
			host:   "1.7.5",
			target: "1.6.3",
		},
		{
			host:          "1.2.0",
			target:        "1.6.0",
			expectedError: `host version 1.2.0 is too old to upgrade to Talos 1.6.0`,
		},
		{
			host:          "1.8.0-alpha.0",
			target:        "1.6.0",
			expectedError: `host version 1.8.0-alpha.0 is too new to downgrade to Talos 1.6.0`,
		},
	} {
		runTalosVersionTest(t, tt)
	}
}

func TestTalosUpgradeCompatibility17(t *testing.T) {
	for _, tt := range []talosVersionTest{
		{
			host:   "1.5.0",
			target: "1.7.0",
		},
		{
			host:   "1.4.0-alpha.0",
			target: "1.7.0",
		},
		{
			host:   "1.4.0",
			target: "1.7.0-alpha.0",
		},
		{
			host:   "1.6.0",
			target: "1.7.1",
		},
		{
			host:   "1.6.0-beta.0",
			target: "1.7.0",
		},
		{
			host:   "1.8.5",
			target: "1.7.3",
		},
		{
			host:          "1.3.0",
			target:        "1.7.0",
			expectedError: `host version 1.3.0 is too old to upgrade to Talos 1.7.0`,
		},
		{
			host:          "1.9.0-alpha.0",
			target:        "1.7.0",
			expectedError: `host version 1.9.0-alpha.0 is too new to downgrade to Talos 1.7.0`,
		},
	} {
		runTalosVersionTest(t, tt)
	}
}

func TestTalosUpgradeCompatibility18(t *testing.T) {
	for _, tt := range []talosVersionTest{
		{
			host:   "1.6.0",
			target: "1.8.0",
		},
		{
			host:   "1.5.0-alpha.0",
			target: "1.8.0",
		},
		{
			host:   "1.5.0",
			target: "1.8.0-alpha.0",
		},
		{
			host:   "1.7.0",
			target: "1.8.1",
		},
		{
			host:   "1.7.0-beta.0",
			target: "1.8.0",
		},
		{
			host:   "1.9.5",
			target: "1.8.3",
		},
		{
			host:          "1.4.0",
			target:        "1.8.0",
			expectedError: `host version 1.4.0 is too old to upgrade to Talos 1.8.0`,
		},
		{
			host:          "1.10.0-alpha.0",
			target:        "1.8.0",
			expectedError: `host version 1.10.0-alpha.0 is too new to downgrade to Talos 1.8.0`,
		},
	} {
		runTalosVersionTest(t, tt)
	}
}

func TestTalosUpgradeCompatibility19(t *testing.T) {
	for _, tt := range []talosVersionTest{
		{
			host:   "1.8.0",
			target: "1.9.0",
		},
		{
			host:   "1.8.0-alpha.0",
			target: "1.9.0",
		},
		{
			host:   "1.8.0",
			target: "1.9.0-alpha.0",
		},
		{
			host:   "1.8.3",
			target: "1.9.1",
		},
		{
			host:   "1.9.0-beta.0",
			target: "1.9.0",
		},
		{
			host:   "1.9.5",
			target: "1.9.3",
		},
		{
			host:          "1.7.0",
			target:        "1.9.0",
			expectedError: `host version 1.7.0 is too old to upgrade to Talos 1.9.0`,
		},
		{
			host:          "1.11.0-alpha.0",
			target:        "1.9.0",
			expectedError: `host version 1.11.0-alpha.0 is too new to downgrade to Talos 1.9.0`,
		},
	} {
		runTalosVersionTest(t, tt)
	}
}

func TestTalosUpgradeCompatibility110(t *testing.T) {
	for _, tt := range []talosVersionTest{
		{
			host:   "1.8.0",
			target: "1.10.0",
		},
		{
			host:   "1.9.0-alpha.0",
			target: "1.10.0",
		},
		{
			host:   "1.8.0",
			target: "1.10.0-alpha.0",
		},
		{
			host:   "1.9.3",
			target: "1.10.1",
		},
		{
			host:   "1.10.0-beta.0",
			target: "1.10.0",
		},
		{
			host:   "1.10.5",
			target: "1.10.3",
		},
		{
			host:          "1.7.0",
			target:        "1.10.0",
			expectedError: `host version 1.7.0 is too old to upgrade to Talos 1.10.0`,
		},
		{
			host:          "1.12.0-alpha.0",
			target:        "1.10.0",
			expectedError: `host version 1.12.0-alpha.0 is too new to downgrade to Talos 1.10.0`,
		},
	} {
		runTalosVersionTest(t, tt)
	}
}

func TestTalosUpgradeCompatibility111(t *testing.T) {
	for _, tt := range []talosVersionTest{
		{
			host:   "1.9.0",
			target: "1.11.0",
		},
		{
			host:   "1.10.0-alpha.0",
			target: "1.11.0",
		},
		{
			host:   "1.9.0",
			target: "1.11.0-alpha.0",
		},
		{
			host:   "1.10.3",
			target: "1.11.1",
		},
		{
			host:   "1.11.0-beta.0",
			target: "1.11.0",
		},
		{
			host:   "1.11.5",
			target: "1.11.3",
		},
		{
			host:          "1.8.0",
			target:        "1.11.0",
			expectedError: `host version 1.8.0 is too old to upgrade to Talos 1.11.0`,
		},
		{
			host:          "1.13.0-alpha.0",
			target:        "1.11.0",
			expectedError: `host version 1.13.0-alpha.0 is too new to downgrade to Talos 1.11.0`,
		},
	} {
		runTalosVersionTest(t, tt)
	}
}

func TestTalosUpgradeCompatibilityUnsupported(t *testing.T) {
	for _, tt := range []talosVersionTest{
		{
			host:          "1.3.0",
			target:        "1.12.0-alpha.0",
			expectedError: `upgrades to version 1.12.0-alpha.0 are not supported`,
		},
		{
			host:          "1.4.0",
			target:        "1.13.0-alpha.0",
			expectedError: `upgrades to version 1.13.0-alpha.0 are not supported`,
		},
	} {
		runTalosVersionTest(t, tt)
	}
}

func TestDisablePredictableNetworkInterfaces(t *testing.T) {
	for _, tt := range []struct {
		host     string
		expected bool
	}{
		{
			host:     "1.3.0",
			expected: true,
		},
		{
			host:     "1.4.0",
			expected: true,
		},
		{
			host:     "1.5.0",
			expected: false,
		},
		{
			host:     "1.6.0",
			expected: false,
		},
		{
			host:     "1.7.0",
			expected: false,
		},
	} {
		t.Run(tt.host, func(t *testing.T) {
			host, err := compatibility.ParseTalosVersion(&machine.VersionInfo{
				Tag: tt.host,
			})
			require.NoError(t, err)

			require.Equal(t, tt.expected, host.DisablePredictableNetworkInterfaces())
		})
	}
}
