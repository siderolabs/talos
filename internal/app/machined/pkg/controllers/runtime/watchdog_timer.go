// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"fmt"
	"os"
	"syscall"
	"time"
	"unsafe"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// WatchdogTimerController watches v1alpha1.Config, creates/updates/deletes kernel module specs.
type WatchdogTimerController struct{}

// Name implements controller.Controller interface.
func (ctrl *WatchdogTimerController) Name() string {
	return "runtime.WatchdogTimerController"
}

// Inputs implements controller.Controller interface.
func (ctrl *WatchdogTimerController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: runtime.NamespaceName,
			Type:      runtime.WatchdogTimerConfigType,
			ID:        optional.Some(runtime.WatchdogTimerConfigID),
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *WatchdogTimerController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: runtime.WatchdogTimerStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo,cyclop
func (ctrl *WatchdogTimerController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
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

	var wd *os.File

	wdClose := func() {
		if wd == nil {
			return
		}

		logger.Info("closing hardware watchdog", zap.String("path", wd.Name()))

		// Magic close: make sure old watchdog won't trip after we close it
		if _, err := wd.WriteString("V"); err != nil {
			logger.Error("failed to send magic close to watchdog", zap.String("path", wd.Name()))
		}

		if err := wd.Close(); err != nil {
			logger.Error("failed to close watchdog", zap.String("path", wd.Name()))
		}

		wd = nil
	}

	defer wdClose()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-tickerC:
			if _, _, err := syscall.Syscall(syscall.SYS_IOCTL, wd.Fd(), unix.WDIOC_KEEPALIVE, 0); err != 0 {
				return fmt.Errorf("failed to feed watchdog: %w", err)
			}

			continue
		case <-r.EventCh():
		}

		cfg, err := safe.ReaderGetByID[*runtime.WatchdogTimerConfig](ctx, r, runtime.WatchdogTimerConfigID)
		if err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting watchdog config: %w", err)
			}
		}

		r.StartTrackingOutputs()

		if cfg == nil {
			tickerStop()
			wdClose()
		} else {
			// close the watchdog if requested to use new one
			if wd != nil && wd.Name() != cfg.TypedSpec().Device {
				wdClose()
			}

			if wd == nil {
				wd, err = os.OpenFile(cfg.TypedSpec().Device, syscall.O_RDWR, 0o600)
				if err != nil {
					return fmt.Errorf("failed to open watchdog device: %s", err)
				}

				logger.Info("opened hardware watchdog", zap.String("path", cfg.TypedSpec().Device))
			}

			timeout := int(cfg.TypedSpec().Timeout.Seconds())

			if _, _, err := syscall.Syscall(syscall.SYS_IOCTL, wd.Fd(), uintptr(unix.WDIOC_SETTIMEOUT), uintptr(unsafe.Pointer(&timeout))); err != 0 {
				return fmt.Errorf("failed to set watchdog timeout: %w", err)
			}

			tickerStop()

			// 3 pings per timeout should suffice in any case
			feedInterval := cfg.TypedSpec().Timeout / 3

			ticker = time.NewTicker(feedInterval)
			tickerC = ticker.C

			if _, _, err := syscall.Syscall(syscall.SYS_IOCTL, wd.Fd(), uintptr(unix.WDIOC_KEEPALIVE), 0); err != 0 {
				return fmt.Errorf("failed to feed watchdog: %w", err)
			}

			logger.Info("set hardware watchdog timeout", zap.Duration("timeout", cfg.TypedSpec().Timeout), zap.Duration("feed_interval", feedInterval))

			if err = safe.WriterModify(ctx, r, runtime.NewWatchdogTimerStatus(cfg.Metadata().ID()), func(status *runtime.WatchdogTimerStatus) error {
				status.TypedSpec().Device = cfg.TypedSpec().Device
				status.TypedSpec().Timeout = cfg.TypedSpec().Timeout
				status.TypedSpec().FeedInterval = feedInterval

				return nil
			}); err != nil {
				return fmt.Errorf("error updating watchdog status: %w", err)
			}
		}

		if err = safe.CleanupOutputs[*runtime.WatchdogTimerStatus](ctx, r); err != nil {
			return err
		}
	}
}
