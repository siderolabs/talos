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
	"github.com/google/go-containerregistry/pkg/v1/cache"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/google"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"

	"github.com/siderolabs/talos/pkg/imager/filemap"
)

const (
	blobsDir     = "blob"
	manifestsDir = "manifests"
)

// rewriteRegistry name back to workaround https://github.com/google/go-containerregistry/pull/69.
func rewriteRegistry(registryName, origRef string) string {
	if registryName == name.DefaultRegistry && !strings.HasPrefix(origRef, name.DefaultRegistry+"/") {
		return "docker.io"
	}

	// convert :port to _port_ to support copying image-cache to vfat filesystems
	idx := strings.LastIndex(registryName, ":")
	if idx > 0 {
		return registryName[:idx] + "_" + registryName[idx+1:] + "_"
	}

	return registryName
}

// Generate generates a cache tarball from the given images.
//
//nolint:gocyclo,cyclop
func Generate(images []string, platform string, insecure bool, imageLayerCachePath, dest string) error {
	v1Platform, err := v1.ParsePlatform(platform)
	if err != nil {
		return fmt.Errorf("parsing platform: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "talos-image-cache-gen")
	if err != nil {
		return fmt.Errorf("creating temporary directory: %w", err)
	}

	if imageLayerCachePath != "" {
		if err := os.MkdirAll(imageLayerCachePath, 0o755); err != nil {
			return fmt.Errorf("creating image layer cache directory: %w", err)
		}
	}

	removeAll := sync.OnceValue(func() error { return os.RemoveAll(tmpDir) })

	defer removeAll() //nolint:errcheck

	if err := os.MkdirAll(filepath.Join(tmpDir, blobsDir), 0o755); err != nil {
		return err
	}

	var nameOptions []name.Option

	craneOpts := []crane.Option{
		crane.WithAuthFromKeychain(
			authn.NewMultiKeychain(
				authn.DefaultKeychain,
				github.Keychain,
				google.Keychain,
			),
		),
	}

	remoteOpts := []remote.Option{
		remote.WithAuthFromKeychain(authn.NewMultiKeychain(
			authn.DefaultKeychain,
			github.Keychain,
			google.Keychain,
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

		referenceDir := filepath.Join(tmpDir, manifestsDir, rewriteRegistry(ref.Context().RegistryStr(), src), filepath.FromSlash(ref.Context().RepositoryStr()), "reference")
		digestDir := filepath.Join(tmpDir, manifestsDir, rewriteRegistry(ref.Context().RegistryStr(), src), filepath.FromSlash(ref.Context().RepositoryStr()), "digest")

		// if the reference was parsed as a tag, use it
		tag, ok := ref.(name.Tag)

		if !ok {
			if base, _, ok := strings.Cut(src, "@"); ok {
				// if the reference was a digest, but contained a tag, re-parse it
				tag, _ = name.NewTag(base, nameOptions...) //nolint:errcheck
			}
		}

		if err = os.MkdirAll(referenceDir, 0o755); err != nil {
			return err
		}

		if err = os.MkdirAll(digestDir, 0o755); err != nil {
			return err
		}

		manifest, err := crane.Manifest(
			ref.String(),
			craneOpts...,
		)
		if err != nil {
			return fmt.Errorf("fetching manifest %q: %w", ref.String(), err)
		}

		rmt, err := remote.Get(
			ref,
			remoteOpts...,
		)
		if err != nil {
			return fmt.Errorf("fetching image %q: %w", ref.String(), err)
		}

		if tag.TagStr() != "" {
			if err := os.WriteFile(filepath.Join(referenceDir, tag.TagStr()), manifest, 0o644); err != nil {
				return err
			}
		}

		if err := os.WriteFile(filepath.Join(digestDir, strings.ReplaceAll(rmt.Digest.String(), "sha256:", "sha256-")), manifest, 0o644); err != nil {
			return err
		}

		img, err := rmt.Image()
		if err != nil {
			return fmt.Errorf("converting image to index: %w", err)
		}

		if imageLayerCachePath != "" {
			img = cache.Image(img, cache.NewFilesystemCache(imageLayerCachePath))
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

		if err := os.WriteFile(filepath.Join(digestDir, fmt.Sprintf("sha256-%x", h.Sum(nil))), platformManifest, 0o644); err != nil {
			return err
		}

		configHash, err := img.ConfigName()
		if err != nil {
			return fmt.Errorf("getting image config hash: %w", err)
		}

		if err := os.WriteFile(filepath.Join(tmpDir, blobsDir, strings.ReplaceAll(configHash.String(), "sha256:", "sha256-")), config, 0o644); err != nil {
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

	// we can always write an empty index, since dest is always a new empty directory
	ociLayout, err := layout.Write(dest, empty.Index)
	if err != nil {
		return fmt.Errorf("creating layout: %w", err)
	}

	if err := ociLayout.AppendImage(newImg, layout.WithPlatform(*v1Platform)); err != nil {
		return fmt.Errorf("appending image: %w", err)
	}

	return removeAll()
}

func processLayer(layer v1.Layer, dstDir string) error {
	digest, err := layer.Digest()
	if err != nil {
		return fmt.Errorf("getting layer digest: %w", err)
	}

	blobPath := filepath.Join(dstDir, blobsDir, strings.ReplaceAll(digest.String(), "sha256:", "sha256-"))

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
