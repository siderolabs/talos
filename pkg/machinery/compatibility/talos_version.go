// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package compatibility

import (
	"fmt"

	"github.com/blang/semver/v4"
	"github.com/siderolabs/gen/pair/ordered"

	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/compatibility/talos110"
	"github.com/siderolabs/talos/pkg/machinery/compatibility/talos111"
	"github.com/siderolabs/talos/pkg/machinery/compatibility/talos12"
	"github.com/siderolabs/talos/pkg/machinery/compatibility/talos13"
	"github.com/siderolabs/talos/pkg/machinery/compatibility/talos14"
	"github.com/siderolabs/talos/pkg/machinery/compatibility/talos15"
	"github.com/siderolabs/talos/pkg/machinery/compatibility/talos16"
	"github.com/siderolabs/talos/pkg/machinery/compatibility/talos17"
	"github.com/siderolabs/talos/pkg/machinery/compatibility/talos18"
	"github.com/siderolabs/talos/pkg/machinery/compatibility/talos19"
)

// TalosVersion embeds Talos version.
type TalosVersion struct {
	version    semver.Version
	majorMinor [2]uint64
}

// ParseTalosVersion parses Talos version.
func ParseTalosVersion(v *machine.VersionInfo) (*TalosVersion, error) {
	parsed, err := semver.ParseTolerant(v.Tag)
	if err != nil {
		return nil, err
	}

	return &TalosVersion{
		version:    parsed,
		majorMinor: [2]uint64{parsed.Major, parsed.Minor},
	}, nil
}

func (v *TalosVersion) String() string {
	return v.version.String()
}

// DisablePredictableNetworkInterfaces returns true if predictable network interfaces should be disabled on upgrade.
func (v *TalosVersion) DisablePredictableNetworkInterfaces() bool {
	if v.majorMinor[0] <= talos14.MajorMinor[0] && v.majorMinor[1] <= talos14.MajorMinor[1] {
		return true
	}

	return false
}

// PrecreateStatePartition returns true if running an 1.8+ installer from a version before <=1.7.x.
//
// Host Talos needs STATE partition to save the machine configuration.
func (v *TalosVersion) PrecreateStatePartition() bool {
	if v.majorMinor[0] <= talos17.MajorMinor[0] && v.majorMinor[1] <= talos17.MajorMinor[1] {
		return true
	}

	return false
}

// UpgradeableFrom checks if the current version of Talos can be used as an upgrade for the given host version.
//
//nolint:gocyclo
func (v *TalosVersion) UpgradeableFrom(host *TalosVersion) error {
	var (
		minHostUpgradeVersion, maxHostDowngradeVersion semver.Version
		deniedHostUpgradeVersions                      []semver.Version
	)

	switch v.majorMinor {
	case talos12.MajorMinor: // upgrades to 1.2.x
		minHostUpgradeVersion, maxHostDowngradeVersion = talos12.MinimumHostUpgradeVersion, talos12.MaximumHostDowngradeVersion
		deniedHostUpgradeVersions = talos12.DeniedHostUpgradeVersions
	case talos13.MajorMinor: // upgrades to 1.3.x
		minHostUpgradeVersion, maxHostDowngradeVersion = talos13.MinimumHostUpgradeVersion, talos13.MaximumHostDowngradeVersion
		deniedHostUpgradeVersions = talos13.DeniedHostUpgradeVersions
	case talos14.MajorMinor: // upgrades to 1.4.x
		minHostUpgradeVersion, maxHostDowngradeVersion = talos14.MinimumHostUpgradeVersion, talos14.MaximumHostDowngradeVersion
		deniedHostUpgradeVersions = talos14.DeniedHostUpgradeVersions
	case talos15.MajorMinor: // upgrades to 1.5.x
		minHostUpgradeVersion, maxHostDowngradeVersion = talos15.MinimumHostUpgradeVersion, talos15.MaximumHostDowngradeVersion
		deniedHostUpgradeVersions = talos15.DeniedHostUpgradeVersions
	case talos16.MajorMinor: // upgrades to 1.6.x
		minHostUpgradeVersion, maxHostDowngradeVersion = talos16.MinimumHostUpgradeVersion, talos16.MaximumHostDowngradeVersion
		deniedHostUpgradeVersions = talos16.DeniedHostUpgradeVersions
	case talos17.MajorMinor: // upgrades to 1.7.x
		minHostUpgradeVersion, maxHostDowngradeVersion = talos17.MinimumHostUpgradeVersion, talos17.MaximumHostDowngradeVersion
		deniedHostUpgradeVersions = talos17.DeniedHostUpgradeVersions
	case talos18.MajorMinor: // upgrades to 1.8.x
		minHostUpgradeVersion, maxHostDowngradeVersion = talos18.MinimumHostUpgradeVersion, talos18.MaximumHostDowngradeVersion
		deniedHostUpgradeVersions = talos18.DeniedHostUpgradeVersions
	case talos19.MajorMinor: // upgrades to 1.9.x
		minHostUpgradeVersion, maxHostDowngradeVersion = talos19.MinimumHostUpgradeVersion, talos19.MaximumHostDowngradeVersion
		deniedHostUpgradeVersions = talos19.DeniedHostUpgradeVersions
	case talos110.MajorMinor: // upgrades to 1.10.x
		minHostUpgradeVersion, maxHostDowngradeVersion = talos110.MinimumHostUpgradeVersion, talos110.MaximumHostDowngradeVersion
		deniedHostUpgradeVersions = talos110.DeniedHostUpgradeVersions
	case talos111.MajorMinor: // upgrades to 1.11.x
		minHostUpgradeVersion, maxHostDowngradeVersion = talos111.MinimumHostUpgradeVersion, talos111.MaximumHostDowngradeVersion
		deniedHostUpgradeVersions = talos111.DeniedHostUpgradeVersions
	default:
		return fmt.Errorf("upgrades to version %s are not supported", v.version.String())
	}

	hostCore := ordered.MakeTriple(host.majorMinor[0], host.majorMinor[1], host.version.Patch)

	minHostUpgradeVersionCore := ordered.MakeTriple(minHostUpgradeVersion.Major, minHostUpgradeVersion.Minor, minHostUpgradeVersion.Patch)

	if hostCore.LessThan(minHostUpgradeVersionCore) {
		return fmt.Errorf("host version %s is too old to upgrade to Talos %s", host.version.String(), v.version.String())
	}

	maxHostDowngradeVersionCore := ordered.MakeTriple(maxHostDowngradeVersion.Major, maxHostDowngradeVersion.Minor, maxHostDowngradeVersion.Patch)

	if hostCore.Compare(maxHostDowngradeVersionCore) >= 0 {
		return fmt.Errorf("host version %s is too new to downgrade to Talos %s", host.version.String(), v.version.String())
	}

	for _, denied := range deniedHostUpgradeVersions {
		if host.version.EQ(denied) {
			return fmt.Errorf("host version %s is denied for upgrade to Talos %s", host.version.String(), v.version.String())
		}
	}

	return nil
}
