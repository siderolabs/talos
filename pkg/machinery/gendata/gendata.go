// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package gendata contains that a variables generated from Makefile script. It's a proper alternative to using
// -ldflags '-X ...'.
package gendata

import (
	_ "embed"
)

var (
	// VersionName declares variable used by version package.
	//go:embed data/name
	VersionName string
	// VersionTag declares variable used by version package.
	//go:embed data/tag
	VersionTag string
	// VersionSHA declares variable used by version package.
	//go:embed data/sha
	VersionSHA string
	// VersionPkgs declares variable used by version package.
	//go:embed data/pkgs
	VersionPkgs string
	// ImagesUsername declares variable used by images package.
	//go:embed data/username
	ImagesUsername string
	// ImagesRegistry declares variable used by images package.
	//go:embed data/registry
	ImagesRegistry string
	// ArtifactsPath declares variable used by helpers package.
	//go:embed data/artifacts
	ArtifactsPath string
)
