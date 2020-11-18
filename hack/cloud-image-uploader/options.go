// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"fmt"
	"path/filepath"
)

// Options for the cli.
type Options struct {
	Tag           string
	ArtifactsPath string
	Architectures []string

	AWSRegions []string
}

// DefaultOptions used throughout the cli.
var DefaultOptions = Options{
	ArtifactsPath: "_out/",
	Architectures: []string{"amd64", "arm64"},
}

// AWSImage returns path to AWS pre-built image.
func (o *Options) AWSImage(architecture string) string {
	return filepath.Join(o.ArtifactsPath, fmt.Sprintf("aws-%s.tar.gz", architecture))
}
