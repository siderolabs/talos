// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime/internal/filehash"
	machineruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// LoadedKernelModuleController presents /proc/modules as a resource.
type LoadedKernelModuleController struct {
	V1Alpha1Mode machineruntime.Mode
}

// Name implements controller.Controller interface.
func (ctrl *LoadedKernelModuleController) Name() string {
	return "runtime.LoadedKernelModuleController"
}

// Inputs implements controller.Controller interface.
func (ctrl *LoadedKernelModuleController) Inputs() []controller.Input {
	return nil
}

// Outputs implements controller.Controller interface.
func (ctrl *LoadedKernelModuleController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: runtime.LoadedKernelModuleType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *LoadedKernelModuleController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	if ctrl.V1Alpha1Mode.InContainer() {
		// no modules in containers
		return nil
	}

	watcher, err := filehash.NewWatcher("/proc/modules")
	if err != nil {
		return fmt.Errorf("error creating filehash watcher: %w", err)
	}

	notifyCh, notifyErrCh := watcher.Run()

	defer watcher.Close() //nolint:errcheck

	for {
		select {
		case updatedPath := <-notifyCh:
			if err := ctrl.reconcile(ctx, r, updatedPath); err != nil {
				return fmt.Errorf("error reconciling LoadedKernelModule resources: %w", err)
			}
		case err = <-notifyErrCh:
			return fmt.Errorf("error watching /proc/modules: %w", err)
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}
	}
}

// Module represents a kernel module parsed from /proc/modules.
type Module struct {
	Name           string
	Size           int
	ReferenceCount int
	Dependencies   []string
	State          string
	Address        string
}

// ParseModules parses the contents of /proc/modules from the given reader
// and returns a slice of module structs representing each loaded kernel module.
func ParseModules(r io.Reader) ([]Module, error) {
	var modules []Module

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 6 {
			continue // malformed line
		}

		name := fields[0]

		size, err := strconv.Atoi(fields[1])
		if err != nil {
			return nil, fmt.Errorf("invalid size for module %s: %v", name, err)
		}

		refCount, err := strconv.Atoi(fields[2])
		if err != nil {
			return nil, fmt.Errorf("invalid instance count for module %s: %v", name, err)
		}

		deps := []string{}
		if fields[3] != "-" {
			deps = slices.DeleteFunc(
				strings.Split(fields[3], ","),
				func(s string) bool { return s == "" },
			)
		}

		modules = append(modules, Module{
			Name:           name,
			Size:           size,
			Dependencies:   deps,
			ReferenceCount: refCount,
			State:          fields[4],
			Address:        fields[5],
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error scanning modules: %w", err)
	}

	return modules, nil
}

func (ctrl *LoadedKernelModuleController) reconcile(ctx context.Context, r controller.Runtime, path string) error {
	r.StartTrackingOutputs()

	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return fmt.Errorf("error opening %s: %w", path, err)
	}

	rawModules, err := ParseModules(f)
	if err != nil {
		return fmt.Errorf("error parsing modules from %s: %w", path, err)
	}

	// create a map to track which modules were touched
	for _, module := range rawModules {
		if err := safe.WriterModify(ctx, r,
			runtime.NewLoadedKernelModule(runtime.NamespaceName, module.Name),
			func(res *runtime.LoadedKernelModule) error {
				res.TypedSpec().Size = module.Size
				res.TypedSpec().ReferenceCount = module.ReferenceCount
				res.TypedSpec().Dependencies = module.Dependencies
				res.TypedSpec().State = module.State
				res.TypedSpec().Address = module.Address

				return nil
			},
		); err != nil {
			return fmt.Errorf("error updating LoadedKernelModule resource: %w", err)
		}
	}

	return safe.CleanupOutputs[*runtime.LoadedKernelModule](ctx, r)
}
