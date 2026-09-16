// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/containers"
)

// StatusController aggregates the per-stage statuses into the user-facing ContainerStatus.
type StatusController struct{}

// Name implements controller.Controller interface.
func (ctrl *StatusController) Name() string {
	return "containers.StatusController"
}

// Inputs implements controller.Controller interface.
func (ctrl *StatusController) Inputs() []controller.Input {
	return []controller.Input{
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
			Type:      containers.ContainerDependencyStatusType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: containers.NamespaceName,
			Type:      containers.ContainerInstanceStatusType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: containers.NamespaceName,
			Type:      containers.ContainerInstanceSpecType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *StatusController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: containers.ContainerStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *StatusController) Run(ctx context.Context, runtime controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-runtime.EventCh():
		}

		if err := ctrl.reconcile(ctx, runtime, logger); err != nil {
			logger.Error("failed to aggregate container statuses", zap.Error(err))

			return err
		}

		runtime.ResetRestartBackoff()
	}
}

// reconcile calculates and writes up-to-date ContainerStatus resources for all containers.
func (ctrl *StatusController) reconcile(ctx context.Context, runtime controller.Runtime, logger *zap.Logger) error {
	runtime.StartTrackingOutputs()

	containerSpecs, err := safe.ReaderListAll[*containers.ContainerSpec](ctx, runtime)
	if err != nil {
		return fmt.Errorf("failed to list container specs: %w", err)
	}

	for containerSpec := range containerSpecs.All() {
		if err := ctrl.reconcileContainerStatus(ctx, runtime, logger, containerSpec); err != nil {
			return err
		}
	}

	if err := safe.CleanupOutputs[*containers.ContainerStatus](ctx, runtime); err != nil {
		return fmt.Errorf("failed to clean up outputs: %w", err)
	}

	return nil
}

func (ctrl *StatusController) reconcileContainerStatus(
	ctx context.Context,
	runtime controller.Runtime,
	logger *zap.Logger,
	containerSpec *containers.ContainerSpec,
) error {
	containerSpecID := containerSpec.Metadata().ID()

	containerInstanceStatus, err := containers.LatestInstanceStatus(ctx, runtime, containerSpecID)
	if err != nil {
		return err
	}

	containerImageStatus, err := containerSpec.TypedSpec().CurrentImageStatus(ctx, runtime, containerSpecID)
	if err != nil {
		return err
	}

	containerDependencyStatus, err := containers.CurrentDependencyStatus(ctx, runtime, containerSpecID)
	if err != nil {
		return err
	}

	containerInstanceSpec, err := containers.CurrentInstanceSpec(ctx, runtime, containerSpecID, containerInstanceStatus)
	if err != nil {
		return err
	}

	// The digest which the current reference resolves to, which is not necessarily the one actually running.
	imageDigest := containers.ImageDigest(containerImageStatus, containerSpec.TypedSpec().Image.Ref)

	var before, after containers.ContainerStatusSpec

	if err := safe.WriterModify(ctx, runtime,
		containers.NewContainerStatus(containers.NamespaceName, containerSpecID),
		func(res *containers.ContainerStatus) error {
			before = *res.TypedSpec()

			res.TypedSpec().Update(containerSpec, containerImageStatus, containerInstanceStatus, containerInstanceSpec, imageDigest, containerDependencyStatus)

			after = *res.TypedSpec()

			return nil
		},
	); err != nil {
		return fmt.Errorf("failed to write container status %q: %w", containerSpecID, err)
	}

	// This is the one place with a before-and-after view of the aggregate, so it is where a state
	// change is worth a line in the log rather than in every controller that causes one.
	logContainerStatusTransition(logger, containerSpecID, before, after)

	return nil
}

// logContainerStatusTransition reports a change in the aggregated state or error of one container.
func logContainerStatusTransition(logger *zap.Logger, containerSpecID string, before, after containers.ContainerStatusSpec) {
	if before.State != after.State {
		fields := []zap.Field{
			zap.String("container", containerSpecID),
			zap.Stringer("from", before.State),
			zap.Stringer("to", after.State),
			zap.Stringer("health", after.Health),
		}

		if len(after.WaitingFor) > 0 {
			fields = append(fields, zap.Strings("waitingFor", after.WaitingFor))
		}

		if after.PID != 0 {
			fields = append(fields, zap.Uint32("pid", after.PID))
		}

		if after.Error != "" {
			fields = append(fields, zap.String("error", after.Error))
		}

		logger.Info("container state changed", fields...)

		return
	}

	// A container that stays in the same state but picks up a new error has still made news.
	if before.Error != after.Error && after.Error != "" {
		logger.Warn("container reported an error",
			zap.String("container", containerSpecID),
			zap.Stringer("state", after.State),
			zap.String("error", after.Error),
		)
	}
}
