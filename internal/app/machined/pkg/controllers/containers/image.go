// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v7"
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

// pullRetryMaxInterval caps the backoff between pull attempts.
const pullRetryMaxInterval = time.Minute

// Puller pulls an image into the taloscontainers namespace.
//
// This is the seam that keeps the controller testable: the default implementation talks to
// containerd, tests substitute a fake.
type Puller interface {
	// Pull fetches the reference and returns what the controller needs to know about the image.
	Pull(ctx context.Context, logger *zap.Logger, imageRef string) (PullResult, error)
	// Close releases the underlying client.
	Close() error
}

// PullResult describes a pulled image.
type PullResult struct {
	// Digest is the resolved digest reference.
	Digest string
	// User is the image config's USER, verbatim.
	User string
}

// ImageController owns pulling images for declared containers.
//
// The pull is the one side effect this controller owns. Each attempt runs in a goroutine per
// container, so a slow pull doesn't stall the others; the controller retries failed attempts with
// backoff. Unlike the runtime controller's goroutine this is bounded work rather than process
// supervision.
type ImageController struct {
	// State provides access to the COSI state, needed to resolve registry configuration.
	State state.State

	// PullerProvider is overridable for testing.
	PullerProvider func() (Puller, error)

	// pullTimeout and retryInterval set the retry policy; overridable for testing.
	pullTimeout, retryInterval time.Duration

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

// imagePullStatus is the state of a pull.
//
// The transitions are:
//
//	pending      -> pulling       step, on the controller goroutine
//	pulling      -> succeeded     the attempt returned an image
//	pulling      -> waitingRetry  the attempt failed, and is worth retrying
//	pulling      -> failed        the attempt failed, and is not retried
//	waitingRetry -> pending       the backoff has passed
//
// Succeeded and failed are final: the pull stays that way until its reference changes or its
// container is removed, at which point the pull is stopped and dropped.
type imagePullStatus uint8

const (
	// An attempt is due, and starts on the next step.
	imagePullStatusPending imagePullStatus = iota
	// An attempt is running.
	imagePullStatusPulling
	// The last attempt failed, and the backoff timer is running.
	imagePullStatusWaitingRetry
	// The image has been pulled successfully.
	imagePullStatusSucceeded
	// The pull has failed for good.
	imagePullStatusFailed
)

// pullState tracks one image ref pull, perhaps across multiple attempts.
type pullState struct {
	imageRef        string
	containerSpecID string
	deadline        time.Time

	puller   Puller
	logger   *zap.Logger
	notifyCh chan struct{}

	ctx    context.Context //nolint:containedctx
	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.Mutex

	status imagePullStatus
	result PullResult
	// err is the last attempt's error, kept while the pull is retried.
	err error

	// retryBackoff yields the wait before the next attempt; not thread-safe, used with the state locked.
	retryBackoff *backoff.ExponentialBackOff
	retryTimer   *time.Timer
	notFound     uint
}

// step starts an attempt if one is due, and returns the state of the pull.
//
// It is called from the controller goroutine only, so that an attempt is never started while stop
// waits for the running one.
func (state *pullState) step() (status imagePullStatus, result PullResult, err error) {
	state.mu.Lock()
	defer state.mu.Unlock()

	if state.status == imagePullStatusPending {
		state.status = imagePullStatusPulling
		state.wg.Go(state.run)
	}

	return state.status, state.result, state.err
}

// run performs one pull attempt, and moves the pull on to its next state.
func (state *pullState) run() {
	// Runs after the state is unlocked.
	defer state.notify()

	logger := state.logger.With(zap.String("container", state.containerSpecID), zap.String("image", state.imageRef))

	logger.Info("pulling container image")

	ctx, cancel := context.WithDeadline(state.ctx, state.deadline)
	defer cancel()

	var result PullResult

	// A panic in a pull must not take down machined.
	err := panicsafe.RunErr(func() error {
		var pullErr error

		result, pullErr = state.puller.Pull(ctx, state.logger, state.imageRef)

		return pullErr
	})

	state.mu.Lock()
	defer state.mu.Unlock()

	state.result, state.err = result, err
	if err == nil {
		state.status = imagePullStatusSucceeded

		logger.Info("image pulled", zap.String("digest", result.Digest), zap.String("user", result.User))

		return
	}

	retryAfter, retry := state.nextRetry(err)
	if !retry {
		state.status = imagePullStatusFailed

		if panicsafe.IsPanic(err) {
			logger.Error("image pull panicked", zap.Error(err))
		} else {
			logger.Error("image pull failed", zap.Error(err))
		}

		return
	}

	state.status = imagePullStatusWaitingRetry
	state.retryTimer = time.AfterFunc(retryAfter, func() {
		state.mu.Lock()

		if state.status == imagePullStatusWaitingRetry {
			state.status = imagePullStatusPending
		}

		state.mu.Unlock()

		state.notify()
	})

	logger.Warn("image pull failed, retrying", zap.Duration("retryAfter", retryAfter), zap.Error(err))
}

// nextRetry tells whether a failed attempt is retried, and how long to wait before the next one.
//
// It is called with the state locked.
func (state *pullState) nextRetry(err error) (retryAfter time.Duration, retry bool) {
	// Not retried: an abandoned pull, a panic, or a terminal error.
	if state.ctx.Err() != nil || panicsafe.IsPanic(err) || image.IsTerminal(err) {
		return 0, false
	}

	if image.IsNotFound(err) {
		state.notFound++

		if state.notFound > image.DefaultPullOptions().MaxNotFoundRetries {
			return 0, false
		}
	}

	retryAfter = state.retryBackoff.NextBackOff()

	// Also covers an attempt cut short by the deadline.
	if time.Now().Add(retryAfter).After(state.deadline) {
		return 0, false
	}

	return retryAfter, true
}

// notify wakes the controller.
func (state *pullState) notify() {
	channel.SendWithContext(state.ctx, state.notifyCh, struct{}{})
}

// stop cancels the pull and waits for a running attempt to return.
func (state *pullState) stop() {
	state.cancel()
	state.wg.Wait()

	state.mu.Lock()
	defer state.mu.Unlock()

	if state.retryTimer != nil {
		state.retryTimer.Stop()
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo,cyclop
func (ctrl *ImageController) Run(ctx context.Context, runtime controller.Runtime, logger *zap.Logger) error {
	if ctrl.PullerProvider == nil {
		ctrl.PullerProvider = ctrl.defaultPullerProvider
	}

	if ctrl.pullTimeout == 0 {
		ctrl.pullTimeout = image.PullTimeout
	}

	if ctrl.retryInterval == 0 {
		ctrl.retryInterval = image.PullRetryInterval
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
		// Cancel all first, so they wind down in parallel.
		for _, pull := range ctrl.pulls {
			pull.cancel()
			defer pull.stop()
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

func (ctrl *ImageController) criIsUp(ctx context.Context, reader controller.Reader) (bool, error) {
	criService, err := safe.ReaderGetByID[*v1alpha1.Service](ctx, reader, criServiceID)
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

		if err := ctrl.reconcileContainer(
			ctx, runtime, logger, puller, notifyCh, containerSpecID, containerSpec.TypedSpec().Image.Ref, containerSpec.TypedSpec().RunAs,
		); err != nil {
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
	runAs containers.ContainerRunAsSpec,
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

		pull = ctrl.instantiatePull(ctx, logger, puller, containerSpecID, imageRef, notifyCh)
		ctrl.pulls[containerSpecID] = pull
	}

	status, result, pullErr := pull.step()

	// The last attempt's error: reported while retrying, and once the pull has failed.
	errText := ""

	if pullErr != nil {
		errText = pullErr.Error()
	}

	switch status {
	case imagePullStatusPending, imagePullStatusPulling, imagePullStatusWaitingRetry:
		logger.Debug(
			"image pull in progress",
			zap.String("container", containerSpecID),
			zap.String("image", imageRef),
		)

		return ctrl.writeStatus(ctx, runtime, containerSpecID, imageRef, containers.ContainerImagePhasePulling, "", errText)
	case imagePullStatusFailed:
		return ctrl.writeStatus(ctx, runtime, containerSpecID, imageRef, containers.ContainerImagePhaseFailed, "", errText)
	case imagePullStatusSucceeded:
		fallthrough
	default:
		// The image is on the node, but a USER it names rather than numbers would otherwise run as
		// root. It is rejected here, before an instance exists, and re-checked on every reconcile so
		// that setting runAs takes effect without a re-pull. The digest stays empty to keep the
		// instance gate shut.
		if err := CheckImageUser(result.User, runAs); err != nil {
			return ctrl.writeStatus(ctx, runtime, containerSpecID, imageRef, containers.ContainerImagePhaseFailed, "", err.Error())
		}

		return ctrl.writeStatus(ctx, runtime, containerSpecID, imageRef, containers.ContainerImagePhaseReady, result.Digest, "")
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

// instantiatePull creates a new pull state machine.
func (ctrl *ImageController) instantiatePull(
	ctx context.Context,
	logger *zap.Logger,
	puller Puller,
	containerSpecID, imageRef string,
	notifyCh chan struct{},
) *pullState {
	pullCtx, cancel := context.WithCancel(ctx)

	// Exponential, with jitter, capped so that a long outage is still probed regularly.
	retryBackoff := backoff.NewExponentialBackOff()
	retryBackoff.InitialInterval = ctrl.retryInterval
	retryBackoff.Multiplier = 2
	retryBackoff.MaxInterval = max(pullRetryMaxInterval, ctrl.retryInterval)
	retryBackoff.Reset()

	pull := &pullState{
		imageRef:        imageRef,
		containerSpecID: containerSpecID,
		deadline:        time.Now().Add(ctrl.pullTimeout),
		retryBackoff:    retryBackoff,
		puller:          puller,
		logger:          logger,
		notifyCh:        notifyCh,
		ctx:             pullCtx,
		cancel:          cancel,
		err:             nil,
		status:          imagePullStatusPending,
	}

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

func (p *containerdPuller) Pull(ctx context.Context, logger *zap.Logger, imageRef string) (PullResult, error) {
	// The taloscontainers namespace keeps these images away from both Kubernetes pods and Talos'
	// own system images.
	ctx = namespaces.WithNamespace(ctx, constants.TalosContainersContainerdNamespace)

	// A single attempt: the controller handles retries.
	logWriter := &zapio.Writer{Log: logger, Level: zapcore.DebugLevel}

	img, err := image.Pull(
		ctx, p.registryBuilder, p.state, p.client, imageRef,
		// IfNotPresent semantics: an image already on the node is not re-fetched, which also keeps a
		// crash-looping container from hammering the registry on every restart.
		image.WithSkipIfAlreadyPulled(),
		image.WithLogWriter(logWriter),
	)
	if err != nil {
		return PullResult{}, err
	}

	config, err := ImageConfig(ctx, img)
	if err != nil {
		return PullResult{}, fmt.Errorf("failed to read the image config: %w", err)
	}

	return PullResult{
		Digest: img.Target().Digest.String(),
		User:   config.User,
	}, nil
}

func (p *containerdPuller) Close() error {
	return p.client.Close()
}
