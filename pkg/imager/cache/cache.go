// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package cache provides facilities for generating a cache tarball from images.
package cache

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/authn/github"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/google/go-containerregistry/pkg/v1/types"

	"github.com/siderolabs/talos/pkg/imager/filemap"
)

const (
	blobsDir     = "blob"
	manifestsDir = "manifests"
)

// Generate generates a cache tarball from the given images.
//
//nolint:gocyclo,cyclop
func Generate(images []string, platform string, insecure bool, dest string) error {
	v1Platform, err := v1.ParsePlatform(platform)
	if err != nil {
		return fmt.Errorf("parsing platform: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "talos-image-cache-gen")
	if err != nil {
		return fmt.Errorf("creating temporary directory: %w", err)
	}

	defer os.RemoveAll(tmpDir) //nolint:errcheck

	if err := os.MkdirAll(filepath.Join(tmpDir, blobsDir), 0o755); err != nil {
		return err
	}

	nameOptions := []name.Option{
		name.StrictValidation,
	}

	craneOpts := []crane.Option{
		crane.WithAuthFromKeychain(
			authn.NewMultiKeychain(
				authn.DefaultKeychain,
				github.Keychain,
			),
		),
	}

	remoteOpts := []remote.Option{
		remote.WithAuthFromKeychain(authn.NewMultiKeychain(
			authn.DefaultKeychain,
			github.Keychain,
		)),
		remote.WithPlatform(*v1Platform),
	}

	if insecure {
		craneOpts = append(craneOpts, crane.Insecure)
		nameOptions = append(nameOptions, name.Insecure)
	}

	for _, src := range images {
		ref, err := name.ParseReference(src, nameOptions...)
		if err != nil {
			return fmt.Errorf("parsing reference %q: %w", src, err)
		}

		if err := os.MkdirAll(filepath.Join(tmpDir, manifestsDir, ref.Context().RegistryStr(), ref.Context().RepositoryStr(), "reference"), 0o755); err != nil {
			return err
		}

		if err := os.MkdirAll(filepath.Join(tmpDir, manifestsDir, ref.Context().RegistryStr(), ref.Context().RepositoryStr(), "digest"), 0o755); err != nil {
			return err
		}

		manifest, err := crane.Manifest(
			src,
			craneOpts...,
		)
		if err != nil {
			return fmt.Errorf("fetching manifest %q: %w", src, err)
		}

		rmt, err := remote.Get(
			ref,
			remoteOpts...,
		)
		if err != nil {
			return fmt.Errorf("fetching image %q: %w", src, err)
		}

		if !strings.HasPrefix(ref.Identifier(), "sha256:") {
			if err := os.WriteFile(filepath.Join(tmpDir, manifestsDir, ref.Context().RegistryStr(), ref.Context().RepositoryStr(), "reference", ref.Identifier()), manifest, 0o644); err != nil {
				return err
			}
		}

		if err := os.WriteFile(filepath.Join(tmpDir, manifestsDir, ref.Context().RegistryStr(), ref.Context().RepositoryStr(), "digest", rmt.Digest.String()), manifest, 0o644); err != nil {
			return err
		}

		img, err := rmt.Image()
		if err != nil {
			return fmt.Errorf("converting image to index: %w", err)
		}

		layers, err := img.Layers()
		if err != nil {
			return fmt.Errorf("getting image layers: %w", err)
		}

		for _, layer := range layers {
			digest, err := layer.Digest()
			if err != nil {
				return fmt.Errorf("getting layer digest: %w", err)
			}

			reader, err := layer.Compressed()
			if err != nil {
				return fmt.Errorf("getting layer reader: %w", err)
			}

			file, err := os.Create(filepath.Join(tmpDir, blobsDir, digest.String()))
			if err != nil {
				return err
			}

			if _, err := io.Copy(file, reader); err != nil {
				if err := file.Close(); err != nil {
					return err
				}

				return err
			}

			if err := file.Close(); err != nil {
				return err
			}
		}
	}

	newImg := mutate.MediaType(empty.Image, types.OCIManifestSchema1)
	newImg = mutate.ConfigMediaType(newImg, types.OCIConfigJSON)

	newImg, err = mutate.CreatedAt(newImg, v1.Time{Time: time.Now()})
	if err != nil {
		return fmt.Errorf("setting created at: %w", err)
	}

	artifacts, err := filemap.Walk(tmpDir, "")
	if err != nil {
		return fmt.Errorf("walking filesystem: %w", err)
	}

	for i := range artifacts {
		artifacts[i].ImageMode = 0o644
	}

	artifactsLayer, err := filemap.Layer(artifacts)
	if err != nil {
		return fmt.Errorf("creating artifacts layer: %w", err)
	}

	newImg, err = mutate.AppendLayers(newImg, artifactsLayer)
	if err != nil {
		return fmt.Errorf("appending artifacts layer: %w", err)
	}

	return tarball.WriteToFile(dest, nil, newImg)
}
