// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package extensions

// Manifest is the structure of the extension manifest.yaml file.
type Manifest struct {
	Version  string   `yaml:"version"`
	Metadata Metadata `yaml:"metadata"`
}

// Metadata describes base extension metadata.
type Metadata struct {
	Name          string        `yaml:"name"`
	Version       string        `yaml:"version"`
	Author        string        `yaml:"author"`
	Description   string        `yaml:"description"`
	Compatibility Compatibility `yaml:"compatibility"`
}

// Compatibility describes extension compatibility.
type Compatibility struct {
	Talos Constraint `yaml:"talos"`
}

// Constraint describes compatibility constraint.
type Constraint struct {
	Version string `yaml:"version"`
}
