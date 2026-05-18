// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/internal/kobject"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/internal/trigger"
	machineruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// KernelModuleStatusController presents both built-in and dynamically loaded Linux kernel modules as resources in the runtime.
type KernelModuleStatusController struct {
	V1Alpha1Mode machineruntime.Mode

	ProcModulesPath        string
	ModulesBuiltinFilePath string
	SysModulePath          string

	// This never changes during the lifetime of the controller, so we can cache it here to avoid re-reading modules.builtin on every reconcile loop.
	BuiltinModuleNames []string

	// ReconcileCh triggers an additional reconcile on each receive. Intended for testing only.
	ReconcileCh <-chan struct{}
}

// Name implements controller.Controller interface.
func (ctrl *KernelModuleStatusController) Name() string {
	return "runtime.KernelModuleStatusController"
}

// Inputs implements controller.Controller interface.
func (ctrl *KernelModuleStatusController) Inputs() []controller.Input {
	return nil
}

// Outputs implements controller.Controller interface.
func (ctrl *KernelModuleStatusController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: runtime.LoadedKernelModuleType,
			Kind: controller.OutputExclusive,
		},
		{
			Type: runtime.KernelModuleStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *KernelModuleStatusController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	if ctrl.V1Alpha1Mode.InContainer() {
		// no modules in containers
		return nil
	}

	if ctrl.ProcModulesPath == "" {
		ctrl.ProcModulesPath = constants.ProcModulesPath
	}

	if ctrl.ModulesBuiltinFilePath == "" {
		ctrl.ModulesBuiltinFilePath = constants.ModulesBuiltinPath
	}

	if ctrl.SysModulePath == "" {
		ctrl.SysModulePath = constants.SysModulePath
	}

	// Pre-load built-in modules first
	if err := ctrl.PreloadBuiltinModules(); err != nil {
		return fmt.Errorf("error preloading built-in modules: %w", err)
	}

	// On startup, there's a burst of module load events corresponding to built-in modules.
	// To avoid reconciling the entire module state for each event in that burst, we use a
	// rate-limited trigger to coalesce them into a single reconcile after the burst settles.
	rateLimitedTrigger := *trigger.NewRateLimitedTrigger(ctx, r, 1, 1)

	// Watch for changes to loaded kernel modules through a kobject watcher.
	watcher, err := kobject.NewWatcher(logger)
	if err != nil {
		return fmt.Errorf("failed to create kobject watcher: %w", err)
	}

	watchCh := watcher.Run("module")
	defer watcher.Close() //nolint:errcheck

	return ctrl.runWatchLoop(ctx, r, rateLimitedTrigger, watchCh, watcher.ErrCh())
}

func (ctrl *KernelModuleStatusController) runWatchLoop( //nolint:gocyclo
	ctx context.Context, r controller.Runtime, rateLimitedTrigger trigger.RateLimitedTrigger, watchCh <-chan *kobject.Event, errCh <-chan error,
) error {
	// Initial reconcile to expose resources immediately.
	rateLimitedTrigger.QueueReconcile()

	for {
		select {
		case _, ok := <-watchCh:
			if !ok {
				select {
				case err := <-errCh:
					return fmt.Errorf("error watching for kernel module changes: %w", err)
				default:
					return nil
				}
			}

			rateLimitedTrigger.QueueReconcile()
		case err := <-errCh:
			return fmt.Errorf("error watching for kernel module changes: %w", err)
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
			if err := ctrl.reconcile(ctx, r); err != nil {
				return fmt.Errorf("error reconciling KernelModuleStatus and LoadedKernelModule resources: %w", err)
			}
		// For testing only
		case <-ctrl.ReconcileCh:
			if err := ctrl.reconcile(ctx, r); err != nil {
				return fmt.Errorf("error reconciling KernelModuleStatus and LoadedKernelModule resources: %w", err)
			}
		}
	}
}

func (ctrl *KernelModuleStatusController) reconcile(ctx context.Context, r controller.Runtime) error {
	r.StartTrackingOutputs()

	// These don't change after startup, but must be applied on every reconcile to ensure they are present in the runtime and not deleted by CleanupOutputs.
	if err := ctrl.reconcileBuiltinModules(ctx, r); err != nil {
		return fmt.Errorf("error reconciling built-in modules: %w", err)
	}

	if err := ctrl.reconcileDynamicModules(ctx, r); err != nil {
		return fmt.Errorf("error reconciling dynamic modules: %w", err)
	}

	return r.CleanupOutputs(
		ctx,
		resource.NewMetadata(runtime.NamespaceName, runtime.LoadedKernelModuleType, "", resource.VersionUndefined),
		resource.NewMetadata(runtime.NamespaceName, runtime.KernelModuleStatusType, "", resource.VersionUndefined),
	)
}

