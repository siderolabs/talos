// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cri

import (
	"context"
	"errors"
	"fmt"
	"time"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/distribution/reference"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xslices"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/etcd"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

// ImageCleanupInterval is the interval at which the image GC controller runs.
const ImageCleanupInterval = 15 * time.Minute

// ImageGCGracePeriod is the minimum age of an image before it can be deleted.
const ImageGCGracePeriod = 4 * ImageCleanupInterval

// NewImageGCController creates a new ImageGCController.
func NewImageGCController(containerdName string, buildExpectedImages bool) *ImageGCController {
	controllerName := "cri." + containerdName + "ImageGCController"

	return &ImageGCController{
		containerdName:      containerdName,
		controllerName:      controllerName,
		buildExpectedImages: buildExpectedImages,
	}
}

// ImageGCController performs garbage collection of unused container images.
type ImageGCController struct {
	ImageServiceProvider func() (ImageServiceProvider, error)

	containerdName             string
	controllerName             string
	buildExpectedImages        bool
	imageFirstSeenUnreferenced map[string]time.Time
}

// ImageServiceProvider wraps the containerd image service.
type ImageServiceProvider interface {
	ImageService() images.Store
	Close() error
}

// Name implements controller.Controller interface.
func (ctrl *ImageGCController) Name() string {
	return ctrl.controllerName
}

// Inputs implements controller.Controller interface.
func (ctrl *ImageGCController) Inputs() []controller.Input {
	inputs := []controller.Input{
		{
			Namespace: v1alpha1.NamespaceName,
			Type:      v1alpha1.ServiceType,
			ID:        optional.Some(ctrl.containerdName),
			Kind:      controller.InputWeak,
		},
	}

	if ctrl.buildExpectedImages {
		inputs = append(inputs,
			controller.Input{
				Namespace: k8s.NamespaceName,
				Type:      k8s.KubeletSpecType,
				ID:        optional.Some(k8s.KubeletID),
				Kind:      controller.InputWeak,
			},
			controller.Input{
				Namespace: etcd.NamespaceName,
				Type:      etcd.SpecType,
				ID:        optional.Some(etcd.SpecID),
				Kind:      controller.InputWeak,
			},
		)
	}

	return inputs
}

// Outputs implements controller.Controller interface.
func (ctrl *ImageGCController) Outputs() []controller.Output {
	return nil
}

func defaultImageServiceProvider(containerdName string) func() (ImageServiceProvider, error) {
	return func() (ImageServiceProvider, error) {
		var addr string

		switch containerdName {
		case "cri":
			addr = constants.CRIContainerdAddress
		case "containerd":
			addr = constants.SystemContainerdAddress
		default:
			return nil, fmt.Errorf("unknown containerd name: %s", containerdName)
		}

		criClient, err := containerd.New(addr)
		if err != nil {
			return nil, fmt.Errorf("error creating containerd client: %w", err)
		}

		return &containerdImageServiceProvider{
			criClient: criClient,
		}, nil
	}
}

type containerdImageServiceProvider struct {
	criClient *containerd.Client
}

func (s *containerdImageServiceProvider) ImageService() images.Store {
	return s.criClient.ImageService()
}

func (s *containerdImageServiceProvider) Close() error {
	return s.criClient.Close()
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo,cyclop
func (ctrl *ImageGCController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	if ctrl.ImageServiceProvider == nil {
		ctrl.ImageServiceProvider = defaultImageServiceProvider(ctrl.containerdName)
	}

	if ctrl.imageFirstSeenUnreferenced == nil {
		ctrl.imageFirstSeenUnreferenced = map[string]time.Time{}
	}

	var (
		containerdIsUp       bool
		expectedImages       []string
		imageServiceProvider ImageServiceProvider
	)

	ticker := time.NewTicker(ImageCleanupInterval)
	defer ticker.Stop()

	defer func() {
		if imageServiceProvider != nil {
			imageServiceProvider.Close() //nolint:errcheck
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if !containerdIsUp || (ctrl.buildExpectedImages && len(expectedImages) == 0) {
				continue
			}

			if imageServiceProvider == nil {
				var err error

				imageServiceProvider, err = ctrl.ImageServiceProvider()
				if err != nil {
					return fmt.Errorf("error creating image service provider: %w", err)
				}
			}

			if err := ctrl.cleanup(ctx, logger, imageServiceProvider.ImageService(), expectedImages); err != nil {
				return fmt.Errorf("error running image cleanup: %w", err)
			}
		case <-r.EventCh():
			containerdService, err := safe.ReaderGet[*v1alpha1.Service](ctx, r, resource.NewMetadata(v1alpha1.NamespaceName, v1alpha1.ServiceType, ctrl.containerdName, resource.VersionUndefined))
			if err != nil && !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting container service: %w", err)
			}

			containerdIsUp = containerdService != nil && containerdService.TypedSpec().Running && containerdService.TypedSpec().Healthy

			expectedImages = nil

			if ctrl.buildExpectedImages {
				etcdSpec, err := safe.ReaderGet[*etcd.Spec](ctx, r, resource.NewMetadata(etcd.NamespaceName, etcd.SpecType, etcd.SpecID, resource.VersionUndefined))
				if err != nil && !state.IsNotFoundError(err) {
					return fmt.Errorf("error getting etcd spec: %w", err)
				}

				if etcdSpec != nil {
					expectedImages = append(expectedImages, etcdSpec.TypedSpec().Image)
				}

				kubeletSpec, err := safe.ReaderGet[*k8s.KubeletSpec](ctx, r, resource.NewMetadata(k8s.NamespaceName, k8s.KubeletSpecType, k8s.KubeletID, resource.VersionUndefined))
				if err != nil && !state.IsNotFoundError(err) {
					return fmt.Errorf("error getting kubelet spec: %w", err)
				}

				if kubeletSpec != nil {
					expectedImages = append(expectedImages, kubeletSpec.TypedSpec().Image)
				}
			}
		}

		r.ResetRestartBackoff()
	}
}

//nolint:gocyclo
func buildExpectedDigests(logger *zap.Logger, actualImages []images.Image, expectedImages []string) (map[string]struct{}, error) {
	var parseErrors error

	expectedReferences := xslices.Map(expectedImages, func(ref string) reference.Named {
		res, parseErr := reference.ParseNamed(ref)

		parseErrors = errors.Join(parseErrors, parseErr)

		return res
	})

	if parseErrors != nil {
		return nil, fmt.Errorf("error parsing expected images: %w", parseErrors)
	}

	expectedDigests := map[string]struct{}{}

	for _, expectedRef := range expectedReferences {
		// easy case: image ref has digest, record it
		if expectedDigested, ok := expectedRef.(reference.Digested); ok {
			expectedDigests[expectedDigested.Digest().String()] = struct{}{}

			continue
		}

		// hard case: iterate over actual images to find the digest for the tag
		for _, image := range actualImages {
			imageRef, err := reference.ParseAnyReference(image.Name)
			if err != nil {
				logger.Debug("failed to parse image reference", zap.Error(err), zap.String("image", image.Name))

				continue
			}

			digest := image.Target.Digest.String()

			if ref, ok := imageRef.(reference.NamedTagged); ok {
				if expectedRef.Name() != ref.Name() {
					continue
				}

				if expectedTagged, ok := expectedRef.(reference.Tagged); ok && ref.Tag() == expectedTagged.Tag() {
					// this is expected image by tag, inject digest
					expectedDigests[digest] = struct{}{}

					break
				}
			}
		}
	}

	return expectedDigests, nil
}

func (ctrl *ImageGCController) cleanup(ctx context.Context, logger *zap.Logger, imageService images.Store, expectedImages []string) error {
	logger.Debug("running image cleanup")

	ctx = namespaces.WithNamespace(ctx, constants.SystemContainerdNamespace)

	actualImages, err := imageService.List(ctx)
	if err != nil {
		return fmt.Errorf("error listing images: %w", err)
	}

	// first pass: scan actualImages and expand expectedImages from tags to digests
	expectedDigests, err := buildExpectedDigests(logger, actualImages, expectedImages)
	if err != nil {
		return err
	}

	// second pass, drop whatever is not expected
	for _, image := range actualImages {
		_, shouldKeep := expectedDigests[image.Target.Digest.String()]

		if shouldKeep {
			logger.Debug("image is referenced, skipping garbage collection", zap.String("image", image.Name))

			delete(ctrl.imageFirstSeenUnreferenced, image.Name)

			continue
		}

		if _, ok := ctrl.imageFirstSeenUnreferenced[image.Name]; !ok {
			ctrl.imageFirstSeenUnreferenced[image.Name] = time.Now()
		}

		// calculate image age two ways, and pick the minimum:
		//  * as CRI reports it, which is the time image got pulled
		//  * as we see it, this means the image won't be deleted until it reaches the age of ImageGCGracePeriod from the moment it became unreferenced
		imageAgeCRI := time.Since(image.CreatedAt)
		imageAgeInternal := time.Since(ctrl.imageFirstSeenUnreferenced[image.Name])

		imageAge := min(imageAgeCRI, imageAgeInternal)

		if imageAge < ImageGCGracePeriod {
			logger.Debug("skipping image cleanup, as it's below minimum age", zap.String("image", image.Name), zap.Duration("age", imageAge))

			continue
		}

		if err = imageService.Delete(ctx, image.Name); err != nil {
			return fmt.Errorf("failed to delete an image %s: %w", image.Name, err)
		}

		delete(ctrl.imageFirstSeenUnreferenced, image.Name)
		logger.Info("deleted an image", zap.String("image", image.Name))
	}

	return nil
}
