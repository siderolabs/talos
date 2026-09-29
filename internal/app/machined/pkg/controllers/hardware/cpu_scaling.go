// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hardware

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/gen/xslices"
	"go.uber.org/zap"
	"k8s.io/utils/cpuset"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/internal/kobject"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/internal/trigger"
	runtimetalos "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

const cpufreqPath = "devices/system/cpu/cpufreq"

// CPUScalingController applies CPUScalingSpec resources to the cpufreq policies under sysfs and
// reports the state of each policy as a CPUScalingStatus.
//
// Applying and reporting live in one controller because the kernel raises no event when a cpufreq
// attribute changes: splitting them would leave the reported state to race the write that caused it.
type CPUScalingController struct {
	V1Alpha1Mode runtimetalos.Mode

	// SysfsPath is the sysfs mount point, defaults to /sys. Overridable for testing.
	SysfsPath string

	// ReconcileCh triggers an additional reconcile on each receive. Intended for testing only.
	ReconcileCh <-chan struct{}

	// defaults holds the value of each attribute before this controller first wrote it.
	defaults map[string]string
}

// Name implements controller.Controller interface.
func (ctrl *CPUScalingController) Name() string {
	return "hardware.CPUScalingController"
}

// Inputs implements controller.Controller interface.
func (ctrl *CPUScalingController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: hardware.NamespaceName,
			Type:      hardware.CPUScalingSpecType,
			Kind:      controller.InputWeak,
		},
		// `machine.sysfs`/SysfsConfig can set a governor too, via KernelParamSpecController; watching
		// its output keeps the reported state from going stale after such a write.
		{
			Namespace: runtime.NamespaceName,
			Type:      runtime.KernelParamStatusType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *CPUScalingController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: hardware.CPUScalingStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *CPUScalingController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	if ctrl.V1Alpha1Mode.InContainer() {
		// cpufreq under /sys belongs to the host, and is read-only for a container
		return nil
	}

	if ctrl.SysfsPath == "" {
		ctrl.SysfsPath = "/sys"
	}

	if ctrl.defaults == nil {
		ctrl.defaults = map[string]string{}
	}

	// a hotplugged socket raises one event per logical CPU, so coalesce the burst
	rateLimitedTrigger := *trigger.NewRateLimitedTrigger(ctx, r, 1, 1)

	watcher, err := kobject.NewWatcher(logger)
	if err != nil {
		return fmt.Errorf("failed to create kobject watcher: %w", err)
	}

	watchCh := watcher.Run("cpu")
	errCh := watcher.ErrCh()

	defer watcher.Close() //nolint:errcheck

	return ctrl.runWatchLoop(ctx, r, logger, rateLimitedTrigger, watchCh, errCh)
}

func (ctrl *CPUScalingController) runWatchLoop( //nolint:gocyclo
	ctx context.Context, r controller.Runtime, logger *zap.Logger,
	rateLimitedTrigger trigger.RateLimitedTrigger, watchCh <-chan *kobject.Event, errCh <-chan error,
) error {
	rateLimitedTrigger.QueueReconcile()

	for {
		select {
		case _, ok := <-watchCh:
			if !ok {
				select {
				case err := <-errCh:
					return fmt.Errorf("error watching for CPU changes: %w", err)
				default:
					return nil
				}
			}

			rateLimitedTrigger.QueueReconcile()
		case err := <-errCh:
			return fmt.Errorf("error watching for CPU changes: %w", err)
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
			if err := ctrl.reconcile(ctx, r, logger); err != nil {
				return err
			}
		case <-ctrl.ReconcileCh:
			if err := ctrl.reconcile(ctx, r, logger); err != nil {
				return err
			}
		}
	}
}

func (ctrl *CPUScalingController) reconcile(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	if err := ctrl.applySpecs(ctx, r, logger); err != nil {
		return err
	}

	return ctrl.reportStatuses(ctx, r)
}

// policies lists the cpufreq policy directories. A machine with no cpufreq driver (most VMs) has
// none, which is not an error.
func (ctrl *CPUScalingController) policies() ([]string, error) {
	return filepath.Glob(filepath.Join(ctrl.SysfsPath, cpufreqPath, "policy*"))
}

func (ctrl *CPUScalingController) reportStatuses(ctx context.Context, r controller.Runtime) error {
	dirs, err := ctrl.policies()
	if err != nil {
		return fmt.Errorf("error listing cpufreq policies: %w", err)
	}

	coreTypes, err := ctrl.readCoreTypes()
	if err != nil {
		return fmt.Errorf("error reading CPU core types: %w", err)
	}

	r.StartTrackingOutputs()

	for _, dir := range dirs {
		id := filepath.Base(dir)

		spec, err := ctrl.readPolicy(dir, coreTypes)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}

			return fmt.Errorf("error reading cpufreq policy %q: %w", id, err)
		}

		if err := safe.WriterModify(ctx, r, hardware.NewCPUScalingStatus(id), func(res *hardware.CPUScalingStatus) error {
			*res.TypedSpec() = spec

			return nil
		}); err != nil {
			return fmt.Errorf("error updating CPUScalingStatus resource %q: %w", id, err)
		}
	}

	return safe.CleanupOutputs[*hardware.CPUScalingStatus](ctx, r)
}

