// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/gen/xslices"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	"k8s.io/utils/cpuset"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	runtimectrls "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime"
	machineruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	runtimecfg "github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// fakeCgroupFS is an in-memory cgroup v2 tree: a cgroup's effective set is its mask intersected
// with its parent's effective set, falling back to the parent's when empty. A child set to
// partition "root" takes its exclusive CPUs out of its parent's effective set; it is not a model
// of the kernel's validity rules, which are injected through partition states and overrides.
type fakeCgroupFS struct {
	mu        sync.Mutex
	online    cpuset.CPUSet
	onlineErr error
	cpus      map[string]string
	exclusive map[string]string
	partition map[string]string
	populated map[string]bool
	writeErr  map[string]error
	// rejected is the mode the kernel leaves when a partition root write is refused.
	rejected map[string]string
	// effective overrides the computed effective set.
	effective map[string]string
	writes    []string
	calls     int
}

func newFakeCgroupFS() *fakeCgroupFS {
	fs := &fakeCgroupFS{
		online:    cpuset.New(0, 1, 2, 3, 4, 5, 6, 7),
		cpus:      map[string]string{},
		exclusive: map[string]string{},
		partition: map[string]string{},
		populated: map[string]bool{},
		writeErr:  map[string]error{},
		rejected:  map[string]string{},
		effective: map[string]string{},
	}

	for _, root := range []string{"init", "system", "podruntime", "taloscontainers", "virtualmachines.partition"} {
		fs.cpus[root] = ""
	}

	return fs
}

func (fs *fakeCgroupFS) lock() func() {
	fs.mu.Lock()
	fs.calls++

	return fs.mu.Unlock
}

func (fs *fakeCgroupFS) Online() (cpuset.CPUSet, error) {
	defer fs.lock()()

	return fs.online, fs.onlineErr
}

func (fs *fakeCgroupFS) Ensure(path string) error {
	defer fs.lock()()

	if _, ok := fs.cpus[path]; !ok {
		fs.cpus[path] = ""
		fs.writes = append(fs.writes, "mkdir "+path)
	}

	return nil
}

func (fs *fakeCgroupFS) CPUs(path string) (cpuset.CPUSet, error) {
	defer fs.lock()()

	value, ok := fs.cpus[path]
	if !ok {
		return cpuset.New(), errors.New("no such cgroup")
	}

	return mustCPUs(value), nil
}

func (fs *fakeCgroupFS) effectiveLocked(path string) cpuset.CPUSet {
	if fs.partition[path] == "root" {
		return mustCPUs(fs.exclusive[path])
	}

	parent := fs.online

	if dir, _, ok := strings.CutLast(path, "/"); ok {
		parent = fs.effectiveLocked(dir)
	}

	set := mustCPUs(fs.cpus[path]).Intersection(parent)

	for child, mode := range fs.partition {
		if dir, _, ok := strings.CutLast(child, "/"); ok && dir == path && mode == "root" {
			set = set.Difference(mustCPUs(fs.exclusive[child]))
		}
	}

	if !set.IsEmpty() {
		return set
	}

	return parent
}

func mustCPUs(list string) cpuset.CPUSet {
	set, err := cpuset.Parse(list)
	if err != nil {
		panic(err)
	}

	return set
}

func (fs *fakeCgroupFS) Effective(path string) (cpuset.CPUSet, error) {
	defer fs.lock()()

	if _, ok := fs.cpus[path]; !ok {
		return cpuset.New(), errors.New("no such cgroup")
	}

	if override, ok := fs.effective[path]; ok {
		return cpuset.Parse(override)
	}

	return fs.effectiveLocked(path), nil
}

func (fs *fakeCgroupFS) SetCPUs(path, cpus string) error {
	defer fs.lock()()

	if err := fs.writeErr[path]; err != nil {
		return err
	}

	if _, ok := fs.cpus[path]; !ok {
		return errors.New("no such cgroup")
	}

	fs.cpus[path] = cpus
	fs.writes = append(fs.writes, path+"="+cpus)

	return nil
}

// Populated is hierarchical, like cgroup.events: tasks anywhere below the cgroup count.
func (fs *fakeCgroupFS) Populated(path string) (bool, error) {
	defer fs.lock()()

	for cgroup, populated := range fs.populated {
		if populated && (cgroup == path || strings.HasPrefix(cgroup, path+"/")) {
			return true, nil
		}
	}

	return false, nil
}

func (fs *fakeCgroupFS) Partition(path string) (string, error) {
	defer fs.lock()()

	if _, ok := fs.cpus[path]; !ok {
		return "", errors.New("no such cgroup")
	}

	if mode, ok := fs.partition[path]; ok {
		return mode, nil
	}

	return "member", nil
}

