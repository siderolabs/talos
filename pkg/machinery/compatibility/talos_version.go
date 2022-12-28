// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package compatibility

import (
	"fmt"

	"github.com/hashicorp/go-version"

	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/compatibility/talos13"
	"github.com/siderolabs/talos/pkg/machinery/compatibility/talos14"
)

// TalosVersion embeds Talos version.
type TalosVersion struct {
	version    version.Version
	majorMinor [2]int
}

// ParseTalosVersion parses Talos version.
func ParseTalosVersion(v *machine.VersionInfo) (*TalosVersion, error) {
	parsed, err := version.NewVersion(v.Tag)
	if err != nil {
		return nil, err
	}

	return &TalosVersion{
		version:    *parsed,
		majorMinor: [2]int{parsed.Segments()[0], parsed.Segments()[1]},
	}, nil
}

func (v *TalosVersion) String() string {
	return v.version.String()
}

// UpgradeableFrom checks if the current version of Talos can be used as an upgrade for the given host version.
func (v *TalosVersion) UpgradeableFrom(host *TalosVersion) error {
	var (
		minHostUpgradeVersion, maxHostDowngradeVersion *version.Version
		deniedHostUpgradeVersions                      []*version.Version
	)

	switch v.majorMinor {
	case talos13.MajorMinor: // upgrades to 1.3.x
		minHostUpgradeVersion, maxHostDowngradeVersion = talos13.MinimumHostUpgradeVersion, talos13.MaximumHostDowngradeVersion
		deniedHostUpgradeVersions = talos13.DeniedHostUpgradeVersions
	case talos14.MajorMinor: // upgrades to 1.4.x
		minHostUpgradeVersion, maxHostDowngradeVersion = talos14.MinimumHostUpgradeVersion, talos14.MaximumHostDowngradeVersion
		deniedHostUpgradeVersions = talos14.DeniedHostUpgradeVersions
	default:
		return fmt.Errorf("upgrades to version %s are not supported", v.version.String())
	}

	if host.version.Core().LessThan(minHostUpgradeVersion) {
		return fmt.Errorf("host version %s is too old to upgrade to Talos %s", host.version.String(), v.version.String())
	}

	if host.version.Core().GreaterThanOrEqual(maxHostDowngradeVersion) {
		return fmt.Errorf("host version %s is too new to downgrade to Talos %s", host.version.String(), v.version.String())
	}

	for _, denied := range deniedHostUpgradeVersions {
		if host.version.Equal(denied) {
			return fmt.Errorf("host version %s is denied for upgrade to Talos %s", host.version.String(), v.version.String())
		}
	}

	return nil
}
