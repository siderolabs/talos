// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package datasource

import (
	"context"
	"sync"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/siderolabs/talos/internal/pkg/dashboard/data"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/resources/cluster"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// API is a data source that gathers information about a Talos node using Talos API.
type API struct {
	*client.Client

	Interval time.Duration

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc

	wg sync.WaitGroup
}

// Run the data poll on interval.
func (source *API) Run(ctx context.Context) <-chan *data.Data {
	dataCh := make(chan *data.Data)

	source.ctx, source.ctxCancel = context.WithCancel(ctx)

	source.wg.Add(1)

	go source.run(dataCh)

	return dataCh
}

func (source *API) run(dataCh chan<- *data.Data) {
	defer source.wg.Done()
	defer close(dataCh)

	ticker := time.NewTicker(source.Interval)
	defer ticker.Stop()

	var oldData, currentData *data.Data

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

//nolint:gocyclo,cyclop
func (source *API) gather() *data.Data {
	result := &data.Data{
		Timestamp: time.Now(),
		Nodes:     map[string]*data.Node{},
	}

	hostnames, _ := source.hostnames() //nolint:errcheck

	for node, hostname := range hostnames {
		result.Nodes[node] = &data.Node{
			Hostname: hostname,
		}
	}

	var resultLock sync.Mutex

	gatherFuncs := []func() error{
		func() error {
			resp, err := source.MachineClient.LoadAvg(source.ctx, &emptypb.Empty{})
			if err != nil {
				return err
			}

			resultLock.Lock()
			defer resultLock.Unlock()

			for _, msg := range resp.GetMessages() {
				node := msg.GetMetadata().GetHostname()

				result.Nodes[node].LoadAvg = msg
			}

			return nil
		},
		func() error {
			resp, err := source.MachineClient.Version(source.ctx, &emptypb.Empty{})
			if err != nil {
				return err
			}

			resultLock.Lock()
			defer resultLock.Unlock()

			for _, msg := range resp.GetMessages() {
				node := msg.GetMetadata().GetHostname()

				result.Nodes[node].Version = msg
			}

			return nil
		},
		func() error {
			resp, err := source.MachineClient.Memory(source.ctx, &emptypb.Empty{})
			if err != nil {
				return err
			}

			resultLock.Lock()
			defer resultLock.Unlock()

			for _, msg := range resp.GetMessages() {
				node := msg.GetMetadata().GetHostname()

				result.Nodes[node].Memory = msg
			}

			return nil
		},
		func() error {
			resp, err := source.MachineClient.SystemStat(source.ctx, &emptypb.Empty{})
			if err != nil {
				return err
			}

			resultLock.Lock()
			defer resultLock.Unlock()

			for _, msg := range resp.GetMessages() {
				node := msg.GetMetadata().GetHostname()

				result.Nodes[node].SystemStat = msg
			}

			return nil
		},
		func() error {
			resp, err := source.MachineClient.CPUInfo(source.ctx, &emptypb.Empty{})
			if err != nil {
				return err
			}

			resultLock.Lock()
			defer resultLock.Unlock()

			for _, msg := range resp.GetMessages() {
				node := msg.GetMetadata().GetHostname()

				result.Nodes[node].CPUsInfo = msg
			}

			return nil
		},
		func() error {
			resp, err := source.MachineClient.NetworkDeviceStats(source.ctx, &emptypb.Empty{})
			if err != nil {
				return err
			}

			resultLock.Lock()
			defer resultLock.Unlock()

			for _, msg := range resp.GetMessages() {
				node := msg.GetMetadata().GetHostname()

				result.Nodes[node].NetDevStats = msg
			}

			return nil
		},
		func() error {
			resp, err := source.MachineClient.DiskStats(source.ctx, &emptypb.Empty{})
			if err != nil {
				return err
			}

			resultLock.Lock()
			defer resultLock.Unlock()

			for _, msg := range resp.GetMessages() {
				node := msg.GetMetadata().GetHostname()

				result.Nodes[node].DiskStats = msg
			}

			return nil
		},
		func() error {
			resp, err := source.MachineClient.Processes(source.ctx, &emptypb.Empty{})
			if err != nil {
				return err
			}

			resultLock.Lock()
			defer resultLock.Unlock()

			for _, msg := range resp.GetMessages() {
				node := msg.GetMetadata().GetHostname()

				result.Nodes[node].Processes = msg
			}

			return nil
		},
		func() error {
			resp, err := source.MachineClient.ServiceList(source.ctx, &emptypb.Empty{})
			if err != nil {
				return err
			}

			resultLock.Lock()
			defer resultLock.Unlock()

			for _, msg := range resp.GetMessages() {
				node := msg.GetMetadata().GetHostname()

				result.Nodes[node].ServiceList = msg
			}

			return nil
		},
	}

	// TODO(dashboard): replace gets/lists with watches to avoid excessive requests (probably create a new data source for resources)
	for node := range hostnames {
		gatherFuncs = append(gatherFuncs,
			resourceGetFunc(source, node, &resultLock,
				runtime.NewMachineStatus(),
				&result.Nodes[node].MachineStatus),
			resourceGetFunc(source, node, &resultLock,
				config.NewMachineType(),
				&result.Nodes[node].MachineType),
			resourceGetFunc(source, node, &resultLock,
				k8s.NewKubeletSpec(k8s.NamespaceName, k8s.KubeletID),
				&result.Nodes[node].KubeletSpec),
			resourceGetFunc(source, node, &resultLock,
				network.NewResolverStatus(network.NamespaceName, network.ResolverID),
				&result.Nodes[node].ResolverStatus),
			resourceGetFunc(source, node, &resultLock,
				network.NewTimeServerStatus(network.NamespaceName, network.TimeServerID),
				&result.Nodes[node].TimeServerStatus),
			resourceGetFunc(source, node, &resultLock,
				hardware.NewSystemInformation(hardware.SystemInformationID),
				&result.Nodes[node].SystemInformation),
			resourceGetFunc(source, node, &resultLock,
				cluster.NewInfo(),
				&result.Nodes[node].ClusterInfo),
			resourceListFunc(source, node, &resultLock,
				k8s.NewStaticPodStatus(k8s.NamespaceName, ""),
				&result.Nodes[node].StaticPodStatuses),
			resourceListFunc(source, node, &resultLock,
				network.NewRouteStatus(network.NamespaceName, ""),
				&result.Nodes[node].RouteStatuses),
			resourceListFunc(source, node, &resultLock,
				network.NewLinkStatus(network.NamespaceName, ""),
				&result.Nodes[node].LinkStatuses),
			resourceListFunc(source, node, &resultLock,
				cluster.NewMember(cluster.NamespaceName, ""),
				&result.Nodes[node].Members),
		)
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

func (source *API) hostnames() (map[string]*machine.Hostname, error) {
	resp, err := source.MachineClient.Hostname(source.ctx, &emptypb.Empty{})
	if err != nil {
		return nil, err
	}

	result := make(map[string]*machine.Hostname, len(resp.GetMessages()))

	for _, msg := range resp.GetMessages() {
		node := msg.GetMetadata().GetHostname()

		result[node] = msg
	}

	return result, nil
}

func resourceGetFunc[T resource.Resource](source *API, node string, lock *sync.Mutex, ref T, field *T) func() error {
	return func() error {
		ctx := source.ctx

		// node might be empty if the client is using the machined socket directly (e.g. vty dashboard)
		if node != "" {
			ctx = client.WithNode(source.ctx, node)
		}

		res, err := safe.StateGet[T](ctx, source.COSI, ref.Metadata())
		if err != nil {
			return err
		}

		lock.Lock()
		defer lock.Unlock()

		*field = res

		return nil
	}
}

func resourceListFunc[T resource.Resource](source *API, node string, lock *sync.Mutex, ref T, field *[]T) func() error {
	return func() error {
		ctx := source.ctx

		// node might be empty if the client is using the machined socket directly (e.g. vty dashboard)
		if node != "" {
			ctx = client.WithNode(source.ctx, node)
		}

		list, err := safe.StateList[T](ctx, source.COSI, ref.Metadata())
		if err != nil {
			return err
		}

		result := make([]T, 0, list.Len())

		for iter := safe.IteratorFromList(list); iter.Next(); {
			result = append(result, iter.Value())
		}

		lock.Lock()
		defer lock.Unlock()

		*field = result

		return nil
	}
}

// Stop the data collection process.
func (source *API) Stop() {
	source.ctxCancel()

	source.wg.Wait()
}
