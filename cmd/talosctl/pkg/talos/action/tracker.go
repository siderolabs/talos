// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package action

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mattn/go-isatty"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"

	"github.com/talos-systems/talos/cmd/talosctl/pkg/talos/global"
	"github.com/talos-systems/talos/cmd/talosctl/pkg/talos/helpers"
	"github.com/talos-systems/talos/pkg/circular"
	machineapi "github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/client"
	"github.com/talos-systems/talos/pkg/machinery/generic/maps"
	"github.com/talos-systems/talos/pkg/reporter"
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
	retryDuration            time.Duration
	isTerminal               bool
	debug                    bool
	cliContext               *global.Args
}

// NewTracker creates a new Tracker.
func NewTracker(
	cliContext *global.Args,
	expectedEventFn func(event client.EventResult) bool,
	actionFn func(ctx context.Context, c *client.Client) (string, error),
	postCheckFn func(ctx context.Context, c *client.Client) error,
	debug bool,
) *Tracker {
	return &Tracker{
		expectedEventFn:          expectedEventFn,
		actionFn:                 actionFn,
		postCheckFn:              postCheckFn,
		nodeToLatestStatusUpdate: make(map[string]reporter.Update, len(cliContext.Nodes)),
		reporter:                 reporter.New(),
		reportCh:                 make(chan nodeUpdate),
		retryDuration:            15 * time.Minute,
		isTerminal:               isatty.IsTerminal(os.Stderr.Fd()),
		debug:                    debug,
		cliContext:               cliContext,
	}
}

// Run executes the action on nodes and tracks its progress by watching events with retries.
// After receiving the expected event, if provided, it tracks the progress by running the post check with retries.
//
//nolint:gocyclo
func (a *Tracker) Run() error {
	var failedNodesToDmesgs sync.Map

	var eg errgroup.Group

	defer func() {
		eg.Wait() //nolint:errcheck

		var failedNodes []string

		failedNodesToDmesgs.Range(func(key, value any) bool {
			failedNodes = append(failedNodes, key.(string))

			return true
		})

		if a.debug && len(failedNodes) > 0 {
			sort.Strings(failedNodes)

			fmt.Printf("console logs for nodes %v:\n", failedNodes)

			for _, node := range failedNodes {
				dmesgReaderRaw, _ := failedNodesToDmesgs.Load(node)
				dmesgReader := dmesgReaderRaw.(io.Reader) //nolint:errcheck

				_, err := io.Copy(os.Stdout, dmesgReader)
				if err != nil {
					fmt.Printf("%v: failed to print debug logs: %v\n", node, err)
				}
			}
		}
	}()

	return a.cliContext.WithClient(func(ctx context.Context, c *client.Client) error {
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
						failedNodesToDmesgs.Store(node, dmesg.GetReader())
					}

					tracker.update(reporter.Update{
						Message: trackErr.Error(),
						Status:  reporter.StatusError,
					})
				}

				return err
			})
		}

		return trackEg.Wait()
	}, grpc.WithConnectParams(grpc.ConnectParams{
		// disable grpc backoff
		Backoff:           backoff.Config{},
		MinConnectTimeout: 20 * time.Second,
	}))
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
				fmt.Printf("%v: %v\n", update.node, update.update.Message)

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
