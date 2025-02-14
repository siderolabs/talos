// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package profile_test

import (
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/siderolabs/gen/maps"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/imager/profile"
)

func TestFillDefaults(t *testing.T) {
	// we can ignore profile that are legacy Boards
	defaultProfiles := maps.Filter(profile.Default, func(k string, v profile.Profile) bool {
		return v.Board == ""
	})

	arches := []string{"amd64", "arm64"}
	versions := []string{"1.9.0", "1.10.0"}

	profiles := maps.Keys(defaultProfiles)

	sort.Strings(profiles)

	for _, profile := range profiles {
		var secureBoot bool

		if strings.HasPrefix(profile, "secureboot") {
			secureBoot = true
		}

		for _, arch := range arches {
			for _, version := range versions {
				p := defaultProfiles[profile].DeepCopy()

				p.Arch = arch
				p.Version = version

				p.Input.FillDefaults(arch, version, secureBoot)
				p.Output.FillDefaults(arch, version, secureBoot)

				require.NoError(t, p.Validate())

				var profileData strings.Builder

				require.NoError(t, p.Dump(&profileData))

				expectedData, err := os.ReadFile("testdata/" + profile + "-" + arch + "-" + version + ".yaml")
				require.NoError(t, err)

				require.Equal(t, string(expectedData), profileData.String(), "profile: %s, platform: %s, version: %s", profile, arch, version)
			}
		}
	}
}
