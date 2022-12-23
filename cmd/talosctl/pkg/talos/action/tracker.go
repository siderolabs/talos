// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package action

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mattn/go-isatty"
	"github.com/siderolabs/gen/containers"
	"github.com/siderolabs/gen/maps"
	"github.com/siderolabs/go-circular"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/global"
	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/helpers"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/reporter"
)

var (
	// MachineReadyEventFn is the predicate function that returns true if the event indicates the machine is ready.
	MachineReadyEventFn = func(event client.EventResult) bool {
		machineStatusEvent, ok := event.Event.Payload.(*machineapi.MachineStatusEvent)
		if !ok {
			return false
		}

		return machineStatusEvent.GetStage() == machineapi.MachineStatusEvent_RUNNING &&
			machineStatusEvent.GetStatus().GetReady()
	}

	// StopAllServicesEventFn is the predicate function that returns true if the event indicates that all services are being stopped.
	StopAllServicesEventFn = func(event client.EventResult) bool {
		taskEvent, ok := event.Event.Payload.(*machineapi.TaskEvent)
		if !ok {
			return false
		}

		return taskEvent.GetTask() == "stopAllServices"
	}
)

type nodeUpdate struct {
	node   string
	update reporter.Update
}

// Tracker runs the action in the actionFn on the nodes and tracks its progress using the provided expectedEventFn and postCheckFn.
type Tracker struct {
	expectedEventFn          func(event client.EventResult) bool
	actionFn                 func(ctx context.Context, c *client.Client) (string, error)
	postCheckFn              func(ctx context.Context, c *client.Client) error
	reporter                 *reporter.Reporter
	nodeToLatestStatusUpdate map[string]reporter.Update
	reportCh                 chan nodeUpdate
	timeout                  time.Duration
	isTerminal               bool
	debug                    bool
	cliContext               *global.Args
}

// TrackerOption is the functional option for the Tracker.
type TrackerOption func(*Tracker)

// WithTimeout sets the timeout for the tracker.
func WithTimeout(timeout time.Duration) TrackerOption {
	return func(t *Tracker) {
		t.timeout = timeout
	}
}

// WithPostCheck sets the post check function.
func WithPostCheck(postCheckFn func(ctx context.Context, c *client.Client) error) TrackerOption {
	return func(t *Tracker) {
		t.postCheckFn = postCheckFn
	}
}

// WithDebug enables debug mode.
func WithDebug(debug bool) TrackerOption {
	return func(t *Tracker) {
		t.debug = debug
	}
}

// NewTracker creates a new Tracker.
func NewTracker(
	cliContext *global.Args,
	expectedEventFn func(event client.EventResult) bool,
	actionFn func(ctx context.Context, c *client.Client) (string, error),
	opts ...TrackerOption,
) *Tracker {
	tracker := Tracker{
		expectedEventFn:          expectedEventFn,
		actionFn:                 actionFn,
		nodeToLatestStatusUpdate: make(map[string]reporter.Update, len(cliContext.Nodes)),
		reporter:                 reporter.New(),
		reportCh:                 make(chan nodeUpdate),
		isTerminal:               isatty.IsTerminal(os.Stderr.Fd()),
		cliContext:               cliContext,
	}

	for _, option := range opts {
		option(&tracker)
	}

	return &tracker
}