// readPolicy reads a single cpufreq policy directory.
//
// Drivers differ in which attributes they expose, so a missing one leaves its field zero.
func (ctrl *CPUScalingController) readPolicy(dir string, coreTypes map[uint32]string) (hardware.CPUScalingStatusSpec, error) {
	var spec hardware.CPUScalingStatusSpec

	// the cpufreq core creates scaling_driver for every policy, so its absence means there is no
	// policy here (any more)
	driver, err := readAttr(dir, "scaling_driver")
	if err != nil {
		return spec, err
	}

	if driver == "" {
		return spec, fs.ErrNotExist
	}

	spec.Driver = driver

	if err = firstError(
		readInto(dir, "affected_cpus", readCPUList, &spec.AffectedCPUs),
		readInto(dir, "related_cpus", readCPUList, &spec.RelatedCPUs),
		readInto(dir, "scaling_available_governors", readFields, &spec.AvailableGovernors),
		readInto(dir, "energy_performance_available_preferences", readFields, &spec.AvailableEPPs),
		readInto(dir, "scaling_governor", readAttr, &spec.Governor),
		readInto(dir, "energy_performance_preference", readAttr, &spec.EnergyPerformancePreference),
		readInto(dir, "cpuinfo_min_freq", readUintAttr, &spec.CPUInfoMinFrequencyKhz),
		readInto(dir, "cpuinfo_max_freq", readUintAttr, &spec.CPUInfoMaxFrequencyKhz),
		readInto(dir, "base_frequency", readUintAttr, &spec.BaseFrequencyKhz),
		readInto(dir, "scaling_min_freq", readUintAttr, &spec.ScalingMinFrequencyKhz),
		readInto(dir, "scaling_max_freq", readUintAttr, &spec.ScalingMaxFrequencyKhz),
	); err != nil {
		return spec, err
	}

	// an offline policy has no affected CPUs, so describe the hardware through the related ones
	cpus := spec.AffectedCPUs
	if len(cpus) == 0 {
		cpus = spec.RelatedCPUs
	}

	if len(cpus) > 0 {
		spec.CoreType = coreTypes[cpus[0]]

		capacity, err := readUintAttr(filepath.Join(ctrl.SysfsPath, "devices/system/cpu", fmt.Sprintf("cpu%d", cpus[0])), "cpu_capacity")
		if err != nil {
			return spec, err
		}

		spec.CPUCapacity = uint32(capacity)
	}

	return spec, nil
}

// readCoreTypes maps logical CPUs to a core type on machines with a hybrid topology, and is empty
// on uniform ones.
func (ctrl *CPUScalingController) readCoreTypes() (map[uint32]string, error) {
	coreTypes := map[uint32]string{}

	for _, source := range []struct {
		path     string
		coreType string
	}{
		{"devices/cpu_core/cpus", hardware.CoreTypePerformance},
		{"devices/cpu_atom/cpus", hardware.CoreTypeEfficiency},
		{"devices/system/cpu/types/intel_core/cpulist", hardware.CoreTypePerformance},
		{"devices/system/cpu/types/intel_atom/cpulist", hardware.CoreTypeEfficiency},
	} {
		cpus, err := readCPUList(filepath.Join(ctrl.SysfsPath, filepath.Dir(source.path)), filepath.Base(source.path))
		if err != nil {
			return nil, err
		}

		for _, cpu := range cpus {
			coreTypes[cpu] = source.coreType
		}
	}

	return coreTypes, nil
}

func (ctrl *CPUScalingController) applySpecs(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	specs, err := safe.ReaderListAll[*hardware.CPUScalingSpec](ctx, r)
	if err != nil {
		return fmt.Errorf("error listing CPUScalingSpec resources: %w", err)
	}

	desired := map[string]struct{}{}

	for spec := range specs.All() {
		id := spec.Metadata().ID()

		for _, attr := range requestedAttributes(spec.TypedSpec()) {
			desired[id+"/"+attr.name] = struct{}{}
		}

		if err = ctrl.apply(id, spec.TypedSpec(), logger); err != nil {
			return fmt.Errorf("error applying CPUScalingSpec %q: %w", id, err)
		}
	}

	// tracked per attribute, so dropping one field from a config restores only that attribute
	for key := range ctrl.defaults {
		if _, ok := desired[key]; ok {
			continue
		}

		if err = ctrl.restore(key); err != nil {
			return fmt.Errorf("error restoring cpufreq attribute %q: %w", key, err)
		}
	}

	return nil
}

// cpufreqAttribute is a sysfs attribute a spec asks to be set, with the attribute listing the
// values the driver accepts for it, if any.
type cpufreqAttribute struct {
	name      string
	value     string
	available string
}

