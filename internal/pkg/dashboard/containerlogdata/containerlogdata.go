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
// switched away from: cancelling a stream does not retract the lines already queued behind it.
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

	LogCh chan Data
}

const (
	// logChBuffer is the capacity of LogCh, matching the logdata package: a generous buffer lets
	// the sender continue during UI-update bursts while the dashboard batches the queued lines.
	logChBuffer = 256

	// tailLines is how much scrollback to request when a stream opens.
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

	source.eg.Go(func() error {
		source.tailWithRetries(utils.NodeContext(ctx, target.Node), target)

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

func (source *Source) tailWithRetries(ctx context.Context, target Target) {
	for {
		err := source.tail(ctx, target)
		if errors.Is(err, context.Canceled) || status.Code(err) == codes.Canceled {
			return
		}

		if err != nil {
			source.send(ctx, Data{Target: target, Error: err.Error()})
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(retryInterval):
		}
	}
}

func (source *Source) tail(ctx context.Context, target Target) error {
	stream, err := source.client.Logs(ctx, target.Namespace, target.Driver, target.ID, true, tailLines)
	if err != nil {
		return fmt.Errorf("error opening log stream: %w", err)
	}

	// Logs arrive as arbitrary byte chunks, so buffer until a newline is seen and emit complete
	// lines, the same way `talosctl logs` does. A chunk boundary in the middle of a line would
	// otherwise show up as two log entries.
	var buf []byte

	return helpers.ReadGRPCStream(stream, func(data *common.Data, _ string, _ bool) error {
		buf = append(buf, data.Bytes...)

		for {
			idx := bytes.IndexByte(buf, '\n')
			if idx < 0 {
				break
			}

			source.send(ctx, Data{Target: target, Log: string(buf[:idx])})

			buf = buf[idx+1:]
		}

		return ctx.Err()
	})
}

func (source *Source) send(ctx context.Context, data Data) {
	select {
	case <-ctx.Done():
	case source.LogCh <- data:
	}
}
