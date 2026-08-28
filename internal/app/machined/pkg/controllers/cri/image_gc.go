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
	"github.com/opencontainers/go-digest"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/containers"
	"github.com/siderolabs/talos/pkg/machinery/resources/etcd"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

// DefaultImageCleanupInterval is the default interval at which the image GC controller runs.
const DefaultImageCleanupInterval = 15 * time.Minute

// DefaultImageGCGracePeriod is the default minimum age of an image before it can be deleted.
const DefaultImageGCGracePeriod = 4 * DefaultImageCleanupInterval

// RefsToRetainFunc returns the image references which must be preserved.
//
// It may return an empty set before its underlying resources have synced (e.g. right after boot).
// That is safe: cleanup only deletes an image once it has looked unreferenced continuously for a full
// GCGracePeriod, tracked per-image from the first time the controller ever observed it as such (see
// imageFirstSeenUnreferenced in cleanup below) — which is reliably longer than resources take to sync.
type RefsToRetainFunc func(ctx context.Context, reader controller.Reader) ([]string, error)

// NewImageGCController creates a new ImageGCController.
//
// containerdName selects the containerd instance (and the v1alpha1.Service gating on it), and
// namespace is the containerd namespace to collect. A nil refsToRetain means nothing is retained,
// i.e. every image in the namespace is eligible for cleanup.
func NewImageGCController(containerdName, namespace string, refsToRetainFunc RefsToRetainFunc) *ImageGCController {
	controllerName := fmt.Sprintf("%s.%s.ImageGCController", containerdName, namespace)

	return &ImageGCController{
		containerdName:      containerdName,
		containerdNamespace: namespace,
		controllerName:      controllerName,
		refsToRetain:        refsToRetainFunc,
	}
}

// ImageGCController performs garbage collection of unused container images.
type ImageGCController struct {
	ImageServiceProvider func() (ImageServiceProvider, error)

	// CleanupInterval and GCGracePeriod default to DefaultImageCleanupInterval and DefaultImageGCGracePeriod.
	//
	// They are fields rather than constants so that tests can run a cleanup cycle without waiting
	// out the production timing; nothing in Talos overrides them.
	CleanupInterval time.Duration
	GCGracePeriod   time.Duration

	containerdName      string
	containerdNamespace string
	// refsToRetain computes the images to preserve; nil retains nothing.
	refsToRetain RefsToRetainFunc

	controllerName             string
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
//
// Only the containerd service is an input: it is the one thing the controller has to wake for.
// Everything else is read on the cleanup tick, see RefsToRetainFunc.
func (ctrl *ImageGCController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: v1alpha1.NamespaceName,
			Type:      v1alpha1.ServiceType,
			ID:        optional.Some(ctrl.containerdName),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: containers.NamespaceName,
			Type:      containers.ContainerSpecType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: containers.NamespaceName,
			Type:      containers.ContainerImageStatusType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: containers.NamespaceName,
			Type:      containers.ContainerInstanceSpecType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: k8s.NamespaceName,
			Type:      k8s.KubeletSpecType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: etcd.NamespaceName,
			Type:      etcd.SpecType,
			Kind:      controller.InputWeak,
		},
	}
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
func (ctrl *ImageGCController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	ctrl.ensureDefaults()

	var (
		containerdIsUp       bool
		imageServiceProvider ImageServiceProvider
	)

	ticker := time.NewTicker(ctrl.CleanupInterval)
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
			if !containerdIsUp {
				continue
			}

			var err error

			imageServiceProvider, err = ctrl.runCleanup(ctx, logger, r, imageServiceProvider)
			if err != nil {
				return err
			}
		case <-r.EventCh():
			var err error

			containerdIsUp, err = ctrl.updateContainerdStatus(ctx, r)
			if err != nil {
				return err
			}
		}

		r.ResetRestartBackoff()
	}
}

// ensureDefaults fills in zero-value fields with their defaults.
func (ctrl *ImageGCController) ensureDefaults() {
	if ctrl.ImageServiceProvider == nil {
		ctrl.ImageServiceProvider = defaultImageServiceProvider(ctrl.containerdName)
	}

	if ctrl.imageFirstSeenUnreferenced == nil {
		ctrl.imageFirstSeenUnreferenced = map[string]time.Time{}
	}

	if ctrl.CleanupInterval == 0 {
		ctrl.CleanupInterval = DefaultImageCleanupInterval
	}

	if ctrl.GCGracePeriod == 0 {
		ctrl.GCGracePeriod = DefaultImageGCGracePeriod
	}
}

