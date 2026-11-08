// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package overlay provides an interface for overlay installers.
package overlay

import "context"

// Installer is an interface for overlay installers.
type Installer[T any] interface {
	GetOptions(ctx context.Context, extra T) (Options, error)
	Install(ctx context.Context, options InstallOptions[T]) error
}

// Options for the overlay installer.
type Options struct {
	Name             string           `yaml:"name"`
	KernelArgs       []string         `yaml:"kernelArgs,omitempty"`
	PartitionOptions PartitionOptions `yaml:"partitionOptions,omitempty"`
	// DeviceTree is the artifacts-relative path to a base device tree blob to
	// embed in a measured UKI `.dtb` section. Empty means no DTB is embedded.
	//
	// The DTB must be built with symbols (dtc -@) if DeviceTreeOverlays are to
	// be applied on top of it.
	DeviceTree string `yaml:"deviceTree,omitempty"`
	// DeviceTreeOverlays are artifacts-relative paths to device tree overlays
	// (.dtbo) applied, in order, on top of DeviceTree before embedding. The
	// overlay selects these from its extra options (e.g. dtOverlays), so
	// applying a board overlay stays opt-in. An entry may be a directory, which
	// applies every .dtbo it contains (lexical order); this lets an artifact ship
	// a set of overlays under a conventional path, e.g. bundled with a u-boot
	// variant, without naming each file.
	DeviceTreeOverlays []string `yaml:"deviceTreeOverlays,omitempty"`
	// DeviceTreeOverlaysInline are raw overlay blobs applied, in order, on top of
	// DeviceTree (after DeviceTreeOverlays). These carry overlays passed at
	// build time (e.g. via an extra option) that don't ship in the artifacts,
	// so a user can inject a local overlay without baking it into an image.
	//
	// Each blob is either a compiled .dtbo (flattened device tree magic) or .dts
	// source, which the imager compiles with its bundled dtc; inline source must
	// be self-contained (no cpp includes). The overlays are applied by the imager
	// with its bundled fdtoverlay/dtc, so an overlay only selects what to merge,
	// not the tool.
	DeviceTreeOverlaysInline [][]byte `yaml:"deviceTreeOverlaysInline,omitempty"`
}

// PartitionOptions for the overlay installer.
type PartitionOptions struct {
	Offset uint64 `yaml:"offset,omitempty"`
}

// InstallOptions for the overlay installer.
type InstallOptions[T any] struct {
	InstallDisk   string `yaml:"installDisk"`
	MountPrefix   string `yaml:"mountPrefix"`
	ArtifactsPath string `yaml:"artifactsPath"`
	ExtraOptions  T      `yaml:"extraOptions,omitempty"`
}

// ExtraOptions for the overlay installer.
type ExtraOptions map[string]any