func (fs *fakeCgroupFS) SetPartition(path, partition string) error {
	defer fs.lock()()

	if err := fs.writeErr[path+" partition"]; err != nil {
		return err
	}

	fs.writes = append(fs.writes, path+" partition="+partition)

	if mode, ok := fs.rejected[path]; ok {
		partition = mode
	}

	fs.partition[path] = partition

	return nil
}

func (fs *fakeCgroupFS) Exclusive(path string) (cpuset.CPUSet, error) {
	defer fs.lock()()

	if _, ok := fs.cpus[path]; !ok {
		return cpuset.New(), errors.New("no such cgroup")
	}

	return mustCPUs(fs.exclusive[path]), nil
}

func (fs *fakeCgroupFS) SetExclusive(path, cpus string) error {
	defer fs.lock()()

	if err := fs.writeErr[path+" exclusive"]; err != nil {
		return err
	}

	fs.exclusive[path] = cpus
	fs.writes = append(fs.writes, path+" exclusive="+cpus)

	return nil
}

func (fs *fakeCgroupFS) ExclusiveEffective(path string) (cpuset.CPUSet, error) {
	defer fs.lock()()

	if fs.partition[path] != "root" {
		return cpuset.New(), nil
	}

	return mustCPUs(fs.exclusive[path]), nil
}

func (fs *fakeCgroupFS) do(fn func(fs *fakeCgroupFS)) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	fn(fs)
}

func (fs *fakeCgroupFS) mask(path string) (string, bool) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	value, ok := fs.cpus[path]

	return value, ok
}

func (fs *fakeCgroupFS) recordedWrites() []string {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	return slices.Clone(fs.writes)
}

func (fs *fakeCgroupFS) callCount() int {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	return fs.calls
}

func ordinaryPolicy() runtime.CPUPartitionSpecSpec {
	return runtime.CPUPartitionSpecSpec{
		Roots: map[string]string{
			"init":            "0-1",
			"system":          "0-1",
			"podruntime":      "0-1",
			"kubepods":        "2-3",
			"taloscontainers": "0-1",
			"virtualmachines": "4-7",
		},
		Slices: []runtime.CPUPartitionSliceSpec{
			{Name: "batch", CPUs: "6-7"},
		},
	}
}

var ordinaryMasks = map[string]string{
	"init":                      "0-1",
	"system":                    "0-1",
	"podruntime":                "0-1",
	"kubepods":                  "2-3",
	"taloscontainers":           "0-1",
	"virtualmachines.partition": "4-7",
	"virtualmachines.partition/shared.partition": "4-5",
	"virtualmachines.partition/batch.partition":  "6-7",
}

type CPUPartitionSuite struct {
	ctest.DefaultSuite

	fs   *fakeCgroupFS
	logs *observer.ObservedLogs
}

func TestCPUPartitionSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &CPUPartitionSuite{Timeout: 10 * time.Second})
}

func (suite *CPUPartitionSuite) SetupTest() {
	suite.fs = newFakeCgroupFS()

	var core zapcore.Core

	core, suite.logs = observer.New(zapcore.WarnLevel)
	suite.Logger = zap.New(core)

	suite.DefaultSuite.SetupTest()
}

func (suite *CPUPartitionSuite) start() {
	suite.Require().NoError(suite.Runtime().RegisterController(&runtimectrls.CPUPartitionController{
		FS: suite.fs,
	}))
}

func (suite *CPUPartitionSuite) capture(spec runtime.CPUPartitionSpecSpec) {
	res := runtime.NewCPUPartitionSpec()
	*res.TypedSpec() = spec

	suite.Create(res)
}

func (suite *CPUPartitionSuite) assertStatus(expected runtime.CPUPartitionStatusSpec) {
	ctest.AssertResource(suite, runtime.CPUPartitionStatusID, func(res *runtime.CPUPartitionStatus, asrt *assert.Assertions) {
		asrt.Equal(expected, *res.TypedSpec())
	})
}

// assertNotReady checks the status is not ready, and the reason is both published and logged.
func (suite *CPUPartitionSuite) assertNotReady(reason string) {
	ctest.AssertResource(suite, runtime.CPUPartitionStatusID, func(res *runtime.CPUPartitionStatus, asrt *assert.Assertions) {
		asrt.False(res.TypedSpec().Ready)
		asrt.Contains(res.TypedSpec().Error, reason)
	})

	suite.Require().EventuallyWithT(func(collect *assert.CollectT) {
		logged := xslices.Map(suite.logs.FilterMessage("controller failed").All(), func(entry observer.LoggedEntry) string {
			err, _ := entry.ContextMap()["error"].(string) //nolint:errcheck

			return err
		})

		assert.True(collect, slices.ContainsFunc(logged, func(err string) bool { return strings.Contains(err, reason) }), "logged %q", logged)
	}, 5*time.Second, 10*time.Millisecond)
}

