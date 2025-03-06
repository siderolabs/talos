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
	NamePrefix    string
	Architectures []string
	TargetClouds  []string

	// AWS options.
	AWSRegions []string

	// Azure options.
	AzureRegions     []string
	AzureGalleryName string
	AzurePreRelease  string
}

// DefaultOptions used throughout the cli.
var DefaultOptions = Options{
	ArtifactsPath: "_out/",
	Architectures: []string{"amd64", "arm64"},
	TargetClouds:  []string{"aws", "azure"},
}

// AWSImage returns path to AWS pre-built image.
func (o *Options) AWSImage(architecture string) string {
	return filepath.Join(o.ArtifactsPath, fmt.Sprintf("aws-%s.raw.zst", architecture))
}

// AzureImage returns path to AWS pre-built image.
func (o *Options) AzureImage(architecture string) string {
	return filepath.Join(o.ArtifactsPath, fmt.Sprintf("azure-%s.vhd.zst", architecture))
}

// GCPImage returns path to GCP pre-built image.
func (o *Options) GCPImage(architecture string) string {
	return filepath.Join(o.ArtifactsPath, fmt.Sprintf("gcp-%s.raw.tar.gz", architecture))
}
