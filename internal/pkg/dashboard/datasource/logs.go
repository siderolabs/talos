// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package datasource

import (
	"context"
	"errors"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/helpers"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"github.com/siderolabs/talos/pkg/machinery/client"
)

// NodeLog is a log line from a node.
type NodeLog struct {
	Node string
	Log  string
}

// Logs is a data source for Kernel (dmesg) logs.
type Logs struct {
	client *client.Client

	logCtxCancel context.CancelFunc

	eg   errgroup.Group
	once sync.Once

	LogCh chan NodeLog
}

// NewLogSource initializes and returns Logs data source.
func NewLogSource(client *client.Client) *Logs {
	return &Logs{
		client: client,
		LogCh:  make(chan NodeLog),
	}
}

// Start starts the data source.
func (l *Logs) Start(ctx context.Context) error {
	var err error

	l.once.Do(func() {
		err = l.start(ctx)
	})

	return err
}

func (l *Logs) start(ctx context.Context) error {
	ctx, l.logCtxCancel = context.WithCancel(ctx)

	dmesgStream, err := l.client.Dmesg(ctx, true, false)
	if err != nil {
		return err
	}

	l.eg.Go(func() error {
		return helpers.ReadGRPCStream(dmesgStream, func(data *common.Data, node string, multipleNodes bool) error {
			if len(data.Bytes) == 0 {
				return nil
			}

			line := strings.TrimSpace(string(data.Bytes))
			if line == "" {
				return nil
			}

			select {
			case <-ctx.Done():
				if errors.Is(ctx.Err(), context.Canceled) {
					return nil
				}

				return ctx.Err()
			case l.LogCh <- NodeLog{Node: node, Log: line}:
			}

			return nil
		})
	})

	return nil
}

// Stop stops the data source.
func (l *Logs) Stop() error {
	l.logCtxCancel()

	return l.eg.Wait()
}
