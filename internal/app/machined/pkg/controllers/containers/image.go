// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers

import (
	"context"
	"fmt"
	"sync"

	containerdapi "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/channel"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/panicsafe"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zapio"

	"github.com/siderolabs/talos/internal/pkg/containers/image"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/containers"
	"github.com/siderolabs/talos/pkg/machinery/resources/cri"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

// criServiceID is the Talos service running the CRI containerd instance.
const criServiceID = "cri"

// Puller pulls an image into the taloscontainers namespace.
//
// This is the seam that keeps the controller testable: the default implementation talks to
// containerd, tests substitute a fake.
type Puller interface {
	// Pull fetches the reference and returns the resolved digest reference.
	Pull(ctx context.Context, logger *zap.Logger, imageRef string) (string, error)
	// Close releases the underlying client.
	Close() error
}

// ImageController owns pulling images for declared containers.
//
// The pull is the one side effect this controller owns. It runs in a goroutine per container
// because a pull retries with backoff for up to image.PullTimeout (20 minutes); doing it inline
// would stall every other container and stop the controller reacting to events. Unlike the runtime
// controller's goroutine this is bounded work rather than process supervision.
type ImageController struct {
	// State provides access to the COSI state, needed to resolve registry configuration.
	State state.State

	// PullerProvider is overridable for testing.
	PullerProvider func() (Puller, error)

	// pulls tracks the in-flight pulls, keyed by container ID.
	pulls map[string]*pullState
}

// Name implements controller.Controller interface.
func (ctrl *ImageController) Name() string {
	return "containers.ImageController"
}

// Inputs implements controller.Controller interface.
func (ctrl *ImageController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: containers.NamespaceName,
			Type:      containers.ContainerSpecType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: v1alpha1.NamespaceName,
			Type:      v1alpha1.ServiceType,
			ID:        optional.Some(criServiceID),
			Kind:      controller.InputWeak,
		},
		// The registry and image cache configuration are not read directly: they are declared so
		// that a mirror or cache appearing mid-flight re-runs the controller.
		{
			Namespace: cri.NamespaceName,
			Type:      cri.RegistriesConfigType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: cri.NamespaceName,
			Type:      cri.ImageCacheConfigType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *ImageController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: containers.ContainerImageStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// pullState tracks one in-flight pull.
type pullState struct {
	imageRef string

	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu          sync.Mutex
	imageDigest string
	err         error
	done        bool
}

func (state *pullState) snapshot() (imageDigest string, err error, done bool) {
	state.mu.Lock()
	defer state.mu.Unlock()

	return state.imageDigest, state.err, state.done
}

func (state *pullState) finish(imageDigest string, err error) {
	state.mu.Lock()
	defer state.mu.Unlock()

	state.imageDigest, state.err, state.done = imageDigest, err, true
}

func (state *pullState) stop() {
	state.cancel()
	state.wg.Wait()
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo,cyclop
func (ctrl *ImageController) Run(ctx context.Context, runtime controller.Runtime, logger *zap.Logger) error {
	if ctrl.PullerProvider == nil {
		ctrl.PullerProvider = ctrl.defaultPullerProvider
	}

	ctrl.pulls = map[string]*pullState{}

	notifyCh := make(chan struct{}, 1)

	var puller Puller

	// Registered before the pull-stopping defer so that it runs after it: the pulls use the
	// puller's client, so they have to be joined before it is closed.
	defer func() {
		if puller != nil {
			puller.Close() //nolint:errcheck
		}
	}()

	defer func() {
		for _, pull := range ctrl.pulls {
			pull.cancel()
			defer pull.wg.Wait()
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-runtime.EventCh():
		case <-notifyCh:
		}

		// Nothing can be pulled until the CRI containerd instance is up, since that is the socket
		// the taloscontainers namespace lives on.
		criUp, err := ctrl.criIsUp(ctx, runtime)
		if err != nil {
			return err
		}

		if criUp && puller == nil {
			if puller, err = ctrl.PullerProvider(); err != nil {
				logger.Error("failed to create image puller", zap.Error(err))

				return fmt.Errorf("failed to create image puller: %w", err)
			}
		}

		if err := ctrl.reconcile(ctx, runtime, logger, puller, notifyCh); err != nil {
			logger.Error("failed to reconcile container images", zap.Error(err))

			return err
		}

		runtime.ResetRestartBackoff()
	}
}

func (ctrl *ImageController) criIsUp(ctx context.Context, runtime controller.Runtime) (bool, error) {
	criService, err := safe.ReaderGetByID[*v1alpha1.Service](ctx, runtime, criServiceID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return false, nil
		}

		return false, fmt.Errorf("failed to get %q service: %w", criServiceID, err)
	}

	return criService.TypedSpec().Running && criService.TypedSpec().Healthy, nil
}

func (ctrl *ImageController) reconcile(
	ctx context.Context,
	runtime controller.Runtime,
	logger *zap.Logger,
	puller Puller,
	notifyCh chan struct{},
) error {
	containerSpecs, err := safe.ReaderListAll[*containers.ContainerSpec](ctx, runtime)
	if err != nil {
		return fmt.Errorf("failed to list container specs: %w", err)
	}

	runtime.StartTrackingOutputs()

	wanted := map[string]struct{}{}

	for containerSpec := range containerSpecs.All() {
		containerSpecID := containerSpec.Metadata().ID()
		wanted[containerSpecID] = struct{}{}

		if err := ctrl.reconcileContainer(ctx, runtime, logger, puller, notifyCh, containerSpecID, containerSpec.TypedSpec().Image.Ref); err != nil {
			return err
		}
	}

	ctrl.pruneAbandoned(logger, wanted)

	return safe.CleanupOutputs[*containers.ContainerImageStatus](ctx, runtime)
}

// reconcileContainer syncs the pull state for a single container and writes its resulting status.
func (ctrl *ImageController) reconcileContainer(
	ctx context.Context,
	runtime controller.Runtime,
	logger *zap.Logger,
	puller Puller,
	notifyCh chan struct{},
	containerSpecID, imageRef string,
) error {
	pull := ctrl.currentPull(logger, containerSpecID, imageRef)

	if pull == nil {
		if puller == nil {
			// Waiting for the CRI service; report pending so the operator can see why.
			logger.Debug(
				"waiting for the container runtime before pulling",
				zap.String("container", containerSpecID),
				zap.String("image", imageRef),
			)

			return ctrl.writeStatus(ctx, runtime, containerSpecID, imageRef, containers.ContainerImagePhasePending, "", "")
		}

		pull = ctrl.startPull(ctx, logger, puller, containerSpecID, imageRef, notifyCh)
		ctrl.pulls[containerSpecID] = pull
	}

	imageDigest, pullErr, done := pull.snapshot()

	switch {
	case !done:
		logger.Debug(
			"image pull in progress",
			zap.String("container", containerSpecID),
			zap.String("image", imageRef),
		)

		return ctrl.writeStatus(ctx, runtime, containerSpecID, imageRef, containers.ContainerImagePhasePulling, "", "")
	case pullErr != nil:
		return ctrl.writeStatus(ctx, runtime, containerSpecID, imageRef, containers.ContainerImagePhaseFailed, "", pullErr.Error())
	default:
		return ctrl.writeStatus(ctx, runtime, containerSpecID, imageRef, containers.ContainerImagePhaseReady, imageDigest, "")
	}
}

// currentPull returns the existing pull for containerSpecID, or nil if there is none.
//
// A changed reference invalidates an in-flight or completed pull: it is stopped and removed, and
// nil is returned so the caller starts a fresh one.
func (ctrl *ImageController) currentPull(logger *zap.Logger, containerSpecID, imageRef string) *pullState {
	pull, exists := ctrl.pulls[containerSpecID]
	if !exists {
		return nil
	}

	if pull.imageRef == imageRef {
		return pull
	}

	logger.Info(
		"container image reference changed, restarting the pull",
		zap.String("container", containerSpecID),
		zap.String("from", pull.imageRef),
		zap.String("to", imageRef),
	)

	pull.stop()
	delete(ctrl.pulls, containerSpecID)

	return nil
}

// pruneAbandoned stops and removes pulls for containers that are no longer wanted.
func (ctrl *ImageController) pruneAbandoned(logger *zap.Logger, wanted map[string]struct{}) {
	for containerSpecID, pull := range ctrl.pulls {
		if _, exists := wanted[containerSpecID]; exists {
			continue
		}

		logger.Info("container is gone, abandoning its image pull", zap.String("container", containerSpecID))

		pull.stop()
		delete(ctrl.pulls, containerSpecID)
	}
}

func (ctrl *ImageController) writeStatus(
	ctx context.Context,
	runtime controller.Runtime,
	containerSpecID string,
	imageRef string,
	phase containers.ContainerImagePhase,
	imageDigest string,
	errText string,
) error {
	if err := safe.WriterModify(
		ctx, runtime,
		containers.NewContainerImageStatus(containers.NamespaceName, containerSpecID),
		func(res *containers.ContainerImageStatus) error {
			res.TypedSpec().Phase = phase
			res.TypedSpec().Image = imageRef
			res.TypedSpec().Digest = imageDigest
			res.TypedSpec().Error = errText

			return nil
		},
	); err != nil {
		return fmt.Errorf("failed to write image status %q: %w", containerSpecID, err)
	}

	return nil
}

// startPull launches the pull for one container.
//
// Transient failures are retried inside the pull itself, for up to image.PullTimeout. Once it does
// give up — or fails terminally, as a denied signature does — the failure is recorded and not
// retried here: the instance controller never opens the gate, the container stays visible as
// failed, and the operator sees the error. A changed reference starts a fresh pull.
func (ctrl *ImageController) startPull(
	ctx context.Context,
	logger *zap.Logger,
	puller Puller,
	containerSpecID, imageRef string,
	notifyCh chan struct{},
) *pullState {
	pull := &pullState{imageRef: imageRef}

	pullCtx, cancel := context.WithCancel(ctx)
	pull.cancel = cancel

	pull.wg.Go(func() {
		defer channel.SendWithContext(pullCtx, notifyCh, struct{}{})

		logger.Info("pulling container image", zap.String("container", containerSpecID), zap.String("image", imageRef))

		var imageDigest string

		// A panic in a pull must not take down machined.
		err := panicsafe.RunErr(func() error {
			var pullErr error

			imageDigest, pullErr = puller.Pull(pullCtx, logger, imageRef)

			return pullErr
		})

		pull.finish(imageDigest, err)

		switch {
		case panicsafe.IsPanic(err):
			logger.Error("image pull panicked", zap.String("container", containerSpecID), zap.Error(err))
		case err != nil:
			logger.Error(
				"image pull failed",
				zap.String("container", containerSpecID),
				zap.String("image", imageRef),
				zap.Error(err),
			)
		default:
			logger.Info(
				"image pulled",
				zap.String("container", containerSpecID),
				zap.String("digest", imageDigest),
			)
		}
	})

	return pull
}

// defaultPullerProvider dials the CRI containerd instance and pulls through the image package.
func (ctrl *ImageController) defaultPullerProvider() (Puller, error) {
	client, err := containerdapi.New(constants.CRIContainerdAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to containerd: %w", err)
	}

	return &containerdPuller{
		client:          client,
		state:           ctrl.State,
		registryBuilder: cri.RegistryBuilder(ctrl.State),
	}, nil
}

type containerdPuller struct {
	client          *containerdapi.Client
	state           state.State
	registryBuilder image.RegistriesBuilder
}

func (p *containerdPuller) Pull(ctx context.Context, logger *zap.Logger, imageRef string) (string, error) {
	// The taloscontainers namespace keeps these images away from both Kubernetes pods and Talos'
	// own system images.
	ctx = namespaces.WithNamespace(ctx, constants.TalosContainersContainerdNamespace)

	// Retries live inside the pull: a registry that is briefly unreachable, or a mirror that comes
	// up mid-retry, resolves without anything upstream having to re-trigger. A denied signature is
	// terminal and comes straight back.
	logWriter := &zapio.Writer{Log: logger, Level: zapcore.DebugLevel}

	ctx, cancel := context.WithTimeout(ctx, image.PullTimeout)
	defer cancel()

	img, err := image.PullWithRetriesAndTimeout(
		ctx, p.registryBuilder, p.state, p.client, imageRef,
		// IfNotPresent semantics: an image already on the node is not re-fetched, which also keeps a
		// crash-looping container from hammering the registry on every restart.
		image.WithSkipIfAlreadyPulled(),
		image.WithLogWriter(logWriter),
	)
	if err != nil {
		return "", err
	}

	return img.Target().Digest.String(), nil
}

func (p *containerdPuller) Close() error {
	return p.client.Close()
}