func (suite *CPUPartitionSuite) assertMasks(expected map[string]string) {
	for path, cpus := range expected {
		mask, ok := suite.fs.mask(path)
		suite.Require().True(ok, path)
		suite.Assert().Equal(cpus, mask, path)
	}
}

// assertNoFurtherWrites checks that the controller leaves the cgroups alone once applied, whatever
// happens to them or to its inputs afterwards.
func (suite *CPUPartitionSuite) assertNoFurtherWrites() {
	fs := suite.fs
	writes := fs.recordedWrites()

	fs.do(func(fs *fakeCgroupFS) { fs.cpus["kubepods"] = "0-7" })

	spec, err := safe.StateGetByID[*runtime.CPUPartitionSpec](suite.Ctx(), suite.State(), runtime.CPUPartitionSpecID)
	suite.Require().NoError(err)
	suite.Update(spec)

	suite.Require().Never(func() bool {
		return !slices.Equal(writes, fs.recordedWrites())
	}, 300*time.Millisecond, 10*time.Millisecond)
}

func ordinaryPoolsStatus() runtime.CPUPartitionStatusSpec {
	return runtime.CPUPartitionStatusSpec{
		Ready:               true,
		KubeletReservedCPUs: "0-1,4-7",
		VirtualMachinePools: []runtime.CPUPartitionPoolSpec{
			{Name: "shared", CPUs: "4-5", Partition: "/virtualmachines.partition/shared.partition"},
			{Name: "batch", CPUs: "6-7", Partition: "/virtualmachines.partition/batch.partition"},
		},
	}
}

func (suite *CPUPartitionSuite) TestNoSpecPublishesNothing() {
	suite.start()

	ctest.AssertNoResource[*runtime.CPUPartitionStatus](suite, runtime.CPUPartitionStatusID)
	suite.Assert().Zero(suite.fs.callCount())
}

func (suite *CPUPartitionSuite) TestNoPolicyIsReadyWithoutCgroupAccess() {
	suite.start()
	suite.capture(runtime.CPUPartitionSpecSpec{})

	suite.assertStatus(runtime.CPUPartitionStatusSpec{Ready: true})
	suite.Assert().Zero(suite.fs.callCount())
}

func (suite *CPUPartitionSuite) TestSetupOnce() {
	suite.start()
	suite.capture(ordinaryPolicy())

	suite.assertStatus(ordinaryPoolsStatus())
	suite.assertMasks(ordinaryMasks)

	writes := suite.fs.recordedWrites()
	suite.Assert().Less(slices.Index(writes, "virtualmachines.partition=4-7"), slices.Index(writes, "virtualmachines.partition/batch.partition=6-7"),
		"the virtual machine root is bounded before its slices")

	suite.assertNoFurtherWrites()
	suite.assertMasks(map[string]string{"kubepods": "0-7"})
}

func (suite *CPUPartitionSuite) TestKubernetesOnlyPolicy() {
	suite.start()
	suite.capture(runtime.CPUPartitionSpecSpec{Roots: map[string]string{"system": "0-1", "kubepods": "2-7"}})

	suite.assertStatus(runtime.CPUPartitionStatusSpec{Ready: true, KubeletReservedCPUs: "0-1"})
	suite.assertMasks(map[string]string{"system": "0-1", "kubepods": "2-7", "virtualmachines.partition": ""})
}

func (suite *CPUPartitionSuite) TestInvalidPolicyFailsClosed() {
	suite.start()
	suite.capture(runtime.CPUPartitionSpecSpec{Roots: map[string]string{"kubepods": "0-7"}})

	suite.assertNotReady("leave no online CPU outside kubepods")
	suite.assertStatus(runtime.CPUPartitionStatusSpec{
		Error: `root kubepods: CPUs "0-7" leave no online CPU outside kubepods`,
	})
	suite.Assert().Empty(suite.fs.recordedWrites())
}

func (suite *CPUPartitionSuite) TestExclusiveSlice() {
	suite.start()

	spec := ordinaryPolicy()
	spec.Slices = []runtime.CPUPartitionSliceSpec{{Name: "database", CPUs: "6-7", Exclusive: true}, {Name: "batch", CPUs: "5"}}
	suite.capture(spec)

	ctest.AssertResource(suite, runtime.CPUPartitionStatusID, func(res *runtime.CPUPartitionStatus, asrt *assert.Assertions) {
		asrt.True(res.TypedSpec().Ready)
	})

	suite.fs.do(func(fs *fakeCgroupFS) {
		suite.Assert().Equal(map[string]string{vmRootPath: "6-7", databasePath: "6-7"}, fs.exclusive)
		suite.Assert().Equal(map[string]string{databasePath: "root"}, fs.partition)
	})

	writes := suite.fs.recordedWrites()
	order := []string{vmRootPath + "=4-7", vmRootPath + " exclusive=6-7", databasePath + "=6-7", databasePath + " exclusive=6-7", databasePath + " partition=root"}

	for i := 1; i < len(order); i++ {
		suite.Assert().Less(slices.Index(writes, order[i-1]), slices.Index(writes, order[i]), "%s before %s", order[i-1], order[i])
	}
}

