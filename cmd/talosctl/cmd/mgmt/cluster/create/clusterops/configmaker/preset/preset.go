// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package preset

import (
	"errors"
	"flag"
	"fmt"
	"net/url"
	"runtime"

	"gopkg.in/typ.v4/slices"

	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create/clusterops"
)

const secureBootSuffix = "-secureboot"

// Preset modifies cluster create options to achieve certain behavior.
type Preset interface {
	Name() string
	Description() string

	// ModifyOptions modifies configs to achieve the desired behavior
	ModifyOptions(presetOps Options, cOps *clusterops.Common, qOps *clusterops.Qemu) error
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
	ISOSecureBoot{},
	PXE{},
	DiskImage{},
	Maintenance{},
}

// Apply validates and applies a set of multiple presets.
func Apply(presetOps Options, cOps *clusterops.Common, qOps *clusterops.Qemu, presetNames []string) error {
	presets, err := slices.MapErr(presetNames, func(name string) (Preset, error) {
		if name == (ISOSecureBoot{}).Name() {
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

	err = Validate(presetNames, presetOps)
	if err != nil {
		return err
	}

	if err := applyDefaultSettings(presetOps, cOps, qOps); err != nil {
		return err
	}

	for _, p := range presets {
		err = p.ModifyOptions(presetOps, cOps, qOps)
		if err != nil {
			return fmt.Errorf("failed to apply %q preset: %w", p.Name(), err)
		}
	}

	return nil
}

// Validate checks if the provided presets are valid and compatible.
//
//nolint:gocyclo
func Validate(presetNames []string, presetOps Options) error {
	bootMethodPresets := []string{ISO{}.Name(), PXE{}.Name(), DiskImage{}.Name(), ISOSecureBoot{}.Name()}

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

	if presetOps.secureBoot && runtime.GOOS == "darwin" {
		// skip the check if it's a unit test environment
		if flag.Lookup("test.v") == nil {
			return errors.New("error: 'secureboot' preset is currently not supported on darwin")
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