func (ctrl *KernelModuleStatusController) reconcileBuiltinModules(ctx context.Context, r controller.Runtime) error {
	for _, name := range ctrl.BuiltinModuleNames {
		var state runtime.KernelModuleState

		if _, err := os.Stat(ctrl.SysModulePath + "/" + name); err == nil {
			// module_init() must've been called for this built-in module, so we can consider it active.
			state = runtime.KernelModuleStateActive
		} else if errors.Is(err, os.ErrNotExist) {
			state = runtime.KernelModuleStateInactive
		} else {
			return fmt.Errorf("error checking if built-in module %s is active: %w", name, err)
		}

		if err := safe.WriterModify(
			ctx, r,
			runtime.NewKernelModuleStatus(runtime.NamespaceName, name),
			func(res *runtime.KernelModuleStatus) error {
				res.TypedSpec().Type = runtime.KernelModuleTypeBuiltin
				res.TypedSpec().State = state

				return nil
			},
		); err != nil {
			return fmt.Errorf("error updating KernelModuleStatus resource for built-in module %s: %w", name, err)
		}
	}

	return nil
}

func (ctrl *KernelModuleStatusController) reconcileDynamicModules(ctx context.Context, r controller.Runtime) error {
	f, err := os.OpenFile(ctrl.ProcModulesPath, os.O_RDONLY, 0)
	if err != nil {
		return fmt.Errorf("error opening %s: %w", ctrl.ProcModulesPath, err)
	}

	defer f.Close() //nolint:errcheck

	rawModules, err := ParseDynamicModules(f)
	if err != nil {
		return fmt.Errorf("error parsing modules from %s: %w", ctrl.ProcModulesPath, err)
	}

	// create a map to track which modules were touched
	for _, module := range rawModules {
		if err := safe.WriterModify(
			ctx, r,
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

		if err := safe.WriterModify(
			ctx, r,
			runtime.NewKernelModuleStatus(runtime.NamespaceName, module.Name),
			func(res *runtime.KernelModuleStatus) error {
				res.TypedSpec().Type = runtime.KernelModuleTypeDynamic
				res.TypedSpec().Size = module.Size
				res.TypedSpec().ReferenceCount = module.ReferenceCount
				res.TypedSpec().Dependencies = module.Dependencies
				res.TypedSpec().State = runtime.ParseDynamicModuleState(module.State)
				res.TypedSpec().Address = module.Address

				return nil
			},
		); err != nil {
			return fmt.Errorf("error updating KernelModuleStatus resource: %w", err)
		}
	}

	return nil
}

// DynamicModule represents a Linux kernel module.
type DynamicModule struct {
	Name           string
	Size           int
	ReferenceCount int
	Dependencies   []string
	State          string
	Address        string
}

// PreloadBuiltinModules reads ModulesBuiltinFilePath and caches the canonical names of all built-in kernel modules.
func (ctrl *KernelModuleStatusController) PreloadBuiltinModules() error {
	f, err := os.Open(ctrl.ModulesBuiltinFilePath)
	if err != nil {
		return fmt.Errorf("error opening %s: %w", ctrl.ModulesBuiltinFilePath, err)
	}

	defer f.Close() //nolint:errcheck

	names, err := ParseBuiltinModuleNames(f)
	if err != nil {
		return fmt.Errorf("error parsing built-in module names from %s: %w", ctrl.ModulesBuiltinFilePath, err)
	}

	ctrl.BuiltinModuleNames = names

	return nil
}

// ParseBuiltinModuleNames parses the contents of a modules.Builtin file from the given reader
// and returns a slice of canonical module names.
//
// Each line in modules.Builtin is a path relative to the kernel module directory, e.g.:
//
//	kernel/arch/x86/crypto/aes-x86_64.ko
//
// The returned names strip the .ko extension and normalize hyphens to underscores, matching
// the convention used by /proc/modules and modprobe.
func ParseBuiltinModuleNames(r io.Reader) ([]string, error) {
	var names []string

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		base := path.Base(line)
		name := strings.TrimSuffix(base, ".ko")
		// Linux normalizes module names: hyphens and underscores are interchangeable.
		name = strings.ReplaceAll(name, "-", "_")

		if name != "" && name != "." {
			names = append(names, name)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error scanning modules.Builtin: %w", err)
	}

	return names, nil
}

// ParseDynamicModules parses the contents of /proc/modules from the given reader
// and returns a slice of module structs representing each loaded kernel module.
func ParseDynamicModules(r io.Reader) ([]DynamicModule, error) {
	var modules []DynamicModule

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

		modules = append(modules, DynamicModule{
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
