// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dashboard

import (
	"context"
	"sync"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"golang.org/x/sync/errgroup"

	"github.com/talos-systems/talos/cmd/talosctl/cmd/talos/dashboard/data"
	"github.com/talos-systems/talos/pkg/machinery/client"
)

// APISource provides monitoring data via Talos API.
type APISource struct {
	*client.Client

	Interval time.Duration

	ctx       context.Context
	ctxCancel context.CancelFunc

	wg sync.WaitGroup
}

// Run the data poll on interval.
func (source *APISource) Run(ctx context.Context) <-chan *data.Data {
	dataCh := make(chan *data.Data)

	source.ctx, source.ctxCancel = context.WithCancel(ctx)

	source.wg.Add(1)

	go source.run(dataCh)

	return dataCh
}

func (source *APISource) run(dataCh chan<- *data.Data) {
	defer source.wg.Done()
	defer close(dataCh)

	ticker := time.NewTicker(source.Interval)
	defer ticker.Stop()

	var oldData, data *data.Data

	for {
		data = source.gather()

		if oldData == nil {
			data.CalculateDiff(data)
		} else {
			data.CalculateDiff(oldData)
		}

		select {
		case dataCh <- data:
		case <-source.ctx.Done():
			return
		}

		select {
		case <-source.ctx.Done():
			return
		case <-ticker.C:
		}

		oldData = data
	}
}

//nolint:gocyclo,cyclop
func (source *APISource) gather() *data.Data {
	result := &data.Data{
		Timestamp: time.Now(),
		Nodes:     map[string]*data.Node{},
	}

	var resultLock sync.Mutex

	gatherFuncs := []func() error{
		func() error {
			resp, err := source.MachineClient.Hostname(source.ctx, &empty.Empty{})
			if err != nil {
				return err
			}

			resultLock.Lock()
			defer resultLock.Unlock()

			for _, msg := range resp.GetMessages() {
				node := msg.GetMetadata().GetHostname()

				if _, ok := result.Nodes[node]; !ok {
					result.Nodes[node] = &data.Node{}
				}

				result.Nodes[node].Hostname = msg
			}

			return nil
		},
		func() error {
			resp, err := source.MachineClient.LoadAvg(source.ctx, &empty.Empty{})
			if err != nil {
				return err
			}

			resultLock.Lock()
			defer resultLock.Unlock()

			for _, msg := range resp.GetMessages() {
				node := msg.GetMetadata().GetHostname()

				if _, ok := result.Nodes[node]; !ok {
					result.Nodes[node] = &data.Node{}
				}

				result.Nodes[node].LoadAvg = msg
			}

			return nil
		},
		func() error {
			resp, err := source.MachineClient.Version(source.ctx, &empty.Empty{})
			if err != nil {
				return err
			}

			resultLock.Lock()
			defer resultLock.Unlock()

			for _, msg := range resp.GetMessages() {
				node := msg.GetMetadata().GetHostname()

				if _, ok := result.Nodes[node]; !ok {
					result.Nodes[node] = &data.Node{}
				}

				result.Nodes[node].Version = msg
			}

			return nil
		},
		func() error {
			resp, err := source.MachineClient.Memory(source.ctx, &empty.Empty{})
			if err != nil {
				return err
			}

			resultLock.Lock()
			defer resultLock.Unlock()

			for _, msg := range resp.GetMessages() {
				node := msg.GetMetadata().GetHostname()

				if _, ok := result.Nodes[node]; !ok {
					result.Nodes[node] = &data.Node{}
				}

				result.Nodes[node].Memory = msg
			}

			return nil
		},
		func() error {
			resp, err := source.MachineClient.SystemStat(source.ctx, &empty.Empty{})
			if err != nil {
				return err
			}

			resultLock.Lock()
			defer resultLock.Unlock()

			for _, msg := range resp.GetMessages() {
				node := msg.GetMetadata().GetHostname()

				if _, ok := result.Nodes[node]; !ok {
					result.Nodes[node] = &data.Node{}
				}

				result.Nodes[node].SystemStat = msg
			}

			return nil
		},
		func() error {
			resp, err := source.MachineClient.CPUInfo(source.ctx, &empty.Empty{})
			if err != nil {
				return err
			}

			resultLock.Lock()
			defer resultLock.Unlock()

			for _, msg := range resp.GetMessages() {
				node := msg.GetMetadata().GetHostname()

				if _, ok := result.Nodes[node]; !ok {
					result.Nodes[node] = &data.Node{}
				}

				result.Nodes[node].CPUsInfo = msg
			}

			return nil
		},
		func() error {
			resp, err := source.MachineClient.NetworkDeviceStats(source.ctx, &empty.Empty{})
			if err != nil {
				return err
			}

			resultLock.Lock()
			defer resultLock.Unlock()

			for _, msg := range resp.GetMessages() {
				node := msg.GetMetadata().GetHostname()

				if _, ok := result.Nodes[node]; !ok {
					result.Nodes[node] = &data.Node{}
				}

				result.Nodes[node].NetDevStats = msg
			}

			return nil
		},
		func() error {
			resp, err := source.MachineClient.DiskStats(source.ctx, &empty.Empty{})
			if err != nil {
				return err
			}

			resultLock.Lock()
			defer resultLock.Unlock()

			for _, msg := range resp.GetMessages() {
				node := msg.GetMetadata().GetHostname()

				if _, ok := result.Nodes[node]; !ok {
					result.Nodes[node] = &data.Node{}
				}

				result.Nodes[node].DiskStats = msg
			}

			return nil
		},
		func() error {
			resp, err := source.MachineClient.Processes(source.ctx, &empty.Empty{})
			if err != nil {
				return err
			}

			resultLock.Lock()
			defer resultLock.Unlock()

			for _, msg := range resp.GetMessages() {
				node := msg.GetMetadata().GetHostname()

				if _, ok := result.Nodes[node]; !ok {
					result.Nodes[node] = &data.Node{}
				}

				result.Nodes[node].Processes = msg
			}

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

// Stop the data collection process.
func (source *APISource) Stop() {
	source.ctxCancel()

	source.wg.Wait()
}
