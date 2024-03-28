// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package action

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/siderolabs/go-circular"
	"github.com/siderolabs/go-retry/retry"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/codes"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/helpers"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/reporter"
)

// nodeTracker tracks the actions of a single node.
type nodeTracker struct {
	ctx     context.Context //nolint:containedctx
	node    string
	tracker *Tracker
	dmesg   *circular.Buffer
	cli     *client.Client
}

// tailDebugLogs starts tailing the dmesg of the node.
func (a *nodeTracker) tailDebugLogs() error {
	return retry.Constant(a.tracker.timeout).RetryWithContext(a.ctx, func(ctx context.Context) error {
		err := func() error {
			stream, err := a.cli.Dmesg(ctx, true, true)
			if err != nil {
				return err
			}

			return helpers.ReadGRPCStream(stream, func(data *common.Data, _ string, _ bool) error {
				_, err := a.dmesg.Write([]byte(fmt.Sprintf("%s: %s", a.node, data.GetBytes())))

				return err
			})
		}()
		if err == nil {
			return nil
		}

		if strings.Contains(err.Error(), "file already closed") {
			return retry.ExpectedError(err)
		}

		statusCode := client.StatusCode(err)
		if errors.Is(err, io.EOF) || statusCode == codes.Unavailable {
			return retry.ExpectedError(err)
		}

		return err
	})
}

func (a *nodeTracker) run() error {
	var (
		actorIDCh                chan string
		nodeEg                   errgroup.Group
		actorID, preActionBootID string
		err                      error
	)

	actorIDCh = make(chan string)

	nodeEg.Go(func() error {
		return a.trackEventsWithRetry(actorIDCh)
	})

	if a.tracker.postCheckFn != nil {
		preActionBootID, err = getBootID(a.ctx, a.cli)
		if err != nil {
			return err
		}
	}

	actorID, err = a.tracker.actionFn(a.ctx, a.cli)
	if err != nil {
		return err
	}

	select {
	case actorIDCh <- actorID:
	case <-a.ctx.Done():
		return a.ctx.Err()
	}

	err = nodeEg.Wait()
	if err != nil {
		return err
	}

	if a.tracker.postCheckFn == nil {
		return nil
	}

	return a.runPostCheckWithRetry(preActionBootID)
}

func (a *nodeTracker) update(update reporter.Update) {
	select {
	case a.tracker.reportCh <- nodeUpdate{
		node:   a.node,
		update: update,
	}:
	case <-a.ctx.Done():
	}
}

func (a *nodeTracker) trackEventsWithRetry(actorIDCh chan string) error {
	var (
		tailEvents     int32
		actorID        string
		waitForActorID = true
	)

	return retry.Constant(a.tracker.timeout).RetryWithContext(a.ctx, func(ctx context.Context) error {
		// retryable function
		err := func() error {
			eventCh := make(chan client.EventResult)

			err := a.cli.EventsWatchV2(ctx, eventCh, client.WithTailEvents(tailEvents))
			if err != nil {
				return err
			}

			if waitForActorID {
				a.update(reporter.Update{
					Message: "waiting for actor ID",
					Status:  reporter.StatusRunning,
				})

				select {
				case actorID = <-actorIDCh:
				case <-ctx.Done():
					return ctx.Err()
				}

				a.update(reporter.Update{
					Message: fmt.Sprintf("actor ID: %v", actorID),
					Status:  reporter.StatusRunning,
				})

				waitForActorID = false
			}

			return a.handleEvents(eventCh, actorID)
		}()

		// handle retryable errors

		statusCode := client.StatusCode(err)
		if errors.Is(err, io.EOF) || statusCode == codes.Unavailable {
			a.update(reporter.Update{
				Message: "unavailable, retrying...",
				Status:  reporter.StatusError,
			})

			tailEvents = -1
			actorID = ""

			return retry.ExpectedError(err)
		}

		if err != nil {
			a.update(reporter.Update{
				Message: fmt.Sprintf("error: %v", err),
				Status:  reporter.StatusError,
			})
		}

		return err
	})
}

func (a *nodeTracker) runPostCheckWithRetry(preActionBootID string) error {
	return retry.Constant(a.tracker.timeout).RetryWithContext(a.ctx, func(ctx context.Context) error {
		// retryable function
		err := func() error {
			err := a.tracker.postCheckFn(ctx, a.cli, preActionBootID)
			if err != nil {
				return err
			}

			a.update(reporter.Update{
				Message: "post check passed",
				Status:  reporter.StatusSucceeded,
			})

			return nil
		}()

		// handle retryable errors
		statusCode := client.StatusCode(err)
		if errors.Is(err, io.EOF) || statusCode == codes.Unavailable || statusCode == codes.Canceled {
			a.update(reporter.Update{
				Message: "unavailable, retrying...",
				Status:  reporter.StatusError,
			})

			return retry.ExpectedError(err)
		}

		return err
	})
}

func (a *nodeTracker) handleEvents(eventCh chan client.EventResult, actorID string) error {
	for {
		var eventResult client.EventResult

		select {
		case eventResult = <-eventCh:
		case <-a.ctx.Done():
			return a.ctx.Err()
		}

		if a.tracker.expectedEventFn(eventResult) {
			status := reporter.StatusSucceeded
			if a.tracker.postCheckFn != nil {
				status = reporter.StatusRunning
			}

			a.update(reporter.Update{
				Message: "events check condition met",
				Status:  status,
			})

			return nil
		}

		if eventResult.Error != nil {
			return eventResult.Error
		}

		if eventResult.Event.ActorID == actorID {
			err := a.handleEvent(eventResult.Event)
			if err != nil {
				return err
			}
		}
	}
}

func (a *nodeTracker) handleEvent(event client.Event) error {
	switch msg := event.Payload.(type) {
	case *machineapi.PhaseEvent:
		a.update(reporter.Update{
			Message: fmt.Sprintf("phase: %s action: %v", msg.GetPhase(), msg.GetAction()),
			Status:  reporter.StatusRunning,
		})

	case *machineapi.TaskEvent:
		a.update(reporter.Update{
			Message: fmt.Sprintf("task: %s action: %v", msg.GetTask(), msg.GetAction()),
			Status:  reporter.StatusRunning,
		})

		if msg.GetTask() == "stopAllServices" {
			return retry.ExpectedErrorf("stopAllServices task completed")
		}

	case *machineapi.SequenceEvent:
		errStr := ""
		if msg.GetError().GetMessage() != "" {
			errStr = fmt.Sprintf(
				" error: [code: %v message: %v]",
				msg.GetError().GetMessage(),
				msg.GetError().GetCode(),
			)
		}

		a.update(reporter.Update{
			Message: fmt.Sprintf("sequence: %s action: %v%v", msg.GetSequence(), msg.GetAction(), errStr),
			Status:  reporter.StatusRunning,
		})

		if errStr != "" {
			return fmt.Errorf("sequence error: %s", msg.GetError().GetMessage())
		}

	case *machineapi.MachineStatusEvent:
		a.update(reporter.Update{
			Message: fmt.Sprintf("stage: %v ready: %v unmetCond: %v", msg.GetStage(), msg.GetStatus().GetReady(), msg.GetStatus().GetUnmetConditions()),
			Status:  reporter.StatusRunning,
		})

	case *machineapi.ServiceStateEvent:
		a.update(reporter.Update{
			Message: fmt.Sprintf("service: %v message: %v healthy: %v", msg.GetService(), msg.GetMessage(), msg.GetHealth().GetHealthy()),
			Status:  reporter.StatusRunning,
		})
	}

	return nil
}
