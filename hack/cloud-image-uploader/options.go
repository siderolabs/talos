// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Options for the cli.
type Options struct {
	Tag           string
	ArtifactsPath string
	NamePrefix    string
	Architectures []string
	TargetClouds  []string

	// ImageFactory options.
	UseFactory        bool
	FactoryHost       string
	FactorySchematics []string

	// AWS options.
	AWSRegions   []string
	AWSForceBIOS bool
}

// DefaultOptions used throughout the cli.
var DefaultOptions = Options{
	ArtifactsPath: "_out/",
	Architectures: []string{"amd64", "arm64"},
	TargetClouds:  []string{"aws"},
	FactoryHost:   "https://factory.talos.dev",
	FactorySchematics: []string{
		"aws:10e276a06c1f86b182757a962258ac00655d3425e5957f617bdc82f06894e39b",
	},
}

// AWSImage returns path to AWS pre-built image.
func (o *Options) AWSImage(architecture string) string {
	return filepath.Join(o.ArtifactsPath, fmt.Sprintf("aws-%s.raw.zst", architecture))
}

// GCPImage returns path to GCP pre-built image.
func (o *Options) GCPImage(architecture string) string {
	return filepath.Join(o.ArtifactsPath, fmt.Sprintf("gcp-%s.raw.tar.gz", architecture))
}

// SchematicFor returns the schematic for the given cloud.
func (o *Options) SchematicFor(cloud string) string {
	for _, schematic := range o.FactorySchematics {
		parts := strings.Split(schematic, ":")
		if len(parts) != 2 {
			continue
		}

		cloudPart := parts[0]
		schematicPart := parts[1]

		if cloudPart == cloud {
			return schematicPart
		}
	}

	return ""
}
