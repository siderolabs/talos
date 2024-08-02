// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package logdata implements the types and the data sources for the data sourced from the Talos dmesg API.
package logdata

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/helpers"
	"github.com/siderolabs/talos/internal/pkg/dashboard/resolver"
	"github.com/siderolabs/talos/internal/pkg/dashboard/util"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"github.com/siderolabs/talos/pkg/machinery/client"
)

// Data is a log line from a node.
type Data struct {
	Node  string
	Log   string
	Error string
}

// Source is a data source for Kernel (dmesg) logs.
type Source struct {
	client *client.Client

	resolver resolver.Resolver

	logCtxCancel context.CancelFunc

	eg   errgroup.Group
	once sync.Once

	LogCh chan Data
}

// NewSource initializes and returns Source data source.
func NewSource(client *client.Client, resolver resolver.Resolver) *Source {
	return &Source{
		client:   client,
		resolver: resolver,
		LogCh:    make(chan Data),
	}
}

// Start starts the data source.
func (source *Source) Start(ctx context.Context) {
	source.once.Do(func() {
		source.start(ctx)
	})
}

// Stop stops the data source.
func (source *Source) Stop() error {
	source.logCtxCancel()

	return source.eg.Wait()
}

func (source *Source) start(ctx context.Context) {
	ctx, source.logCtxCancel = context.WithCancel(ctx)

	for _, nodeContext := range util.NodeContexts(ctx) {
		source.eg.Go(func() error {
			return source.tailNodeWithRetries(nodeContext.Ctx, nodeContext.Node)
		})
	}
}

func (source *Source) tailNodeWithRetries(ctx context.Context, node string) error {
	for {
		readErr := source.readDmesg(ctx, node)
		if errors.Is(readErr, context.Canceled) || status.Code(readErr) == codes.Canceled {
			return nil
		}

		if readErr != nil {
			resolved := source.resolver.Resolve(node)

			source.LogCh <- Data{Node: resolved, Error: readErr.Error()}
		}

		// back off a bit before retrying
		sleepWithContext(ctx, 30*time.Second)
	}
}

func (source *Source) readDmesg(ctx context.Context, node string) error {
	dmesgStream, err := source.client.Dmesg(ctx, true, false)
	if err != nil {
		return fmt.Errorf("dashboard: error opening dmesg stream: %w", err)
	}

	readErr := helpers.ReadGRPCStream(dmesgStream, func(data *common.Data, _ string, _ bool) error {
		if len(data.Bytes) == 0 {
			return nil
		}

		line := strings.TrimSpace(string(data.Bytes))
		if line == "" {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case source.LogCh <- Data{Node: node, Log: line}:
		}

		return nil
	})
	if readErr != nil {
		return fmt.Errorf("error reading dmesg stream: %w", readErr)
	}

	return nil
}

func sleepWithContext(ctx context.Context, d time.Duration) {
	timer := time.NewTimer(d)
	select {
	case <-ctx.Done():
		if !timer.Stop() {
			<-timer.C
		}
	case <-timer.C:
	}
}
