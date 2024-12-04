// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/events"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner/process"
	"github.com/siderolabs/talos/internal/pkg/environment"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

type scrubSchedule struct {
	period   time.Duration
	upcoming time.Time
}

// FSScrubController watches v1alpha1.Config and schedules filesystem online check tasks.
type FSScrubController struct {
	Runtime  runtime.Runtime
	schedule map[string]scrubSchedule
}

// Name implements controller.Controller interface.
func (ctrl *FSScrubController) Name() string {
	return "runtime.FSScrubController"
}

// Inputs implements controller.Controller interface.
func (ctrl *FSScrubController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: block.NamespaceName,
			Type:      block.VolumeStatusType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: block.NamespaceName,
			Type:      block.VolumeConfigType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *FSScrubController) Outputs() []controller.Output {
	return []controller.Output{}
}

// Run implements controller.Controller interface.
func (ctrl *FSScrubController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	var (
		ticker  *time.Ticker
		tickerC <-chan time.Time
	)

	tickerStop := func() {
		if ticker == nil {
			return
		}

		ticker.Stop()

		ticker = nil
		tickerC = nil
	}

	defer tickerStop()

	tickerStop()

	ticker = time.NewTicker(15 * time.Second)
	tickerC = ticker.C

	ctrl.schedule = make(map[string]scrubSchedule)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-tickerC:
			if err := ctrl.scrub("/var", []string{}); err != nil {
				return fmt.Errorf("error running filesystem scrub: %w", err)
			}

			continue
		case <-r.EventCh():
			err := ctrl.updateSchedule(ctx, r, logger)
			if err != nil {
				return err
			}
		}
	}
}

func (ctrl *FSScrubController) updateSchedule(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	volumesStatus, err := safe.ReaderListAll[*block.VolumeStatus](ctx, r)
	if err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("error getting volume status: %w", err)
	}

	logger.Warn("reading volume status")
	volumesStatus.ForEach(func(item *block.VolumeStatus) {
		vol := item.TypedSpec()

		logger.Warn("volume status", zap.Reflect("item", vol))

		if vol.Phase != block.VolumePhaseReady {
			logger.Warn("vol.Phase != block.VolumePhaseReady", zap.Reflect("item", vol))

			return
		}

		if vol.Filesystem != block.FilesystemTypeXFS {
			logger.Warn("vol.Filesystem != block.FilesystemTypeXFS", zap.Reflect("item", vol))

			return
		}

		volumeConfig, err := safe.ReaderGetByID[*block.VolumeConfig](ctx, r, item.Metadata().ID())
		if err != nil {
			logger.Warn("err", zap.Error(err))

			return
		}

		mountpoint := volumeConfig.TypedSpec().Mount.TargetPath

		if _, ok := ctrl.schedule[mountpoint]; !ok {
			per := 10 * time.Second
			seconds := rand.Int64N(int64(per.Seconds()))

			ctrl.schedule[mountpoint] = scrubSchedule{
				period:   per,
				upcoming: time.Now().Add(time.Duration(seconds * int64(time.Second))),
			}

			logger.Warn("scheduled", zap.String("path", mountpoint), zap.Reflect("upcoming", ctrl.schedule[mountpoint].upcoming))
		}
	})

	return nil
}

func (ctrl *FSScrubController) scrub(mountpoint string, opts []string) error {
	args := []string{"/usr/sbin/xfs_scrub", "-T", "-v"}
	args = append(args, opts...)
	args = append(args, mountpoint)

	r := process.NewRunner(
		false,
		&runner.Args{
			ID:          "fs_scrub",
			ProcessArgs: args,
		},
		runner.WithLoggingManager(ctrl.Runtime.Logging()),
		runner.WithEnv(environment.Get(ctrl.Runtime.Config())),
		runner.WithOOMScoreAdj(-999),
		runner.WithDroppedCapabilities(constants.XFSScrubDroppedCapabilities),
		runner.WithPriority(19),
		runner.WithIOPriority(runner.IoprioClassIdle, 7),
		runner.WithSchedulingPolicy(runner.SchedulingPolicyIdle),
	)

	return r.Run(func(s events.ServiceState, msg string, args ...any) {})
}