// A failed setup is reported and retried; once it succeeds the controller never writes again.
func (suite *CPUPartitionSuite) TestFailedSetupRetriesUntilReady() {
	suite.fs.do(func(fs *fakeCgroupFS) { fs.writeErr["kubepods"] = errors.New("device or resource busy") })

	suite.start()
	suite.capture(ordinaryPolicy())

	suite.assertNotReady("device or resource busy")

	suite.fs.do(func(fs *fakeCgroupFS) { delete(fs.writeErr, "kubepods") })

	suite.assertStatus(ordinaryPoolsStatus())
	suite.assertMasks(ordinaryMasks)
	suite.assertNoFurtherWrites()
}

func (suite *CPUPartitionSuite) TestVerificationFailureIsNotReady() {
	suite.fs.do(func(fs *fakeCgroupFS) { fs.effective["system"] = "0-7" })

	suite.start()
	suite.capture(ordinaryPolicy())

	suite.assertNotReady(`root system: effective CPUs "0-7" differ from "0-1"`)
}

func (suite *CPUPartitionSuite) TestPopulatedVirtualMachineRootIsNotTakenOver() {
	suite.fs.do(func(fs *fakeCgroupFS) {
		fs.cpus[vmRootPath] = "0-7"
		fs.cpus[vmRootPath+"/machine-qemu-legacy.scope"] = ""
		fs.populated[vmRootPath+"/machine-qemu-legacy.scope"] = true
	})

	suite.start()
	suite.capture(ordinaryPolicy())

	suite.assertNotReady("already runs tasks")
	suite.Assert().Empty(suite.fs.recordedWrites())
}

func (suite *CPUPartitionSuite) TestRejectedPartitionRootIsNotReady() {
	suite.fs.do(func(fs *fakeCgroupFS) { fs.rejected[databasePath] = "root invalid (Parent is not a partition root)" })

	suite.start()

	spec := ordinaryPolicy()
	spec.Slices = []runtime.CPUPartitionSliceSpec{{Name: "database", CPUs: "6-7", Exclusive: true}}
	suite.capture(spec)

	suite.assertNotReady(`slice database: partition "root invalid (Parent is not a partition root)", expected "root"`)
}

func (suite *CPUPartitionSuite) TestOnlineCPUsUnreadableIsNotReady() {
	suite.fs.do(func(fs *fakeCgroupFS) { fs.onlineErr = errors.New("permission denied") })

	suite.start()
	suite.capture(ordinaryPolicy())

	suite.assertNotReady("error reading online CPUs: permission denied")
	suite.Assert().Empty(suite.fs.recordedWrites())
}

const (
	vmRootPath   = "virtualmachines.partition"
	databasePath = "virtualmachines.partition/database.partition"
)

func TestCPUPartitionConfigCapturesOnce(t *testing.T) {
	t.Parallel()

	s := &CPUPartitionSuite{Timeout: 10 * time.Second}
	s.SetT(t)
	s.SetupTest()

	defer s.TearDownTest()

	s.Require().NoError(s.Runtime().RegisterController(&runtimectrls.CPUPartitionConfigController{V1Alpha1Mode: machineruntime.ModeMetal}))
	s.start()

	doc := runtimecfg.NewCPUPartitionConfigV1Alpha1()
	doc.SystemConfig = &runtimecfg.CPUPartitionRoot{RootCPUs: "0-1"}
	doc.KubepodsConfig = &runtimecfg.CPUPartitionRoot{RootCPUs: "2-7"}

	cfg, err := container.New(&v1alpha1.Config{}, doc)
	s.Require().NoError(err)

	active := config.NewMachineConfig(cfg)
	s.Create(active)

	s.assertStatus(runtime.CPUPartitionStatusSpec{Ready: true, KubeletReservedCPUs: "0-1"})

	doc.KubepodsConfig.RootCPUs = "3-7"
	cfg, err = container.New(&v1alpha1.Config{}, doc)
	s.Require().NoError(err)

	edited := config.NewMachineConfig(cfg)
	edited.Metadata().SetVersion(active.Metadata().Version())
	s.Update(edited)

	ctx, st := s.Ctx(), s.State()

	s.Require().Never(func() bool {
		spec, getErr := safe.StateGetByID[*runtime.CPUPartitionSpec](ctx, st, runtime.CPUPartitionSpecID)

		return getErr != nil || spec.TypedSpec().Roots["kubepods"] != "2-7"
	}, 300*time.Millisecond, 10*time.Millisecond)
}
