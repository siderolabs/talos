// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package preset

import (
	"fmt"
	"net/url"

	"gopkg.in/typ.v4/slices"

	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create/clusterops"
)

const secureBootSuffix = "-secureboot"

// Preset modifies cluster create options to achieve certain behavior.
type Preset interface {
	Name() string
	Description() string

	// ModifuOptions modifies configs to achieve the desired behavior
	ModifuOptions(presetOps Options, cOps *clusterops.Common, qOps *clusterops.Qemu) error
}

// Options are the options required for presets to function.
type Options struct {
	SchematicID     string
	ImageFactoryURL *url.URL

	// secureBoot preset also affects other presets so this option needs to be shared.
	secureBoot bool
}

// Presets is a list of all available presets.
var Presets = [...]Preset{
	ISO{},
	PXE{},
	DiskImage{},
	Maintenance{},
	SecureBoot{},
}

// Apply validates and applies a set of multiple presets.
func Apply(presetOps Options, cOps *clusterops.Common, qOps *clusterops.Qemu, presetNames []string) error {
	presets, err := slices.MapErr(presetNames, func(name string) (Preset, error) {
		if name == (SecureBoot{}).Name() {
			presetOps.secureBoot = true
		}

		for _, p := range Presets {
			if p.Name() == name {
				return p, nil
			}
		}

		return nil, fmt.Errorf("error: unknown preset: %q", name)
	})
	if err != nil {
		return err
	}

	err = ValidatePresets(presetNames, presetOps)
	if err != nil {
		return err
	}

	if err := applyDefaultSettings(presetOps, cOps, qOps); err != nil {
		return err
	}

	for _, p := range presets {
		err = p.ModifuOptions(presetOps, cOps, qOps)
		if err != nil {
			return fmt.Errorf("failed to apply %q preset: %w", p.Name(), err)
		}
	}

	return nil
}

func ValidatePresets(presetNames []string, presetOps Options) error {
	bootMethodPresets := []string{ISO{}.Name(), PXE{}.Name(), DiskImage{}.Name()}
	// check if at least one boot method preset is selected, but no more than one
	bootMethodPresetCount := 0
	for _, name := range presetNames {
		for _, bm := range bootMethodPresets {
			if name == bm {
				bootMethodPresetCount++
			}
		}
	}

	if bootMethodPresetCount == 0 {
		return fmt.Errorf("error: at least one boot method preset must be specified (one of %v)", bootMethodPresets)
	}

	if bootMethodPresetCount > 1 {
		return fmt.Errorf("error: multiple boot method presets specified, please select only one (one of %v)", bootMethodPresets)
	}

	// when secure boot is enabled ensure that iso preset is selected
	if presetOps.secureBoot {
		found := false
		isoPresetName := ISO{}.Name()
		for _, name := range presetNames {
			if name == isoPresetName {
				found = true
			}
		}

		if !found {
			return fmt.Errorf("error: secureboot preset can only be used with the iso preset")
		}
	}

	return nil
}

func applyDefaultSettings(presetOps Options, cOps *clusterops.Common, qOps *clusterops.Qemu) error {
	installerName := "metal-installer"
	if presetOps.secureBoot {
		installerName += secureBootSuffix
	}

	installerURL, err := url.JoinPath(presetOps.ImageFactoryURL.Host, installerName, presetOps.SchematicID+":"+cOps.TalosVersion)
	if err != nil {
		return fmt.Errorf("failed to build installer image URL: %w", err)
	}

	qOps.NodeInstallImage = installerURL

	return nil
}
