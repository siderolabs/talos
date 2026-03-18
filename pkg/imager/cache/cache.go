// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package cache provides facilities for generating a cache tarball from images.
package cache

import (
	"cmp"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
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
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/siderolabs/go-retry/retry"

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
func Generate(images []string, platforms []string, insecure bool, imageLayerCachePath, dest string, flat bool, withCosignSignatures bool) error {
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

	if len(platforms) < 1 {
		return fmt.Errorf("must specify at least one platform")
	}

	allPlatforms := len(platforms) == 1 && platforms[0] == "all"

	if err := os.MkdirAll(filepath.Join(tmpDir, blobsDir), 0o755); err != nil {
		return err
	}

	var nameOptions []name.Option

	keychain := authn.NewMultiKeychain(
		authn.DefaultKeychain,
		github.Keychain,
		google.Keychain,
	)

	craneOpts := []crane.Option{
		crane.WithAuthFromKeychain(keychain),
	}

	sigRemoteOpts := []remote.Option{
		remote.WithAuthFromKeychain(keychain),
	}

	if insecure {
		craneOpts = append(craneOpts, crane.Insecure)
		nameOptions = append(nameOptions, name.Insecure)
	}

	if allPlatforms {
		remoteOpts := []remote.Option{
			remote.WithAuthFromKeychain(keychain),
		}

		if err := retryImages(images, func(src string) error {
			return processImageAllPlatforms(src, tmpDir, imageLayerCachePath, nameOptions, craneOpts, remoteOpts, sigRemoteOpts, withCosignSignatures)
		}); err != nil {
			return err
		}
	} else {
		for _, platform := range platforms {
			v1Platform, err := v1.ParsePlatform(platform)
			if err != nil {
				return fmt.Errorf("parsing platform: %w", err)
			}

			remoteOpts := []remote.Option{
				remote.WithAuthFromKeychain(keychain),
				remote.WithPlatform(*v1Platform),
			}

			if err := retryImages(images, func(src string) error {
				return processImage(src, tmpDir, imageLayerCachePath, platform, nameOptions, craneOpts, remoteOpts, sigRemoteOpts, withCosignSignatures)
			}); err != nil {
				return err
			}
		}
	}

	if flat {
		return move(tmpDir, dest)
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

	outerPlatformStr := "linux/amd64"
	if !allPlatforms {
		outerPlatformStr = platforms[0]
	}

	imagePlatform, err := v1.ParsePlatform(outerPlatformStr)
	if err != nil {
		return fmt.Errorf("parsing platform: %w", err)
	}

	if err := ociLayout.AppendImage(newImg, layout.WithPlatform(*imagePlatform)); err != nil {
		return fmt.Errorf("appending image: %w", err)
	}

	return removeAll()
}

func move(src, dest string) error {
	if err := os.Rename(src, dest); err == nil {
		return nil
	} else if !errors.Is(err, syscall.EXDEV) {
		// not a cross-device error - return it
		return err
	}

	// cross-device: must copy+remove
	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	if info.IsDir() {
		if err := copyDir(src, dest); err != nil {
			return err
		}
	} else {
		if err := copyFile(src, dest, info.Mode()); err != nil {
			return err
		}
	}

	return os.RemoveAll(src)
}

func copyFile(src, dest string, perm fs.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close() //nolint:errcheck

	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}

	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer out.Close() //nolint:errcheck

	if _, err = io.Copy(out, in); err != nil {
		return err
	}

	return nil
}

func copyDir(src, dest string) error {
	return filepath.Walk(src, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		target := filepath.Join(dest, rel)

		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}

		return copyFile(path, target, info.Mode())
	})
}

func retryImages(images []string, fn func(src string) error) error {
	for _, src := range images {
		r := retry.Exponential(
			30*time.Minute,
			retry.WithUnits(time.Second),
			retry.WithJitter(time.Second),
			retry.WithErrorLogging(true),
		)

		if err := r.Retry(func() error {
			if err := fn(src); err != nil {
				switch {
				case errors.Is(err, new(name.ErrBadName)):
					return err

				default:
					return retry.ExpectedError(err)
				}
			}

			return nil
		}); err != nil {
			return fmt.Errorf("failed to process image: %w", err)
		}
	}

	return nil
}

