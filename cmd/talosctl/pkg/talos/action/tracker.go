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
	"slices"
	"strings"
	"time"

	"github.com/mattn/go-isatty"
	"github.com/siderolabs/gen/containers"
	"github.com/siderolabs/gen/maps"
	"github.com/siderolabs/go-circular"
	"github.com/siderolabs/go-retry/retry"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/talos/cmd/talosctl/cmd/common"
	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/global"
	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/helpers"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/reporter"
)

const unauthorizedBootIDFallback = "(unauthorized)"

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

	// BootIDChangedPostCheckFn is a post check function that returns nil if the boot ID has changed.
	BootIDChangedPostCheckFn = func(ctx context.Context, c *client.Client, preActionBootID string) error {
		if preActionBootID == unauthorizedBootIDFallback {
			return nil
		}

		currentBootID, err := getBootID(ctx, c)
		if err != nil {
			return err
		}

		if preActionBootID == currentBootID {
			return retry.ExpectedErrorf("didn't reboot yet")
		}

		return nil
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
	postCheckFn              func(ctx context.Context, c *client.Client, preActionBootID string) error
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
func WithPostCheck(postCheckFn func(ctx context.Context, c *client.Client, preActionBootID string) error) TrackerOption {
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

		// Reporter is started, it will print the errors if there is any.
		// So from here on we can suppress the command error to be printed to avoid it being printed twice.
		common.SuppressErrors = true

		var trackEg errgroup.Group

		for _, node := range a.cliContext.Nodes {
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
				trackErr := tracker.run()
				if trackErr != nil {
					if a.debug {
						failedNodesToDmesgs.Set(node, dmesg.GetReader())
					}

					tracker.update(reporter.Update{
						Message: trackErr.Error(),
						Status:  reporter.StatusError,
					})
				}

				return trackErr
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
		slices.Sort(failedNodes)

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
	slices.Sort(nodes)

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

// getBootID reads the boot ID from the node.
// It returns the node as the first return value and the boot ID as the second.
func getBootID(ctx context.Context, c *client.Client) (string, error) {
	reader, err := c.Read(ctx, "/proc/sys/kernel/random/boot_id")
	if err != nil {
		return "", err
	}

	defer reader.Close() //nolint:errcheck

	body, err := io.ReadAll(reader)
	if err != nil {
		if status.Code(err) == codes.PermissionDenied { // we are not authorized to read the boot ID, skip the check
			return unauthorizedBootIDFallback, nil
		}

		return "", err
	}

	bootID := strings.TrimSpace(string(body))

	return bootID, reader.Close()
}
