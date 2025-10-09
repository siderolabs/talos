// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package artifacts

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/siderolabs/go-retry/retry"
)

// Mirror mirrors the given images to the destination registry.
func Mirror(ctx context.Context, options []crane.Option, images []string, destRegistry string) error {
	// TODO:
	// - do not use crane, but puller/pusher directly
	// - validate signatures on copy
	// - rewrite signatures
	for i, image := range images {
		ref, err := name.ParseReference(image)
		if err != nil {
			return fmt.Errorf("error parsing image reference %q: %w", image, err)
		}

		registry := ref.Context().RegistryStr()
		repo := ref.Context().RepositoryStr()
		tag := extractTagAndDigest(image)

		slog.Info("mirroring image",
			"src", registry,
			"dest", destRegistry,
			"image", repo,
			"tag", tag,
			"progress", fmt.Sprintf("%d/%d", i+1, len(images)),
		)

		destRef, err := rewriteRegistry(ref, destRegistry)
		if err != nil {
			return fmt.Errorf("error rewriting registry for image %q: %w", image, err)
		}

		srcImage := ref.Name()
		destImage := destRef.Name()

		digest, err := copyManifest(ctx, options, srcImage, destImage)
		if err != nil {
			return fmt.Errorf("error mirroring image %d/%d (%q to %q): %w", i+1, len(images), srcImage, destImage, err)
		}

		srcSigRef, destSigRef, err := makeSignature(ref, digest, destRegistry)
		if err != nil {
			return fmt.Errorf("error getting signature for image %q (%d/%d): %w", srcImage, i+1, len(images), err)
		}

		srcSigRegistry := srcSigRef.Context().RegistryStr()
		destSigRegistry := destSigRef.Context().RegistryStr()
		sigRepo := srcSigRef.Context().RepositoryStr()
		sigTag := extractTagAndDigest(srcSigRef.Name())

		slog.Info("mirroring signature",
			"src", srcSigRegistry,
			"dest", destSigRegistry,
			"image", sigRepo,
			"tag", sigTag,
			"progress", fmt.Sprintf("%d/%d", i+1, len(images)),
		)

		srcSig := srcSigRef.Name()
		destSig := destSigRef.Name()

		if _, err := copyManifest(ctx, options, srcSig, destSig); err != nil {
			// We only log the error, not all images will have signatures
			slog.Debug("error mirroring signature",
				"error", err,
				"src", srcSigRegistry,
				"dest", destSigRegistry,
				"image", sigRepo,
				"tag", sigTag,
				"progress", fmt.Sprintf("%d/%d", i+1, len(images)),
			)
		}
	}

	return nil
}

func extractTagAndDigest(image string) string {
	// Find the repository/image separator
	lastSlash := strings.LastIndex(image, "/")
	if lastSlash == -1 {
		lastSlash = -1
	}

	remainder := image[lastSlash+1:]

	// Look for both tag and digest: name:tag@digest
	if tagIdx := strings.Index(remainder, ":"); tagIdx != -1 {
		if digestIdx := strings.Index(remainder, "@"); digestIdx != -1 {
			// Both present: :tag@digest
			return remainder[tagIdx:]
		}
		// Only tag: :tag
		return remainder[tagIdx:]
	}

	// Only digest: @digest
	if digestIdx := strings.Index(remainder, "@"); digestIdx != -1 {
		return remainder[digestIdx:]
	}

	return ""
}

func rewriteRegistry(ref name.Reference, newRegistry string) (name.Reference, error) {
	repo := ref.Context().RepositoryStr()

	// Create new repository with the new registry
	newRepo, err := name.NewRepository(newRegistry + "/" + repo)
	if err != nil {
		return nil, err
	}

	// Recreate the reference with the new repository
	switch r := ref.(type) {
	case name.Tag:
		return name.NewTag(newRepo.String() + ":" + r.TagStr())
	case name.Digest:
		return name.NewDigest(newRepo.String() + "@" + r.DigestStr())
	default:
		return name.NewTag(newRepo.String() + ":latest")
	}
}

func makeSignature(ref name.Reference, digest, destRegistry string) (name.Reference, name.Reference, error) {
	sigTag := strings.ReplaceAll(digest, ":", "-") + ".sig"

	// Create signature reference with the same repository but signature tag
	repo := ref.Context()

	srcSigRef, err := name.NewTag(repo.String() + ":" + sigTag)
	if err != nil {
		return nil, nil, err
	}

	destSigRef, err := rewriteRegistry(srcSigRef, destRegistry)
	if err != nil {
		return nil, nil, err
	}

	return srcSigRef, destSigRef, nil
}

func copyManifest(ctx context.Context, options []crane.Option, src, dest string) (string, error) {
	r := retry.Exponential(
		30*time.Minute,
		retry.WithUnits(time.Second),
		retry.WithJitter(time.Second),
		retry.WithErrorLogging(true),
	)

	isChecksum := strings.HasSuffix(src, ".sig")

	var digest string

	if err := r.RetryWithContext(
		ctx, func(ctx context.Context) error {
			options = append(options,
				crane.WithContext(ctx),
			)

			srcByDigest := src

			if !strings.Contains(src, "@sha256:") {
				d, err := crane.Digest(src, options...)
				if err != nil {
					if isChecksum && strings.Contains(err.Error(), "MANIFEST_UNKNOWN") {
						// Signatures are not available for all images, do not retry
						return fmt.Errorf("signature not found for %s", src)
					}

					return retry.ExpectedError(err)
				}

				srcByDigest = fmt.Sprintf("%s@%s", src, d)
			}

			if err := crane.Copy(
				srcByDigest, dest,
				options...,
			); err != nil {
				return retry.ExpectedError(err)
			}

			digest = strings.Split(srcByDigest, "@")[1]

			return nil
		}); err != nil {
		return digest, fmt.Errorf("error copying manifest: %w", err)
	}

	return digest, nil
}
