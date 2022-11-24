// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package compatibility

import (
	"fmt"

	"github.com/hashicorp/go-version"

	"github.com/siderolabs/talos/pkg/machinery/compatibility/talos13"
)

// KubernetesVersion embeds Kubernetes version.
type KubernetesVersion struct {
	version version.Version
}

// ParseKubernetesVersion parses Talos version.
func ParseKubernetesVersion(v string) (*KubernetesVersion, error) {
	parsed, err := version.NewVersion(v)
	if err != nil {
		return nil, err
	}

	return &KubernetesVersion{
		version: *parsed,
	}, nil
}

func (v *KubernetesVersion) String() string {
	return v.version.String()
}

// SupportedWith checks if the Kubernetes version is supported with specified version of Talos.
func (v *KubernetesVersion) SupportedWith(target *TalosVersion) error {
	switch target.majorMinor {
	case talos13.MajorMinor: // upgrades to 1.3.x
		if v.version.Core().LessThan(talos13.MinimumKubernetesVersion) {
			return fmt.Errorf("version of Kubernetes %s is too old to be used with Talos %s", v.version.String(), target.version.String())
		}

		if v.version.Core().GreaterThanOrEqual(talos13.MaximumKubernetesVersion) {
			return fmt.Errorf("version of Kubernetes %s is too new to be used with Talos %s", v.version.String(), target.version.String())
		}

		return nil
	default:
		return fmt.Errorf("compatibility with version %s is not supported", target.String())
	}
}