// Run executes the action on nodes and tracks its progress by watching events with retries.
// After receiving the expected event, if provided, it tracks the progress by running the post check with retries.
//
//nolint:gocyclo
func (a *Tracker) Run() error {
	var failedNodesToDmesgs containers.ConcurrentMap[string, io.Reader]

	var eg errgroup.Group

	err := a.cliContext.WithClient(func(ctx context.Context, c *client.Client) error {
		ctx, cancel := context.WithTimeout(ctx, a.timeout)
		defer cancel()

		if err := helpers.ClientVersionCheck(ctx, c); err != nil {
			return err
		}

		eg.Go(func() error {
			return a.runReporter(ctx)
		})

		var trackEg errgroup.Group

		for _, node := range a.cliContext.Nodes {
			node := node

			var (
				dmesg *circular.Buffer
				err   error
			)

			if a.debug {
				dmesg, err = circular.NewBuffer()
				if err != nil {
					return err
				}
			}

			tracker := nodeTracker{
				ctx:     client.WithNode(ctx, node),
				node:    node,
				tracker: a,
				dmesg:   dmesg,
				cli:     c,
			}

			if a.debug {
				eg.Go(tracker.tailDebugLogs)
			}

			trackEg.Go(func() error {
				if trackErr := tracker.run(); trackErr != nil {
					if a.debug {
						failedNodesToDmesgs.Set(node, dmesg.GetReader())
					}

					tracker.update(reporter.Update{
						Message: trackErr.Error(),
						Status:  reporter.StatusError,
					})
				}

				return nil
			})
		}

		return trackEg.Wait()
	}, grpc.WithConnectParams(grpc.ConnectParams{
		// disable grpc backoff
		Backoff:           backoff.Config{},
		MinConnectTimeout: 20 * time.Second,
	}))
	if errors.Is(err, context.Canceled) {
		err = nil
	}

	eg.Wait() //nolint:errcheck

	if !a.debug {
		return err
	}

	var failedNodes []string

	failedNodesToDmesgs.ForEach(func(key string, _ io.Reader) {
		failedNodes = append(failedNodes, key)
	})

	if len(failedNodes) > 0 {
		sort.Strings(failedNodes)

		fmt.Fprintf(os.Stderr, "console logs for nodes %q:\n", failedNodes)

		for _, node := range failedNodes {
			dmesgReader, _ := failedNodesToDmesgs.Get(node)

			_, copyErr := io.Copy(os.Stderr, dmesgReader)
			if copyErr != nil {
				fmt.Fprintf(os.Stderr, "%q: failed to print debug logs: %v\n", node, copyErr)
			}
		}
	}

	return err
}

// runReporter starts the (colored) stderr reporter.
func (a *Tracker) runReporter(ctx context.Context) error {
	var (
		update       nodeUpdate
		reportUpdate reporter.Update
	)

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			if a.isTerminal {
				a.reporter.Report(reportUpdate)
			}

			return ctx.Err()

		case <-ticker.C:
			if a.isTerminal {
				a.reporter.Report(reportUpdate)
			}

		case update = <-a.reportCh:
			if !a.isTerminal {
				fmt.Fprintf(os.Stderr, "%q: %v\n", update.node, update.update.Message)

				continue
			}

			reportUpdate = a.processNodeUpdate(update)
		}
	}
}

func (a *Tracker) processNodeUpdate(update nodeUpdate) reporter.Update {
	if update.node != "" {
		a.nodeToLatestStatusUpdate[update.node] = update.update
	}

	nodes := maps.Keys(a.nodeToLatestStatusUpdate)
	sort.Strings(nodes)

	messages := make([]string, 0, len(nodes)+1)
	messages = append(messages, fmt.Sprintf("watching nodes: %v", nodes))

	for _, node := range nodes {
		nUpdate := a.nodeToLatestStatusUpdate[node]

		messages = append(messages, fmt.Sprintf("    * %s: %s", node, nUpdate.Message))
	}

	combinedMessage := strings.Join(messages, "\n")
	combinedStatus := func() reporter.Status {
		combined := reporter.StatusSucceeded

		for _, status := range a.nodeToLatestStatusUpdate {
			if status.Status == reporter.StatusError {
				return reporter.StatusError
			}

			if status.Status == reporter.StatusRunning {
				combined = reporter.StatusRunning
			}
		}

		return combined
	}()

	return reporter.Update{
		Message: combinedMessage,
		Status:  combinedStatus,
	}
}