//nolint:gocyclo,cyclop
func processImage(
	src, tmpDir, imageLayerCachePath, platform string,
	nameOptions []name.Option,
	craneOpts []crane.Option,
	remoteOpts []remote.Option,
	sigRemoteOpts []remote.Option,
	withCosignSignatures bool,
) error {
	fmt.Fprintf(os.Stderr, "fetching image %q (%s)\n", src, platform)

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

	if withCosignSignatures {
		if err := processCosignSignature(src, rmt.Digest, tmpDir, nameOptions, craneOpts, sigRemoteOpts); err != nil {
			return err
		}
	}

	img, err := rmt.Image()
	if err != nil {
		return fmt.Errorf("converting image to index: %w", err)
	}

	if imageLayerCachePath != "" {
		img = cache.Image(img, cache.NewFilesystemCache(imageLayerCachePath))
	}

	return cacheImage(img, digestDir, tmpDir)
}

//nolint:gocyclo
func processImageAllPlatforms(
	src, tmpDir, imageLayerCachePath string,
	nameOptions []name.Option,
	craneOpts []crane.Option,
	remoteOpts []remote.Option,
	sigRemoteOpts []remote.Option,
	withCosignSignatures bool,
) error {
	ref, err := name.ParseReference(src, nameOptions...)
	if err != nil {
		return fmt.Errorf("parsing reference %q: %w", src, err)
	}

	rmt, err := remote.Get(ref, remoteOpts...)
	if err != nil {
		return fmt.Errorf("fetching %q: %w", src, err)
	}

	if rmt.MediaType != types.OCIImageIndex && rmt.MediaType != types.DockerManifestList {
		return processImage(src, tmpDir, imageLayerCachePath, "linux/amd64", nameOptions, craneOpts,
			append(remoteOpts, remote.WithPlatform(v1.Platform{OS: "linux", Architecture: "amd64"})),
			sigRemoteOpts, withCosignSignatures)
	}

	idx, err := rmt.ImageIndex()
	if err != nil {
		return fmt.Errorf("getting image index %q: %w", src, err)
	}

	idxManifest, err := idx.IndexManifest()
	if err != nil {
		return fmt.Errorf("getting index manifest %q: %w", src, err)
	}

	digestDir := filepath.Join(tmpDir, manifestsDir, rewriteRegistry(ref.Context().RegistryStr(), src), filepath.FromSlash(ref.Context().RepositoryStr()), "digest")
	if err := os.MkdirAll(digestDir, 0o755); err != nil {
		return err
	}

	first := true

	for _, desc := range idxManifest.Manifests {
		if desc.Platform != nil && desc.Platform.OS != "unknown" {
			platRemoteOpts := append(append([]remote.Option{}, remoteOpts...), remote.WithPlatform(*desc.Platform))
			platformLabel := desc.Platform.OS + "/" + desc.Platform.Architecture

			if err := processImage(src, tmpDir, imageLayerCachePath, platformLabel, nameOptions, craneOpts, platRemoteOpts, sigRemoteOpts, first && withCosignSignatures); err != nil {
				return err
			}

			first = false

			continue
		}

		childImg, err := idx.Image(desc.Digest)
		if err != nil {
			return fmt.Errorf("getting attestation image %s: %w", desc.Digest, err)
		}

		if err := cacheImage(childImg, digestDir, tmpDir); err != nil {
			return err
		}
	}

	return nil
}

