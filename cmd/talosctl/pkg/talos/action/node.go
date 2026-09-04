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
	"time"

	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-circular"
	"github.com/siderolabs/go-retry/retry"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/codes"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/action/internal/stall"
	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/helpers"
	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/safeout"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/reporter"
)

const (
	// eventStallTimeout bounds how long the tracker waits on an event stream which was established
	// successfully, but delivers nothing.
	//
	// The tracker's only liveness signal is an attempt returning an error, and a connection whose
	// peer went away without closing it never produces one: EventsWatchV2 blocks in stream.Recv
	// before it even returns, and retry only re-evaluates its deadline between attempts, so a
	// single blocked attempt bypasses the retry loop entirely until the overall timeout fires.
	//
	// A reconnect resumes from the last observed event ID, so firing on a legitimate quiet period
	// (an upgrade waiting on a slow image pull emits no events for minutes) neither misses nor
	// replays events: it is invisible to the rest of the tracking logic, which is what allows the
	// timeout to be shorter than the longest expected gap between events.
	eventStallTimeout = 5 * time.Minute

	// postCheckTimeout bounds a single post check attempt, which runs over the same connection and
	// can block in the same way the event stream does.
	postCheckTimeout = 2 * time.Minute
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
				_, err := fmt.Fprintf(a.dmesg, "%s: %s", a.node, data.GetBytes())

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

//nolint:gocyclo
func (a *nodeTracker) trackEventsWithRetry(actorIDCh chan string) error {
	var (
		tailEvents     int32
		lastEventID    string
		actorID        string
		waitForActorID = true
	)

	return retry.Constant(a.tracker.timeout).RetryWithContext(a.ctx, func(ctx context.Context) error {
		// each attempt runs under its own context so that the stall detector can abort an attempt
		// which established a stream but is not receiving anything from it
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		detector := stall.NewDetector(ctx, eventStallTimeout, cancel)

		// resume from the last observed event if the position is known: event IDs are ordered
		// across reboots, so this replays whatever was missed while disconnected without
		// re-delivering the events which were already handled
		opts := []client.EventsOptionFunc{client.WithTailEvents(tailEvents)}
		if lastEventID != "" {
			opts = []client.EventsOptionFunc{client.WithTailID(lastEventID)}
		}

		// retryable function
		err := func() error {
			eventCh := make(chan client.EventResult)

			err := a.cli.EventsWatchV2(ctx, eventCh, opts...)
			if err != nil {
				return err
			}

			detector.Poke()

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
					Message: fmt.Sprintf("actor ID: %v", safeout.String(actorID)),
					Status:  reporter.StatusRunning,
				})

				waitForActorID = false

				detector.Poke()
			}

			return a.handleEvents(ctx, eventCh, actorID, &lastEventID, detector.Poke)
		}()
		if err == nil {
			return nil
		}

		// handle retryable errors

		if detector.Stalled() {
			a.update(reporter.Update{
				Message: fmt.Sprintf("no events received in %s, reconnecting...", eventStallTimeout),
				Status:  reporter.StatusRunning,
			})

			return retry.ExpectedError(err)
		}

		statusCode := client.StatusCode(err)
		if errors.Is(err, io.EOF) || statusCode == codes.Unavailable || statusCode == codes.Canceled {
			a.update(reporter.Update{
				Message: "unavailable, retrying...",
				Status:  reporter.StatusError,
			})

			if lastEventID == "" {
				// the position in the event stream is not known, so fall back to replaying the
				// whole history, dropping the actor ID as the replay covers events which predate
				// the action
				tailEvents = -1
				actorID = ""
			}

			return retry.ExpectedError(err)
		}

		a.update(reporter.Update{
			Message: fmt.Sprintf("error: %v", safeout.String(err.Error())),
			Status:  reporter.StatusError,
		})

		return err
	})
}

func (a *nodeTracker) runPostCheckWithRetry(preActionBootID string) error {
	return retry.Constant(a.tracker.timeout).RetryWithContext(a.ctx, func(ctx context.Context) error {
		// bound a single attempt: the post check talks to the node over the same connection the
		// event stream used, so it can block indefinitely for the same reason
		ctx, cancel := context.WithTimeout(ctx, postCheckTimeout)
		defer cancel()

		// retryable function
		err := func() error {
			err := a.tracker.postCheckFn(ctx, a.cli, a.node, preActionBootID)
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
		if errors.Is(err, context.DeadlineExceeded) || client.StatusCode(err) == codes.DeadlineExceeded {
			a.update(reporter.Update{
				Message: fmt.Sprintf("post check stuck for %s, retrying...", postCheckTimeout),
				Status:  reporter.StatusRunning,
			})

			return retry.ExpectedError(err)
		}

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

func (a *nodeTracker) handleEvents(ctx context.Context, eventCh chan client.EventResult, actorID string, lastEventID *string, progress func()) error {
	for {
		var eventResult client.EventResult

		select {
		case eventResult = <-eventCh:
		case <-ctx.Done():
			return ctx.Err()
		}

		progress()

		if eventResult.Event.ID != "" {
			*lastEventID = eventResult.Event.ID
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
			Message: fmt.Sprintf("phase: %s action: %v", safeout.String(msg.GetPhase()), msg.GetAction()),
			Status:  reporter.StatusRunning,
		})

	case *machineapi.TaskEvent:
		a.update(reporter.Update{
			Message: fmt.Sprintf("task: %s action: %v", safeout.String(msg.GetTask()), msg.GetAction()),
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
				msg.GetError().GetCode(),
				safeout.String(msg.GetError().GetMessage()),
			)
		}

		a.update(reporter.Update{
			Message: fmt.Sprintf("sequence: %s action: %v%v", safeout.String(msg.GetSequence()), msg.GetAction(), errStr),
			Status:  reporter.StatusRunning,
		})

		if msg.GetSequence() == "reboot" {
			return retry.ExpectedErrorf("reboot sequence completed")
		}

		if errStr != "" {
			return fmt.Errorf("sequence error: %s", safeout.String(msg.GetError().GetMessage()))
		}

	case *machineapi.MachineStatusEvent:
		a.update(reporter.Update{
			Message: fmt.Sprintf("stage: %v ready: %v unmetCond: %v", msg.GetStage(), msg.GetStatus().GetReady(),
				xslices.Map(msg.GetStatus().GetUnmetConditions(), func(c *machineapi.MachineStatusEvent_MachineStatus_UnmetCondition) string {
					return safeout.String(c.GetName())
				})),
			Status: reporter.StatusRunning,
		})

	case *machineapi.ServiceStateEvent:
		a.update(reporter.Update{
			Message: fmt.Sprintf("service: %v message: %v healthy: %v", safeout.String(msg.GetService()), safeout.String(msg.GetMessage()), msg.GetHealth().GetHealthy()),
			Status:  reporter.StatusRunning,
		})
	}

	return nil
}
