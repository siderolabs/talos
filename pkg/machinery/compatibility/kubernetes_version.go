// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package compatibility

import (
	"fmt"

	"github.com/blang/semver/v4"
	"github.com/siderolabs/gen/pair/ordered"

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

// KubernetesVersion embeds Kubernetes version.
type KubernetesVersion struct {
	vers semver.Version
}

// ParseKubernetesVersion parses Kubernetes version.
func ParseKubernetesVersion(v string) (*KubernetesVersion, error) {
	parsed, err := semver.ParseTolerant(v)
	if err != nil {
		return nil, err
	}

	return &KubernetesVersion{
		vers: parsed,
	}, nil
}

func (v *KubernetesVersion) String() string {
	return v.vers.String()
}

// SupportedWith checks if the Kubernetes version is supported with specified version of Talos.
//
//nolint:gocyclo
func (v *KubernetesVersion) SupportedWith(target *TalosVersion) error {
	var minK8sVersion, maxK8sVersion semver.Version

	switch target.majorMinor {
	case talos12.MajorMinor: // upgrades to 1.2.x
		minK8sVersion, maxK8sVersion = talos12.MinimumKubernetesVersion, talos12.MaximumKubernetesVersion
	case talos13.MajorMinor: // upgrades to 1.3.x
		minK8sVersion, maxK8sVersion = talos13.MinimumKubernetesVersion, talos13.MaximumKubernetesVersion
	case talos14.MajorMinor: // upgrades to 1.4.x
		minK8sVersion, maxK8sVersion = talos14.MinimumKubernetesVersion, talos14.MaximumKubernetesVersion
	case talos15.MajorMinor: // upgrades to 1.5.x
		minK8sVersion, maxK8sVersion = talos15.MinimumKubernetesVersion, talos15.MaximumKubernetesVersion
	case talos16.MajorMinor: // upgrades to 1.6.x
		minK8sVersion, maxK8sVersion = talos16.MinimumKubernetesVersion, talos16.MaximumKubernetesVersion
	case talos17.MajorMinor: // upgrades to 1.7.x
		minK8sVersion, maxK8sVersion = talos17.MinimumKubernetesVersion, talos17.MaximumKubernetesVersion
	case talos18.MajorMinor: // upgrades to 1.8.x
		minK8sVersion, maxK8sVersion = talos18.MinimumKubernetesVersion, talos18.MaximumKubernetesVersion
	case talos19.MajorMinor: // upgrades to 1.9.x
		minK8sVersion, maxK8sVersion = talos19.MinimumKubernetesVersion, talos19.MaximumKubernetesVersion
	case talos110.MajorMinor: // upgrades to 1.10.x
		minK8sVersion, maxK8sVersion = talos110.MinimumKubernetesVersion, talos110.MaximumKubernetesVersion
	case talos111.MajorMinor: // upgrades to 1.11.x
		minK8sVersion, maxK8sVersion = talos111.MinimumKubernetesVersion, talos111.MaximumKubernetesVersion
	default:
		return fmt.Errorf("compatibility with version %s is not supported", target.String())
	}

	core := ordered.MakeTriple(v.vers.Major, v.vers.Minor, v.vers.Patch)
	minK8sVersionCore := ordered.MakeTriple(minK8sVersion.Major, minK8sVersion.Minor, minK8sVersion.Patch)

	if core.LessThan(minK8sVersionCore) {
		return fmt.Errorf("version of Kubernetes %s is too old to be used with Talos %s", v.vers.String(), target.version.String())
	}

	maxK8sVersionCore := ordered.MakeTriple(maxK8sVersion.Major, maxK8sVersion.Minor, maxK8sVersion.Patch)

	if core.Compare(maxK8sVersionCore) >= 0 {
		return fmt.Errorf("version of Kubernetes %s is too new to be used with Talos %s", v.vers.String(), target.version.String())
	}

	return nil
}
