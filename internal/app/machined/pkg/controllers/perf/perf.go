// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package perf

import (
	"context"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/prometheus/procfs"
	"go.uber.org/zap"

	perfadapter "github.com/siderolabs/talos/internal/app/machined/pkg/adapters/perf"
	"github.com/siderolabs/talos/pkg/machinery/resources/perf"
)

const updateInterval = time.Second * 30

// StatsController manages v1alpha1.Stats which is the current snaphot of the machine CPU and Memory consumption.
type StatsController struct{}

// Name implements controller.StatsController interface.
func (ctrl *StatsController) Name() string {
	return "perf.StatsController"
}

// Inputs implements controller.StatsController interface.
func (ctrl *StatsController) Inputs() []controller.Input {
	return nil
}

// Outputs implements controller.StatsController interface.
func (ctrl *StatsController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: perf.CPUType,
			Kind: controller.OutputExclusive,
		},
		{
			Type: perf.MemoryType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.StatsController interface.
func (ctrl *StatsController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	ticker := time.NewTicker(updateInterval)

	defer ticker.Stop()

	var (
		fs  procfs.FS
		err error
	)

	fs, err = procfs.NewDefaultFS()
	if err != nil {
		return err
	}

	for {
		select {
		case <-r.EventCh():
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}

		if err := ctrl.updateMemory(ctx, r, &fs); err != nil {
			return err
		}

		if err := ctrl.updateCPU(ctx, r, &fs); err != nil {
			return err
		}

		r.ResetRestartBackoff()
	}
}

func (ctrl *StatsController) updateCPU(ctx context.Context, r controller.Runtime, fs *procfs.FS) error {
	cpu := perf.NewCPU()

	stat, err := fs.Stat()
	if err != nil {
		return err
	}

	return r.Modify(ctx, cpu, func(r resource.Resource) error {
		perfadapter.CPU(r.(*perf.CPU)).Update(&stat)

		return nil
	})
}

func (ctrl *StatsController) updateMemory(ctx context.Context, r controller.Runtime, fs *procfs.FS) error {
	mem := perf.NewMemory()

	info, err := fs.Meminfo()
	if err != nil {
		return err
	}

	return r.Modify(ctx, mem, func(r resource.Resource) error {
		perfadapter.Memory(r.(*perf.Memory)).Update(&info)

		return nil
	})
}
