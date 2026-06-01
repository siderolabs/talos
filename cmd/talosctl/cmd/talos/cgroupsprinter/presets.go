// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cgroupsprinter

import (
	"embed"
	"io/fs"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/siderolabs/gen/xslices"
	"go.yaml.in/yaml/v4"
)

//go:embed presets/*.yaml
var presetsFS embed.FS

// GetPresetNames returns the list of preset names.
func GetPresetNames() []string {
	list, err := presetsFS.ReadDir("presets")
	if err != nil {
		panic(err) // should not fail
	}

	presets := xslices.Map(list, func(dirEntry fs.DirEntry) string {
		// cut extension
		return strings.TrimSuffix(dirEntry.Name(), filepath.Ext(dirEntry.Name()))
	})

	slices.Sort(presets)

	return presets
}

// GetPreset returns the preset by name.
func GetPreset(name string) Schema {
	// embed.FS always uses / as separator, even on Windows, we need OS-agnostic path joining here
	f, err := presetsFS.Open(path.Join("presets", name+".yaml"))
	if err != nil {
		panic(err) // should not fail
	}

	defer f.Close() //nolint:errcheck

	var schema Schema

	if err := yaml.NewDecoder(f).Decode(&schema); err != nil {
		panic(err) // should not fail
	}

	if err := schema.Compile(); err != nil {
		panic(err) // should not fail
	}

	return schema
}