// runCleanup runs a single cleanup cycle, lazily creating imageServiceProvider if it isn't set yet.
//
// It returns imageServiceProvider back (created or unchanged) so the caller can keep closing it on exit.
func (ctrl *ImageGCController) runCleanup(ctx context.Context, logger *zap.Logger, reader controller.Reader, imageServiceProvider ImageServiceProvider) (ImageServiceProvider, error) {
	retainRefs, err := ctrl.safeRefsToRetain(ctx, reader)
	if err != nil {
		return imageServiceProvider, fmt.Errorf("error computing images to retain: %w", err)
	}

	if imageServiceProvider == nil {
		imageServiceProvider, err = ctrl.ImageServiceProvider()
		if err != nil {
			return nil, fmt.Errorf("error creating image service provider: %w", err)
		}
	}

	if err := ctrl.cleanup(ctx, logger, imageServiceProvider.ImageService(), retainRefs); err != nil {
		return imageServiceProvider, fmt.Errorf("error running image cleanup: %w", err)
	}

	return imageServiceProvider, nil
}

// updateContainerdStatus reports whether the watched containerd service is running and healthy.
func (ctrl *ImageGCController) updateContainerdStatus(ctx context.Context, r controller.Runtime) (bool, error) {
	containerdService, err := safe.ReaderGet[*v1alpha1.Service](ctx, r, resource.NewMetadata(v1alpha1.NamespaceName, v1alpha1.ServiceType, ctrl.containerdName, resource.VersionUndefined))
	if err != nil && !state.IsNotFoundError(err) {
		return false, fmt.Errorf("error getting container service: %w", err)
	}

	return containerdService != nil && containerdService.TypedSpec().Running && containerdService.TypedSpec().Healthy, nil
}

// safeRefsToRetain calls RefsToRetain, treating a nil one as retaining nothing.
func (ctrl *ImageGCController) safeRefsToRetain(ctx context.Context, reader controller.Reader) ([]string, error) {
	if ctrl.refsToRetain == nil {
		return nil, nil
	}

	return ctrl.refsToRetain(ctx, reader)
}

// buildExpectedDigests resolves the expected image references to the digests they name.
//
// An expectation is either a bare digest, which resolves to itself, or a reference, which either
// carries its digest or has to be matched by name and tag against the images actually stored.
//
//nolint:gocyclo
func buildExpectedDigests(logger *zap.Logger, actualImages []images.Image, expectedImages []string) (map[string]struct{}, error) {
	var (
		parseErrors        error
		expectedReferences []reference.Named
	)

	expectedDigests := map[string]struct{}{}

	for _, ref := range expectedImages {
		// Bare digest, as ContainerImageStatus and ContainerInstanceSpec carry it: nothing to
		// resolve.
		if dgst, err := digest.Parse(ref); err == nil {
			expectedDigests[dgst.String()] = struct{}{}

			continue
		}

		// ParseDockerRef rather than ParseNamed, because it is what the pull path normalizes with
		// (see internal/pkg/containers/image.Pull), and the containerd image record is named after
		// the result.
		parsed, parseErr := reference.ParseDockerRef(ref)
		if parseErr != nil {
			parseErrors = errors.Join(parseErrors, parseErr)

			continue
		}

		expectedReferences = append(expectedReferences, parsed)
	}

	if parseErrors != nil {
		return nil, fmt.Errorf("error parsing expected images: %w", parseErrors)
	}

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

			imageDigest := image.Target.Digest.String()

			if ref, ok := imageRef.(reference.NamedTagged); ok {
				if expectedRef.Name() != ref.Name() {
					continue
				}

				if expectedTagged, ok := expectedRef.(reference.Tagged); ok && ref.Tag() == expectedTagged.Tag() {
					// this is expected image by tag, inject digest
					expectedDigests[imageDigest] = struct{}{}

					break
				}
			}
		}
	}

	return expectedDigests, nil
}

func (ctrl *ImageGCController) cleanup(ctx context.Context, logger *zap.Logger, imageService images.Store, expectedImages []string) error {
	logger.Debug("running image cleanup")

	ctx = namespaces.WithNamespace(ctx, ctrl.containerdNamespace)

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
		//  * as we see it, this means the image won't be deleted until it reaches the age of GCGracePeriod from the moment it became unreferenced
		imageAgeCRI := time.Since(image.CreatedAt)
		imageAgeInternal := time.Since(ctrl.imageFirstSeenUnreferenced[image.Name])

		imageAge := min(imageAgeCRI, imageAgeInternal)

		if imageAge < ctrl.GCGracePeriod {
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
