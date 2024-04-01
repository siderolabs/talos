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
//
//gotagsrewrite:gen
type Metadata struct {
	Name          string        `yaml:"name" protobuf:"1"`
	Version       string        `yaml:"version" protobuf:"2"`
	Author        string        `yaml:"author" protobuf:"3"`
	Description   string        `yaml:"description" protobuf:"4"`
	Compatibility Compatibility `yaml:"compatibility" protobuf:"5"`
	ExtraInfo     string        `yaml:"extraInfo,omitempty" protobuf:"6"`
}

// Compatibility describes extension compatibility.
//
//gotagsrewrite:gen
type Compatibility struct {
	Talos Constraint `yaml:"talos" protobuf:"1"`
}

// Constraint describes compatibility constraint.
//
//gotagsrewrite:gen
type Constraint struct {
	Version string `yaml:"version" protobuf:"1"`
}
