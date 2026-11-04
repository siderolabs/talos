// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"
	"k8s.io/utils/cpuset"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime/internal/cpupartition"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

const (
	partitionMember = "member"
	partitionRoot   = "root"
)

// CPUPartitionController applies the CPU partition policy of the boot once.
//
// It sets every bounded cgroup, verifies them and publishes a ready CPUPartitionStatus, after which
// it never touches the cgroups again: the masks are not watched or repaired. A failed setup
// publishes a status with the error and is retried until it first succeeds.
// Without a policy, which container mode never has, the status is ready without any cgroup access.
//
// Exclusive slices are remote partition roots: the virtual machine root stays a member and
// delegates their CPUs through its cpuset.cpus.exclusive, every other cgroup stays a member.
type CPUPartitionController struct {
	// FS defaults to the host cgroup and sysfs mounts.
	FS cpupartition.CgroupFS
}

// Name implements controller.Controller interface.
func (ctrl *CPUPartitionController) Name() string {
	return "runtime.CPUPartitionController"
}

// Inputs implements controller.Controller interface.
func (ctrl *CPUPartitionController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: runtime.NamespaceName,
			Type:      runtime.CPUPartitionSpecType,
			ID:        optional.Some(runtime.CPUPartitionSpecID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *CPUPartitionController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: runtime.CPUPartitionStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
// A failed setup returns an error, so the controller runtime logs it and retries with backoff. Once
// ready it returns nil, and the controller runtime never runs it again.
func (ctrl *CPUPartitionController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	if ctrl.FS == nil {
		ctrl.FS = cpupartition.HostFS{CgroupRoot: constants.CgroupMountPath, SysfsPath: "/sys"}
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		ready, err := ctrl.reconcile(ctx, r)
		if err != nil {
			return err
		}

		if ready {
			return nil
		}
	}
}

// reconcile applies the policy of the boot once it is captured and publishes the outcome.
func (ctrl *CPUPartitionController) reconcile(ctx context.Context, r controller.Runtime) (bool, error) {
	spec, err := safe.ReaderGetByID[*runtime.CPUPartitionSpec](ctx, r, runtime.CPUPartitionSpecID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return false, nil
		}

		return false, fmt.Errorf("error getting CPU partition spec: %w", err)
	}

	status, applyErr := ctrl.status(spec.TypedSpec())

	if err = publishCPUPartitionStatus(ctx, r, status); err != nil {
		return false, err
	}

	if applyErr != nil {
		return false, applyErr
	}

	return true, nil
}

// status applies the policy and returns the status to publish.
func (ctrl *CPUPartitionController) status(spec *runtime.CPUPartitionSpecSpec) (runtime.CPUPartitionStatusSpec, error) {
	if len(spec.Roots) == 0 {
		return runtime.CPUPartitionStatusSpec{Ready: true}, nil
	}

	applied, err := ctrl.apply(spec)
	if err == nil {
		return applied, nil
	}

	return runtime.CPUPartitionStatusSpec{Error: err.Error()}, err
}

func (ctrl *CPUPartitionController) apply(spec *runtime.CPUPartitionSpecSpec) (runtime.CPUPartitionStatusSpec, error) {
	online, err := ctrl.FS.Online()
	if err != nil {
		return runtime.CPUPartitionStatusSpec{}, fmt.Errorf("error reading online CPUs: %w", err)
	}

	policy, err := cpupartition.NewPolicy(spec, online)
	if err != nil {
		return runtime.CPUPartitionStatusSpec{}, err
	}

	if err = ctrl.setup(policy); err != nil {
		return runtime.CPUPartitionStatusSpec{}, err
	}

	return policy.Status(), nil
}

// setup writes every cgroup of the boot policy, parents first, then verifies all of them. It is
// idempotent, so a retry after a partial failure resumes towards the same policy.
func (ctrl *CPUPartitionController) setup(policy cpupartition.Policy) error {
	if err := ctrl.preflight(policy); err != nil {
		return err
	}

	for _, mask := range policy.Masks {
		if err := ctrl.write(mask); err != nil {
			return err
		}
	}

	var errs error

	for _, mask := range policy.Masks {
		errs = errors.Join(errs, ctrl.verify(policy.Partitioned, mask))
	}

	return errs
}

// preflight refuses setup over a virtual machine root which already runs tasks: they were not
// started under the policy, and moving them is not done. The populated flag is hierarchical, so
// this covers its slices and any unknown children.
func (ctrl *CPUPartitionController) preflight(policy cpupartition.Policy) error {
	if !slices.ContainsFunc(policy.Masks, func(mask cpupartition.Mask) bool { return mask.Target == cpupartition.VirtualMachinesRoot }) {
		return nil
	}

	// Talos creates the virtual machine root at boot, before any controller runs.
	populated, err := ctrl.FS.Populated(cpupartition.VirtualMachinesRoot.CgroupPath())
	if err != nil {
		return fmt.Errorf("error reading the tasks of %s: %w", cpupartition.VirtualMachinesRoot, err)
	}

	if populated {
		return fmt.Errorf("%s: already runs tasks started before the CPU partition policy", cpupartition.VirtualMachinesRoot)
	}

	return nil
}

// write creates the cgroup and writes its mask, its exclusive CPUs and its partition mode, in that order.
func (ctrl *CPUPartitionController) write(mask cpupartition.Mask) error {
	path := mask.Target.CgroupPath()

	if err := ctrl.FS.Ensure(path); err != nil {
		return fmt.Errorf("error creating %s: %w", mask.Target, err)
	}

	if err := ctrl.FS.SetCPUs(path, mask.CPUs.String()); err != nil {
		return fmt.Errorf("error writing %s: %w", mask.Target, err)
	}

	if !mask.ExclusiveCPUs.IsEmpty() {
		if err := ctrl.FS.SetExclusive(path, mask.ExclusiveCPUs.String()); err != nil {
			return fmt.Errorf("error writing the exclusive CPUs of %s: %w", mask.Target, err)
		}
	}

	if !mask.Exclusive {
		return nil
	}

	// The kernel ignores a write of the current mode and retries an invalid root; verify reads the outcome.
	if err := ctrl.FS.SetPartition(path, partitionRoot); err != nil {
		return fmt.Errorf("error making %s a partition root: %w", mask.Target, err)
	}

	return nil
}

// verify reads back the configured CPUs, the partition mode and exclusive CPUs when an exclusive
// slice makes them matter, and the effective CPUs of the target, each once.
func (ctrl *CPUPartitionController) verify(partitioned bool, mask cpupartition.Mask) error {
	path := mask.Target.CgroupPath()

	type check struct {
		what string
		want cpuset.CPUSet
		read func(string) (cpuset.CPUSet, error)
	}

	checks := []check{{"configured CPUs", mask.CPUs, ctrl.FS.CPUs}}

	if partitioned {
		if err := ctrl.verifyPartition(mask); err != nil {
			return err
		}

		if !mask.ExclusiveCPUs.IsEmpty() {
			checks = append(checks, check{"exclusive CPUs", mask.ExclusiveCPUs, ctrl.FS.Exclusive})
		}

		if mask.Exclusive {
			checks = append(checks, check{"effective exclusive CPUs", mask.ExclusiveCPUs, ctrl.FS.ExclusiveEffective})
		}
	}

	// When exclusive slices take every CPU of the virtual machine root the kernel falls back to the
	// parent's CPUs, which no machine may use.
	if want := mask.Effective(); !want.IsEmpty() {
		checks = append(checks, check{"effective CPUs", want, ctrl.FS.Effective})
	}

	for _, c := range checks {
		actual, err := c.read(path)
		if err != nil {
			return fmt.Errorf("error reading the %s of %s: %w", c.what, mask.Target, err)
		}

		if !actual.Equals(c.want) {
			return fmt.Errorf("%s: %s %q differ from %q", mask.Target, c.what, actual, c.want)
		}
	}

	return nil
}

func (ctrl *CPUPartitionController) verifyPartition(mask cpupartition.Mask) error {
	want := partitionMember
	if mask.Exclusive {
		want = partitionRoot
	}

	mode, err := ctrl.FS.Partition(mask.Target.CgroupPath())
	if err != nil {
		return fmt.Errorf("error reading the partition of %s: %w", mask.Target, err)
	}

	if mode != want {
		return fmt.Errorf("%s: partition %q, expected %q", mask.Target, mode, want)
	}

	return nil
}

func publishCPUPartitionStatus(ctx context.Context, r controller.Runtime, applied runtime.CPUPartitionStatusSpec) error {
	if err := safe.WriterModify(ctx, r, runtime.NewCPUPartitionStatus(), func(res *runtime.CPUPartitionStatus) error {
		*res.TypedSpec() = applied

		return nil
	}); err != nil {
		return fmt.Errorf("error publishing CPU partition status: %w", err)
	}

	return nil
}
