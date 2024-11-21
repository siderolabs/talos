// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package cache provides facilities for generating a cache tarball from images.
package cache

import (
	"cmp"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dustin/go-humanize"
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

	removeAll := sync.OnceValue(func() error { return os.RemoveAll(tmpDir) })

	defer removeAll() //nolint:errcheck

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
		fmt.Fprintf(os.Stderr, "fetching image %q\n", src)

		ref, err := name.ParseReference(src, nameOptions...)
		if err != nil {
			return fmt.Errorf("parsing reference %q: %w", src, err)
		}

		referenceDir := filepath.Join(tmpDir, manifestsDir, ref.Context().RegistryStr(), ref.Context().RepositoryStr(), "reference")
		digestDir := filepath.Join(tmpDir, manifestsDir, ref.Context().RegistryStr(), ref.Context().RepositoryStr(), "digest")

		// get the tag from the reference (if it's there)
		var tag name.Tag

		base, _, ok := strings.Cut(src, "@")
		if !ok {
			tag, _ = name.NewTag(src, nameOptions...) //nolint:errcheck
		} else {
			tag, _ = name.NewTag(base, nameOptions...) //nolint:errcheck
		}

		if err = os.MkdirAll(referenceDir, 0o755); err != nil {
			return err
		}

		if err = os.MkdirAll(digestDir, 0o755); err != nil {
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

		filename := tag.TagStr()
		if filename == "" {
			filename = rmt.Digest.String()
		}

		if err := os.WriteFile(filepath.Join(digestDir, filename), manifest, 0o644); err != nil {
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

		config, err := img.RawConfigFile()
		if err != nil {
			return fmt.Errorf("getting image config: %w", err)
		}

		platformManifest, err := img.RawManifest()
		if err != nil {
			return fmt.Errorf("getting image platform manifest: %w", err)
		}

		h := sha256.New()
		if _, err := h.Write(platformManifest); err != nil {
			return fmt.Errorf("platform manifest hash: %w", err)
		}

		if err := os.WriteFile(filepath.Join(digestDir, fmt.Sprintf("sha256:%x", h.Sum(nil))), platformManifest, 0o644); err != nil {
			return err
		}

		configHash, err := img.ConfigName()
		if err != nil {
			return fmt.Errorf("getting image config hash: %w", err)
		}

		if err := os.WriteFile(filepath.Join(tmpDir, blobsDir, configHash.String()), config, 0o644); err != nil {
			return err
		}

		for _, layer := range layers {
			if err = processLayer(layer, tmpDir); err != nil {
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

	artifactsLayer, err := filemap.Layer(artifacts)
	if err != nil {
		return fmt.Errorf("creating artifacts layer: %w", err)
	}

	newImg, err = mutate.AppendLayers(newImg, artifactsLayer)
	if err != nil {
		return fmt.Errorf("appending artifacts layer: %w", err)
	}

	if err := tarball.WriteToFile(dest, nil, newImg); err != nil {
		return fmt.Errorf("writing tarball: %w", err)
	}

	return removeAll()
}

func processLayer(layer v1.Layer, dstDir string) error {
	digest, err := layer.Digest()
	if err != nil {
		return fmt.Errorf("getting layer digest: %w", err)
	}

	blobPath := filepath.Join(dstDir, blobsDir, digest.String())

	if _, err := os.Stat(blobPath); err == nil {
		// we already have this blob, skip it
		return nil
	}

	size, err := layer.Size()
	if err != nil {
		return fmt.Errorf("getting layer size: %w", err)
	}

	fmt.Fprintf(os.Stderr, "> layer %q (size %s)...\n", digest, humanize.Bytes(uint64(size)))

	reader, err := layer.Compressed()
	if err != nil {
		return fmt.Errorf("getting layer reader: %w", err)
	}

	rdrCloser := sync.OnceValue(reader.Close)
	defer rdrCloser() //nolint:errcheck

	file, err := os.Create(blobPath)
	if err != nil {
		return err
	}

	fileCloser := sync.OnceValue(file.Close)
	defer fileCloser() //nolint:errcheck

	if _, err := io.Copy(file, reader); err != nil {
		return err
	}

	return cmp.Or(rdrCloser(), fileCloser())
}