//nolint:gocyclo,cyclop
func processCosignSignature(
	src string,
	digest v1.Hash,
	tmpDir string,
	nameOptions []name.Option,
	craneOpts []crane.Option,
	remoteOpts []remote.Option,
) error {
	ref, err := name.ParseReference(src, nameOptions...)
	if err != nil {
		return fmt.Errorf("parsing reference for cosign %q: %w", src, err)
	}

	baseTag := strings.ReplaceAll(digest.String(), "sha256:", "sha256-")

	for _, sigTagStr := range []string{baseTag + ".sig", baseTag} {
		sigRef, err := name.NewTag(ref.Context().String()+":"+sigTagStr, nameOptions...)
		if err != nil {
			continue
		}

		fmt.Fprintf(os.Stderr, "fetching cosign signature %q\n", sigRef.String())

		sigManifest, err := crane.Manifest(sigRef.String(), craneOpts...)
		if err != nil {
			var transportErr *transport.Error
			if errors.As(err, &transportErr) && transportErr.StatusCode == http.StatusNotFound {
				continue
			}

			return fmt.Errorf("fetching cosign manifest %q: %w", sigRef.String(), err)
		}

		sigRmt, err := remote.Get(sigRef, remoteOpts...)
		if err != nil {
			var transportErr *transport.Error
			if errors.As(err, &transportErr) && transportErr.StatusCode == http.StatusNotFound {
				continue
			}

			return fmt.Errorf("fetching cosign image %q: %w", sigRef.String(), err)
		}

		referenceDir := filepath.Join(tmpDir, manifestsDir, rewriteRegistry(sigRef.Context().RegistryStr(), src), filepath.FromSlash(sigRef.Context().RepositoryStr()), "reference")
		digestDir := filepath.Join(tmpDir, manifestsDir, rewriteRegistry(sigRef.Context().RegistryStr(), src), filepath.FromSlash(sigRef.Context().RepositoryStr()), "digest")

		if err = os.MkdirAll(referenceDir, 0o755); err != nil {
			return err
		}

		if err = os.MkdirAll(digestDir, 0o755); err != nil {
			return err
		}

		if err := os.WriteFile(filepath.Join(referenceDir, sigTagStr), sigManifest, 0o644); err != nil {
			return err
		}

		if err := os.WriteFile(filepath.Join(digestDir, strings.ReplaceAll(sigRmt.Digest.String(), "sha256:", "sha256-")), sigManifest, 0o644); err != nil {
			return err
		}

		switch sigRmt.MediaType { //nolint:exhaustive
		case types.OCIImageIndex, types.DockerManifestList:
			idx, err := sigRmt.ImageIndex()
			if err != nil {
				return fmt.Errorf("getting cosign image index: %w", err)
			}

			idxManifest, err := idx.IndexManifest()
			if err != nil {
				return fmt.Errorf("getting cosign index manifest: %w", err)
			}

			for _, descriptor := range idxManifest.Manifests {
				childImg, err := idx.Image(descriptor.Digest)
				if err != nil {
					return fmt.Errorf("getting cosign child image %s: %w", descriptor.Digest, err)
				}

				if err := cacheImage(childImg, digestDir, tmpDir); err != nil {
					return err
				}
			}
		default:
			img, err := sigRmt.Image()
			if err != nil {
				return fmt.Errorf("getting cosign image: %w", err)
			}

			if err := cacheImage(img, digestDir, tmpDir); err != nil {
				return err
			}
		}

		return nil
	}

	return nil
}

func cacheImage(img v1.Image, digestDir, tmpDir string) error {
	manifest, err := img.RawManifest()
	if err != nil {
		return fmt.Errorf("getting image manifest: %w", err)
	}

	h := sha256.New()
	if _, err := h.Write(manifest); err != nil {
		return fmt.Errorf("hashing manifest: %w", err)
	}

	if err := os.WriteFile(filepath.Join(digestDir, fmt.Sprintf("sha256-%x", h.Sum(nil))), manifest, 0o644); err != nil {
		return err
	}

	config, err := img.RawConfigFile()
	if err != nil {
		return fmt.Errorf("getting image config: %w", err)
	}

	configHash, err := img.ConfigName()
	if err != nil {
		return fmt.Errorf("getting image config hash: %w", err)
	}

	if err := os.WriteFile(filepath.Join(tmpDir, blobsDir, strings.ReplaceAll(configHash.String(), "sha256:", "sha256-")), config, 0o644); err != nil {
		return err
	}

	layers, err := img.Layers()
	if err != nil {
		return fmt.Errorf("getting image layers: %w", err)
	}

	for _, layer := range layers {
		if err := processLayer(layer, tmpDir); err != nil {
			return err
		}
	}

	return nil
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
