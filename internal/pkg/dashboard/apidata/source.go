// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package apidata

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/helpers"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/client/multiplex"
)

// Source is a data source that gathers information about a Talos node using Talos API.
type Source struct {
	*client.Client

	Nodes []string

	Interval time.Duration

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc

	wg sync.WaitGroup
}

// Run the data poll on interval.
func (source *Source) Run(ctx context.Context) <-chan *Data {
	dataCh := make(chan *Data)

	source.ctx, source.ctxCancel = context.WithCancel(ctx)

	source.wg.Add(1)

	go source.run(dataCh)

	return dataCh
}

// Stop the data collection process.
func (source *Source) Stop() {
	source.ctxCancel()

	source.wg.Wait()
}

func (source *Source) run(dataCh chan<- *Data) {
	defer source.wg.Done()
	defer close(dataCh)

	ticker := time.NewTicker(source.Interval)
	defer ticker.Stop()

	var oldData, currentData *Data

	for {
		currentData = source.gather()

		if oldData == nil {
			currentData.CalculateDiff(currentData)
		} else {
			currentData.CalculateDiff(oldData)
		}

		select {
		case dataCh <- currentData:
		case <-source.ctx.Done():
			return
		}

		select {
		case <-source.ctx.Done():
			return
		case <-ticker.C:
		}

		oldData = currentData
	}
}

type protoMsg[T any] interface {
	GetMessages() []T
}

func unpack[T helpers.Message](nodes map[string]*Node, resultLock *sync.Mutex, nodeName string, msg T, setter func(node *Node, value T)) {
	resultLock.Lock()
	defer resultLock.Unlock()

	if _, ok := nodes[nodeName]; !ok {
		nodes[nodeName] = &Node{}
	}

	setter(nodes[nodeName], msg)
}

func runGather[t helpers.Message](
	source *Source,
	nodes map[string]*Node,
	resultLock *sync.Mutex,
	gatherFunc func(context.Context) (protoMsg[t], error),
	setter func(node *Node, value t),
) error {
	var (
		resp protoMsg[t]
		err  error
	)

	if len(source.Nodes) == 1 && source.Nodes[0] == "" {
		// local gather case, no need to multiplex by node
		resp, err = gatherFunc(source.ctx)
		if err != nil {
			return err
		}

		for _, msg := range resp.GetMessages() {
			unpack(nodes, resultLock, "", msg, setter)
		}

		return nil
	}

	respCh := multiplex.Unary(source.ctx, source.Nodes, func(ctx context.Context) (protoMsg[t], error) {
		return gatherFunc(ctx)
	})

	var errs error

	for msg := range respCh {
		if msg.Err != nil {
			errs = errors.Join(errs, fmt.Errorf("error gathering data from node %q: %w", msg.Node, msg.Err))

			continue
		}

		for _, m := range msg.Payload.GetMessages() {
			unpack(nodes, resultLock, msg.Node, m, setter)
		}
	}

	return errs
}

//nolint:gocyclo
func (source *Source) gather() *Data {
	result := &Data{
		Timestamp: time.Now(),
		Nodes:     map[string]*Node{},
	}

	var resultLock sync.Mutex

	gatherFuncs := []func() error{
		func() error {
			return runGather(
				source, result.Nodes, &resultLock,
				func(ctx context.Context) (protoMsg[*machine.LoadAvg], error) {
					return source.MachineClient.LoadAvg(ctx, &emptypb.Empty{})
				},
				func(node *Node, value *machine.LoadAvg) {
					node.LoadAvg = value
				},
			)
		},
		func() error {
			return runGather(
				source, result.Nodes, &resultLock,
				func(ctx context.Context) (protoMsg[*machine.Memory], error) {
					return source.MachineClient.Memory(ctx, &emptypb.Empty{})
				},
				func(node *Node, value *machine.Memory) {
					node.Memory = value
				},
			)
		},
		func() error {
			return runGather(
				source, result.Nodes, &resultLock,
				func(ctx context.Context) (protoMsg[*machine.SystemStat], error) {
					return source.MachineClient.SystemStat(ctx, &emptypb.Empty{})
				},
				func(node *Node, value *machine.SystemStat) {
					node.SystemStat = value
				},
			)
		},
		func() error {
			return runGather(
				source, result.Nodes, &resultLock,
				func(ctx context.Context) (protoMsg[*machine.CPUsFreqStats], error) {
					return source.MachineClient.CPUFreqStats(ctx, &emptypb.Empty{})
				},
				func(node *Node, value *machine.CPUsFreqStats) {
					node.CPUsFreqStats = value
				},
			)
		},
		func() error {
			return runGather(
				source, result.Nodes, &resultLock,
				func(ctx context.Context) (protoMsg[*machine.CPUsInfo], error) {
					return source.MachineClient.CPUInfo(ctx, &emptypb.Empty{})
				},
				func(node *Node, value *machine.CPUsInfo) {
					node.CPUsInfo = value
				},
			)
		},
		func() error {
			return runGather(
				source, result.Nodes, &resultLock,
				func(ctx context.Context) (protoMsg[*machine.NetworkDeviceStats], error) {
					return source.MachineClient.NetworkDeviceStats(ctx, &emptypb.Empty{})
				},
				func(node *Node, value *machine.NetworkDeviceStats) {
					node.NetDevStats = value
				},
			)
		},
		func() error {
			return runGather(
				source, result.Nodes, &resultLock,
				func(ctx context.Context) (protoMsg[*machine.DiskStats], error) {
					return source.MachineClient.DiskStats(ctx, &emptypb.Empty{})
				},
				func(node *Node, value *machine.DiskStats) {
					node.DiskStats = value
				},
			)
		},
		func() error {
			return runGather(
				source, result.Nodes, &resultLock,
				func(ctx context.Context) (protoMsg[*machine.Process], error) {
					return source.MachineClient.Processes(ctx, &emptypb.Empty{})
				},
				func(node *Node, value *machine.Process) {
					node.Processes = value
				},
			)
		},
		func() error {
			return runGather(
				source, result.Nodes, &resultLock,
				func(ctx context.Context) (protoMsg[*machine.ServiceList], error) {
					return source.MachineClient.ServiceList(ctx, &emptypb.Empty{})
				},
				func(node *Node, value *machine.ServiceList) {
					node.ServiceList = value
				},
			)
		},
	}

	var eg errgroup.Group

	for _, f := range gatherFuncs {
		eg.Go(f)
	}

	if err := eg.Wait(); err != nil {
		// TODO: handle error
		_ = err
	}

	return result
}
