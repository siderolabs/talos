// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package logdata implements the types and the data sources for the data sourced from the Talos dmesg API.
package logdata

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

// Data is a log line from a node.
type Data struct {
	Node string
	Log  string
}

// Source is a data source for Kernel (dmesg) logs.
type Source struct {
	client *client.Client

	logCtxCancel context.CancelFunc

	eg   errgroup.Group
	once sync.Once

	LogCh chan Data
}

// NewSource initializes and returns Source data source.
func NewSource(client *client.Client) *Source {
	return &Source{
		client: client,
		LogCh:  make(chan Data),
	}
}

// Start starts the data source.
func (source *Source) Start(ctx context.Context) error {
	var err error

	source.once.Do(func() {
		err = source.start(ctx)
	})

	return err
}

// Stop stops the data source.
func (source *Source) Stop() error {
	source.logCtxCancel()

	return source.eg.Wait()
}

func (source *Source) start(ctx context.Context) error {
	ctx, source.logCtxCancel = context.WithCancel(ctx)

	dmesgStream, err := source.client.Dmesg(ctx, true, false)
	if err != nil {
		return err
	}

	source.eg.Go(func() error {
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
			case source.LogCh <- Data{Node: node, Log: line}:
			}

			return nil
		})
	})

	return nil
}
