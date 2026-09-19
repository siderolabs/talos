// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package containerlogdata implements the data source for the logs of a single container.
//
// It differs from the logdata package in that it is on-demand and single-target: the dashboard
// tails exactly the container the user is looking at, and switches the stream over when the
// selection changes.
package containerlogdata

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/helpers"
	"github.com/siderolabs/talos/internal/pkg/dashboard/utils"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"github.com/siderolabs/talos/pkg/machinery/client"
)

// Target identifies the container being tailed.
type Target struct {
	Node      string
	Namespace string
	Driver    common.ContainerDriver
	ID        string
}

// Data is a log line from a container.
//
// It carries its Target so that the consumer can drop lines belonging to a stream it has already
// switched away from: canceling a stream does not retract the lines already queued behind it.
type Data struct {
	Target Target
	Log    string
	Error  string
}

// Source streams the logs of one container at a time.
type Source struct {
	client *client.Client

	cancel context.CancelFunc
	eg     errgroup.Group

	// delivered is the target whose scrollback is already on screen. Resuming that same target -
	// a reopen after a failure, or a screen the operator left and came back to - therefore asks
	// for new lines only. The zero value matches no real target, which always has an ID.
	delivered Target

	LogCh chan Data
}

const (
	// logChBuffer is the capacity of LogCh, matching the logdata package: a generous buffer lets
	// the sender continue during UI-update bursts while the dashboard batches the queued lines.
	logChBuffer = 256

	// tailLines is how much scrollback to request for a container nothing has been shown for yet.
	// Zero means "new lines only" to the machine API, and that is what every other open asks for.
	tailLines = 1000

	// retryInterval is the wait before reopening a stream that failed. It is deliberately short:
	// a container being restarted comes back on the order of seconds, and the operator watching
	// its logs is usually watching precisely because it keeps failing.
	retryInterval = 5 * time.Second
)

// NewSource initializes and returns a Source.
func NewSource(client *client.Client) *Source {
	return &Source{
		client: client,
		LogCh:  make(chan Data, logChBuffer),
	}
}

// Start tails the logs of the given container, replacing whatever stream was running before.
func (source *Source) Start(ctx context.Context, target Target) {
	source.Stop()

	ctx, source.cancel = context.WithCancel(ctx)

	// Scrollback is only worth asking for when the viewer is not already showing it: the consumer
	// appends whatever arrives, so tailing again would repeat up to tailLines lines it still has on
	// screen. The cost is the lines written while the stream was down, which is the lesser of the
	// two confusions.
	tail := int32(tailLines)
	if source.delivered == target {
		tail = 0
	}

	source.eg.Go(func() error {
		source.tailWithRetries(utils.NodeContext(ctx, target.Node), target, tail)

		return nil
	})
}

// Stop stops the running stream, if any, and waits for it to finish.
func (source *Source) Stop() {
	if source.cancel == nil {
		return
	}

	source.cancel()
	source.cancel = nil

	source.eg.Wait() //nolint:errcheck // tailWithRetries never returns an error
}

// Forget stops the running stream and drops the record of what it has already delivered, so that
// the next stream tails the scrollback again. It belongs with clearing the viewer the lines went
// into, and stopping first is what keeps the record from being written behind its back.
func (source *Source) Forget() {
	source.Stop()

	source.delivered = Target{}
}

func (source *Source) tailWithRetries(ctx context.Context, target Target, tail int32) {
	for {
		err := source.tail(ctx, target, tail)
		if errors.Is(err, context.Canceled) || status.Code(err) == codes.Canceled {
			return
		}

		if err != nil {
			source.send(ctx, Data{Target: target, Error: err.Error()})
		}

		// The scrollback request is only dropped once a line has actually been shown: an open that
		// failed before delivering anything left the viewer empty, and asking for new lines only
		// would lose the history for good.
		if source.delivered == target {
			tail = 0
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(retryInterval):
		}
	}
}

func (source *Source) tail(ctx context.Context, target Target, tailLines int32) error {
	stream, err := source.client.Logs(ctx, target.Namespace, target.Driver, target.ID, true, tailLines)
	if err != nil {
		return fmt.Errorf("error opening log stream: %w", err)
	}

	// Logs arrive as arbitrary byte chunks, so buffer until a newline is seen and emit complete
	// lines, the same way `talosctl logs` does. A chunk boundary in the middle of a line would
	// otherwise show up as two log entries.
	var buf []byte

	err = helpers.ReadGRPCStream(stream, func(data *common.Data, _ string, _ bool) error {
		buf = append(buf, data.Bytes...)

		for {
			idx := bytes.IndexByte(buf, '\n')
			if idx < 0 {
				break
			}

			source.sendLog(ctx, target, string(buf[:idx]))

			buf = buf[idx+1:]
		}

		return ctx.Err()
	})

	// A stream that ended mid-line still has that line to show: a container whose last write had no
	// trailing newline would otherwise lose it entirely, as would every retry that drops a partial
	// line on the floor. Cancellation is the one case where it is dropped, the consumer having
	// switched away already.
	if len(buf) > 0 && ctx.Err() == nil {
		source.sendLog(ctx, target, string(buf))
	}

	return err
}

// sendLog emits a log line and records that the target now has something on screen.
func (source *Source) sendLog(ctx context.Context, target Target, log string) {
	if source.send(ctx, Data{Target: target, Log: log}) {
		source.delivered = target
	}
}

// send emits a message, and reports whether it was delivered rather than dropped on cancellation.
func (source *Source) send(ctx context.Context, data Data) bool {
	select {
	case <-ctx.Done():
		return false
	case source.LogCh <- data:
		return true
	}
}
