// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Copyright 2022 Nokia

package image

import (
	"archive/tar"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/namespaces"
	"github.com/hashicorp/go-multierror"
	"github.com/talos-systems/go-retry/retry"

	containerdrunner "github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/containerd"
	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/safepath"
)

// Image pull retry settings.
const (
	PullTimeout       = 20 * time.Minute
	PullRetryInterval = 5 * time.Second
)

// Image import retry settings.
const (
	ImportTimeout       = 5 * time.Minute
	ImportRetryInterval = 5 * time.Second
	ImportRetryJitter   = time.Second
)

// PullOption is an option for Pull function.
type PullOption func(*PullOptions)

// PullOptions configure Pull function.
type PullOptions struct {
	SkipIfAlreadyPulled bool
}

// WithSkipIfAlreadyPulled skips pulling if image is already pulled and unpacked.
func WithSkipIfAlreadyPulled() PullOption {
	return func(opts *PullOptions) {
		opts.SkipIfAlreadyPulled = true
	}
}

// Pull is a convenience function that wraps the containerd image pull func with
// retry functionality.
func Pull(ctx context.Context, reg config.Registries, client *containerd.Client, ref string, opt ...PullOption) (img containerd.Image, err error) {
	var opts PullOptions

	for _, o := range opt {
		o(&opts)
	}

	if opts.SkipIfAlreadyPulled {
		img, err = imageIsPulledAndUnpacked(ctx, client, ref)
		if err == nil {
			return img, nil
		}
	}

	resolver := NewResolver(reg)

	err = retry.Exponential(PullTimeout, retry.WithUnits(PullRetryInterval), retry.WithErrorLogging(true)).Retry(func() error {
		if img, err = client.Pull(
			ctx,
			ref,
			containerd.WithPullUnpack,
			containerd.WithResolver(resolver),
			containerd.WithChildLabelMap(images.ChildGCLabelsFilterLayers),
		); err != nil {
			err = fmt.Errorf("failed to pull image %q: %w", ref, err)

			if errdefs.IsNotFound(err) || errdefs.IsCanceled(err) {
				return err
			}

			return retry.ExpectedError(err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return img, nil
}

// Import is a convenience function that wraps containerd image import with retries.
func Import(ctx context.Context, imagePath, indexName string) error {
	importer := containerdrunner.NewImporter(constants.SystemContainerdNamespace, containerdrunner.WithContainerdAddress(constants.SystemContainerdAddress))

	return retry.Exponential(ImportTimeout, retry.WithUnits(ImportRetryInterval), retry.WithJitter(ImportRetryJitter), retry.WithErrorLogging(true)).Retry(func() error {
		err := retry.ExpectedError(importer.Import(ctx, &containerdrunner.ImportRequest{
			Path: imagePath,
			Options: []containerd.ImportOpt{
				containerd.WithIndexName(indexName),
			},
		}))

		if err != nil && os.IsNotExist(err) {
			return err
		}

		return retry.ExpectedError(err)
	})
}

// LoadImagesFromCache is a convenience function that imports container images from a specified image cache/archive.
func LoadImagesFromCache(ctx context.Context, client *containerd.Client, archive config.ImageCache, ref string) error {
	archivePath := archive.Path()

	err := existsAndIsDirectory(archivePath)
	if err != nil {
		return err
	}

	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	//nolint:errcheck
	defer f.Close()

	tarNames, err := f.Readdirnames(-1)
	if err != nil {
		return err
	}

	if len(tarNames) == 0 {
		return fmt.Errorf("cache directory (%q) is empty", archivePath)
	}

	ctx = namespaces.WithNamespace(ctx, archive.Namespace())

	// If reference is defined (for kubelet and etcd services), only matched tarfile is imported.
	if ref != "" {
		_, err = imageIsPulledAndUnpacked(ctx, client, ref)
		if err == nil {
			return nil
		}

		var matched string

		matched, err = getCorrectTarFile(tarNames, archivePath, ref)
		if err != nil {
			return err
		}

		tarNames[0] = matched
		tarNames = tarNames[0:1]
	}

	err = importImagesFromCache(ctx, client, tarNames, archivePath, archive.Namespace())
	if err != nil {
		return err
	}

	return nil
}

func imageIsPulledAndUnpacked(ctx context.Context, client *containerd.Client, ref string) (img containerd.Image, err error) {
	img, err = client.GetImage(ctx, ref)
	if err == nil {
		var unpacked bool

		unpacked, err = img.IsUnpacked(ctx, "")
		if err == nil && unpacked {
			return img, nil
		}
	}

	return img, err
}

func getCorrectTarFile(tarFiles []string, path, ref string) (string, error) {
	matched := ""

	var errs *multierror.Error

	for _, tarFile := range tarFiles {
		repoTags, err := extractImageName(filepath.Join(path, tarFile))
		if err != nil {
			errs = multierror.Append(errs, err)

			continue
		}

		if len(repoTags) == 1 && repoTags[0] == ref {
			matched = tarFile

			break
		}
	}

	if matched == "" {
		return "", fmt.Errorf("couldn't find the match between image reference and available compressed images: %q", errs)
	}

	return matched, nil
}

// extractImageManifestFromTar extract the manifest file containing the image RepoTag inside the archive file.
func extractImageManifestFromTar(tarFile string) ([]byte, error) {
	file, err := os.Open(tarFile)
	if err != nil {
		return nil, err
	}
	//nolint:errcheck
	defer file.Close()

	manifest := "manifest.json"

	tr := tar.NewReader(file)

	for {
		hdr, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				break
			}

			return nil, err
		}

		hdrPath := safepath.CleanPath(hdr.Name)
		if hdrPath == "" {
			return nil, fmt.Errorf("empty tar header path")
		}

		if hdrPath == manifest {
			if hdr.Typeflag == tar.TypeDir || hdr.Typeflag == tar.TypeSymlink {
				return nil, fmt.Errorf("%s is not a file", manifest)
			}

			return ioutil.ReadAll(tr)
		}
	}

	return nil, fmt.Errorf("couldn't find file %s in the archive, wrong image format", manifest)
}

func extractImageName(path string) ([]string, error) {
	data, err := extractImageManifestFromTar(path)
	if err != nil {
		return nil, fmt.Errorf("failed to extract the image manifest file from %s %q", path, err)
	}

	var content []map[string]interface{}

	err = json.Unmarshal(data, &content)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest file, %v", err)
	}

	repoTags := []string{}

	// Image tar file can combine multiple images.
	for _, c := range content {
		if repos, ok := c["RepoTags"].([]interface{}); ok {
			for _, r := range repos {
				if str, ok := r.(string); ok {
					repoTags = append(repoTags, str)
				}
			}
		}
	}

	return repoTags, nil
}

func importImagesFromCache(ctx context.Context, client *containerd.Client, images []string, path, namespace string) error {
	fmt.Printf("importing images from cache for %q namespace\n", namespace)

	var errs *multierror.Error

	importer := containerdrunner.NewImporter(
		namespace,
		containerdrunner.WithContainerdAddress(constants.CRIContainerdAddress))

	for _, image := range images {
		imagePath := filepath.Join(path, image)

		repoTags, err := extractImageName(imagePath)
		if err != nil {
			errs = multierror.Append(errs, fmt.Errorf("failed to import %s image %q", image, err))

			continue
		}

		var req *containerdrunner.ImportRequest

		for _, r := range repoTags {
			// Create a request if the image is not pulled or unpacked.
			if _, err = imageIsPulledAndUnpacked(ctx, client, r); err != nil {
				req = &containerdrunner.ImportRequest{
					Path:    imagePath,
					Options: []containerd.ImportOpt{},
				}

				break
			}
		}

		if req != nil {
			err = retry.Exponential(ImportTimeout, retry.WithUnits(ImportRetryInterval), retry.WithJitter(ImportRetryJitter), retry.WithErrorLogging(true)).Retry(func() error {
				err = retry.ExpectedError(importer.Import(ctx, req))

				if err != nil {
					if os.IsNotExist(err) {
						return err
					}
				}

				return retry.ExpectedError(err)
			})

			if err != nil {
				errs = multierror.Append(errs, err)
			}
		}
	}

	if errs != nil {
		return errs
	}

	return nil
}

func existsAndIsDirectory(path string) error {
	dir, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("directory %q does not exists", path)
		}

		return err
	}

	if !dir.IsDir() {
		return fmt.Errorf("path %q is not a directory", path)
	}

	return nil
}
