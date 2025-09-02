// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package profile_test

import (
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/blang/semver/v4"
	"github.com/siderolabs/gen/maps"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/imager/profile"
	"github.com/siderolabs/talos/pkg/machinery/version"
)

func TestFillDefaults(t *testing.T) {
	t.Parallel()

	// we can ignore profile that are legacy Boards
	defaultProfiles := maps.Filter(profile.Default, func(k string, v profile.Profile) bool {
		return v.Board == ""
	})

	arches := []string{"amd64", "arm64"}
	versions := []string{"1.9.0", "1.10.0", "1.11.0", "1.12.0"}

	lastVersion := semver.MustParse(versions[len(versions)-1])

	currentVersion, err := semver.ParseTolerant(version.Tag)
	require.NoError(t, err)

	currentVersion.Patch = 0
	currentVersion.Pre = nil

	require.True(t, lastVersion.EQ(currentVersion), "last version %s should be equal to current version %s", lastVersion, currentVersion)

	profiles := maps.Keys(defaultProfiles)

	sort.Strings(profiles)

	// flip this to true to generate missing testdata files
	const recordMissing = false

	for _, profile := range profiles {
		t.Run(profile, func(t *testing.T) {
			t.Parallel()

			var secureBoot bool

			if strings.HasPrefix(profile, "secureboot") {
				secureBoot = true
			}

			for _, arch := range arches {
				t.Run(arch, func(t *testing.T) {
					t.Parallel()

					for _, version := range versions {
						t.Run(version, func(t *testing.T) {
							t.Parallel()

							p := defaultProfiles[profile].DeepCopy()

							p.Arch = arch
							p.Version = version

							p.Input.FillDefaults(arch, version, secureBoot)
							p.Output.FillDefaults(arch, version, secureBoot)

							require.NoError(t, p.Validate())

							var profileData strings.Builder

							require.NoError(t, p.Dump(&profileData))

							expectedData, err := os.ReadFile("testdata/" + profile + "-" + arch + "-" + version + ".yaml")
							if os.IsNotExist(err) && recordMissing {
								require.NoError(t, os.WriteFile("testdata/"+profile+"-"+arch+"-"+version+".yaml", []byte(profileData.String()), 0o644))
							} else {
								require.NoError(t, err)

								require.Equal(t, string(expectedData), profileData.String(), "profile: %s, platform: %s, version: %s", profile, arch, version)
							}
						})
					}
				})
			}
		})
	}
}
