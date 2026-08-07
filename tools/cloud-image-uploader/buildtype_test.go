// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import "testing"

func TestDeriveBuildType(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name       string
		tag        string
		namePrefix string
		expected   string
	}{
		{
			name:     "stable release",
			tag:      "v1.13.4",
			expected: BuildTypeRelease,
		},
		{
			name:     "alpha release",
			tag:      "v1.14.0-alpha.1",
			expected: BuildTypeRelease,
		},
		{
			name:     "rc release",
			tag:      "v1.14.0-rc.0",
			expected: BuildTypeRelease,
		},
		{
			name:     "nightly off a pre-release tag",
			tag:      "v1.14.0-alpha.1-101-g11a7fbe4c",
			expected: BuildTypeNightly,
		},
		{
			name:     "nightly off a stable tag",
			tag:      "v1.13.4-7-gdeadbeef",
			expected: BuildTypeNightly,
		},
		{
			name:     "unrecognizable version",
			tag:      "some-branch-build",
			expected: BuildTypeNightly,
		},
		{
			name:       "e2e run keeps its build type despite a release tag",
			tag:        "v1.13.4",
			namePrefix: "talos-e2e",
			expected:   BuildTypeE2E,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if actual := DeriveBuildType(test.tag, test.namePrefix); actual != test.expected {
				t.Errorf("DeriveBuildType(%q, %q) = %q, expected %q", test.tag, test.namePrefix, actual, test.expected)
			}
		})
	}
}
