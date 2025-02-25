// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package apidata

import (
	"context"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/helpers"
	"github.com/siderolabs/talos/internal/pkg/dashboard/resolver"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
)

// Source is a data source that gathers information about a Talos node using Talos API.
type Source struct {
	*client.Client

	Resolver resolver.Resolver

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

func unpack[T helpers.Message](source *Source, nodes map[string]*Node, resultLock *sync.Mutex, resp protoMsg[T], setter func(node *Node, value T)) {
	resultLock.Lock()
	defer resultLock.Unlock()

	for _, msg := range resp.GetMessages() {
		node := source.node(msg)

		if _, ok := nodes[node]; !ok {
			nodes[node] = &Node{}
		}

		if msg.GetMetadata().GetError() != "" {
			continue
		}

		setter(nodes[node], msg)
	}
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
			resp, err := source.MachineClient.LoadAvg(source.ctx, &emptypb.Empty{})
			if err != nil {
				return err
			}

			unpack(source, result.Nodes, &resultLock, resp, func(node *Node, value *machine.LoadAvg) {
				node.LoadAvg = value
			})

			return nil
		},
		func() error {
			resp, err := source.MachineClient.Version(source.ctx, &emptypb.Empty{})
			if err != nil {
				return err
			}

			unpack(source, result.Nodes, &resultLock, resp, func(node *Node, value *machine.Version) {
				node.Version = value
			})

			return nil
		},
		func() error {
			resp, err := source.MachineClient.Memory(source.ctx, &emptypb.Empty{})
			if err != nil {
				return err
			}

			unpack(source, result.Nodes, &resultLock, resp, func(node *Node, value *machine.Memory) {
				node.Memory = value
			})

			return nil
		},
		func() error {
			resp, err := source.MachineClient.SystemStat(source.ctx, &emptypb.Empty{})
			if err != nil {
				return err
			}

			unpack(source, result.Nodes, &resultLock, resp, func(node *Node, value *machine.SystemStat) {
				node.SystemStat = value
			})

			return nil
		},
		func() error {
			resp, err := source.MachineClient.CPUFreqStats(source.ctx, &emptypb.Empty{})
			if err != nil {
				return err
			}

			unpack(source, result.Nodes, &resultLock, resp, func(node *Node, value *machine.CPUsFreqStats) {
				node.CPUsFreqStats = value
			})

			return nil
		},
		func() error {
			resp, err := source.MachineClient.CPUInfo(source.ctx, &emptypb.Empty{})
			if err != nil {
				return err
			}

			unpack(source, result.Nodes, &resultLock, resp, func(node *Node, value *machine.CPUsInfo) {
				node.CPUsInfo = value
			})

			return nil
		},
		func() error {
			resp, err := source.MachineClient.NetworkDeviceStats(source.ctx, &emptypb.Empty{})
			if err != nil {
				return err
			}

			unpack(source, result.Nodes, &resultLock, resp, func(node *Node, value *machine.NetworkDeviceStats) {
				node.NetDevStats = value
			})

			return nil
		},
		func() error {
			resp, err := source.MachineClient.DiskStats(source.ctx, &emptypb.Empty{})
			if err != nil {
				return err
			}

			unpack(source, result.Nodes, &resultLock, resp, func(node *Node, value *machine.DiskStats) {
				node.DiskStats = value
			})

			return nil
		},
		func() error {
			resp, err := source.MachineClient.Processes(source.ctx, &emptypb.Empty{})
			if err != nil {
				return err
			}

			unpack(source, result.Nodes, &resultLock, resp, func(node *Node, value *machine.Process) {
				node.Processes = value
			})

			return nil
		},
		func() error {
			resp, err := source.MachineClient.ServiceList(source.ctx, &emptypb.Empty{})
			if err != nil {
				return err
			}

			unpack(source, result.Nodes, &resultLock, resp, func(node *Node, value *machine.ServiceList) {
				node.ServiceList = value
			})

			return nil
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

func (source *Source) node(msg helpers.Message) string {
	hostname := msg.GetMetadata().GetHostname()

	return source.Resolver.Resolve(hostname)
}