func requestedAttributes(spec *hardware.CPUScalingSpecSpec) []cpufreqAttribute {
	return xslices.Filter([]cpufreqAttribute{
		{name: "scaling_min_freq", value: formatFreq(spec.MinFrequencyKhz)},
		{name: "scaling_max_freq", value: formatFreq(spec.MaxFrequencyKhz)},
		{name: "scaling_governor", value: spec.Governor, available: "scaling_available_governors"},
		{name: "energy_performance_preference", value: spec.EnergyPerformancePreference, available: "energy_performance_available_preferences"},
	}, func(attr cpufreqAttribute) bool { return attr.value != "" })
}

func (ctrl *CPUScalingController) apply(policy string, spec *hardware.CPUScalingSpecSpec, logger *zap.Logger) error {
	dir := filepath.Join(ctrl.SysfsPath, cpufreqPath, policy)

	attrs := requestedAttributes(spec)

	// The kernel clamps scaling_min_freq against the current scaling_max_freq and vice versa, so a
	// window which does not overlap the old one cannot be set in one pass. Repeating the first
	// attribute converges whether the window moves up or down.
	if len(attrs) > 1 && attrs[0].name == "scaling_min_freq" && attrs[1].name == "scaling_max_freq" {
		attrs = append(attrs, attrs[0])
	}

	for _, attr := range attrs {
		if attr.available != "" {
			supported, err := ctrl.supports(dir, attr.available, attr.value)
			if err != nil {
				return err
			}

			if !supported {
				// a configuration written for different hardware should not wedge the controller
				logger.Warn("cpufreq value not supported by the driver, skipping",
					zap.String("policy", policy),
					zap.String("attribute", attr.name),
					zap.String("value", attr.value),
				)

				continue
			}
		}

		if err := ctrl.write(dir, policy, attr.name, attr.value); err != nil {
			return err
		}
	}

	return nil
}

func (ctrl *CPUScalingController) supports(dir, availableAttr, value string) (bool, error) {
	available, err := readFields(dir, availableAttr)
	if err != nil {
		return false, err
	}

	// a driver which publishes no list accepts anything it understands
	return available == nil || slices.Contains(available, value), nil
}

// write sets a sysfs attribute, remembering its previous value the first time it is touched.
func (ctrl *CPUScalingController) write(dir, policy, name, value string) error {
	key := policy + "/" + name

	if _, ok := ctrl.defaults[key]; !ok {
		current, err := readAttr(dir, name)
		if err != nil {
			return err
		}

		if current == "" {
			// the driver does not expose this attribute
			return nil
		}

		ctrl.defaults[key] = current
	}

	if err := os.WriteFile(filepath.Join(dir, name), []byte(value), 0o644); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			delete(ctrl.defaults, key)

			return nil
		}

		return fmt.Errorf("error writing %q: %w", filepath.Join(dir, name), err)
	}

	return nil
}

func (ctrl *CPUScalingController) restore(key string) error {
	policy, name, _ := strings.Cut(key, "/")

	path := filepath.Join(ctrl.SysfsPath, cpufreqPath, policy, name)

	if err := os.WriteFile(path, []byte(ctrl.defaults[key]), 0o644); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("error writing %q: %w", path, err)
	}

	delete(ctrl.defaults, key)

	return nil
}

// readInto reads an attribute with read and stores it in dest.
func readInto[T any](dir, name string, read func(dir, name string) (T, error), dest *T) func() error {
	return func() error {
		value, err := read(dir, name)
		if err != nil {
			return err
		}

		*dest = value

		return nil
	}
}

func firstError(fns ...func() error) error {
	for _, fn := range fns {
		if err := fn(); err != nil {
			return err
		}
	}

	return nil
}

// readAttr reads a sysfs attribute, returning an empty string when the driver does not expose it.
func readAttr(dir, name string) (string, error) {
	contents, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", nil
		}

		return "", err
	}

	return strings.TrimSpace(string(contents)), nil
}

func readFields(dir, name string) ([]string, error) {
	value, err := readAttr(dir, name)
	if err != nil || value == "" {
		return nil, err
	}

	return strings.Fields(value), nil
}

func readUintAttr(dir, name string) (uint64, error) {
	value, err := readAttr(dir, name)
	if err != nil || value == "" {
		return 0, err
	}

	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("error parsing %q: %w", filepath.Join(dir, name), err)
	}

	return parsed, nil
}

// readCPUList reads an attribute in Linux CPU list format, e.g. `0-5,34`.
func readCPUList(dir, name string) ([]uint32, error) {
	value, err := readAttr(dir, name)
	if err != nil || value == "" {
		return nil, err
	}

	set, err := cpuset.Parse(value)
	if err != nil {
		return nil, fmt.Errorf("error parsing %q: %w", filepath.Join(dir, name), err)
	}

	return xslices.Map(set.List(), func(cpu int) uint32 { return uint32(cpu) }), nil
}

func formatFreq(khz uint64) string {
	if khz == 0 {
		return ""
	}

	return strconv.FormatUint(khz, 10)
}
