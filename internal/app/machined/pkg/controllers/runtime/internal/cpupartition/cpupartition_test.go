// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cpupartition_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/cpuset"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime/internal/cpupartition"
	"github.com/siderolabs/talos/pkg/machinery/config/config"
)

func cpus(list string) cpuset.CPUSet {
	set, err := cpuset.Parse(list)
	if err != nil {
		panic(err)
	}

	return set
}

func cpusPtr(list string) *cpuset.CPUSet {
	set := cpus(list)

	return &set
}

var (
	initRoot = cpupartition.Root(config.CPUPartitionRootInit)
	kubepods = cpupartition.Root(config.CPUPartitionRootKubepods)
	vmRoot   = cpupartition.Root(config.CPUPartitionRootVirtualMachines)
	database = cpupartition.Slice("database")
	cache    = cpupartition.Slice("cache")
	shared   = cpupartition.Shared
)

// accepted is the accepted policy applied to an 8-CPU host: database exclusive on 4-5, shared 6-7.
func accepted() cpupartition.Policy {
	return cpupartition.Policy{
		Sets: map[cpupartition.Target]cpuset.CPUSet{
			initRoot: cpus("0-1"),
			cpupartition.Root(config.CPUPartitionRootSystem):     cpus("0-1"),
			cpupartition.Root(config.CPUPartitionRootPodRuntime): cpus("0-1"),
			kubepods: cpus("2-3"),
			cpupartition.Root(config.CPUPartitionRootTalosContainers): cpus("0-1"),
			vmRoot:   cpus("4-7"),
			database: cpus("4-5"),
			shared:   cpus("6-7"),
		},
		Exclusive:          map[string]bool{"database": true},
		KubeletReservation: cpusPtr("0-1,4-7"),
	}
}

func running(name string, target cpupartition.Target) cpupartition.Consumer {
	return cpupartition.Consumer{Name: name, Applied: target, Desired: target, State: cpupartition.ConsumerRunning}
}

func steps(result cpupartition.Result) []string {
	out := make([]string, 0, len(result.Steps))

	for _, step := range result.Steps {
		out = append(out, step.String())
	}

	return out
}

func blocks(result cpupartition.Result) []string {
	out := make([]string, 0, len(result.Blocked))

	for _, block := range result.Blocked {
		out = append(out, block.String())
	}

	return out
}

func TestPlanNoOp(t *testing.T) {
	t.Parallel()

	result := cpupartition.Plan(cpupartition.Input{
		Applied:   accepted(),
		Desired:   accepted(),
		Consumers: []cpupartition.Consumer{running("db", database), running("web", shared)},
		Online:    cpus("0-7"),
	})

	assert.False(t, result.IsBlocked())
	assert.Empty(t, result.Steps)
	assert.Empty(t, result.EnforcementLoss)
}

func TestPlanLiveMaskChanges(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name      string
		desired   func(p *cpupartition.Policy)
		consumers []cpupartition.Consumer

		expectedSteps []string
	}{
		{
			name: "non-empty shrink of an occupied exclusive slice",
			desired: func(p *cpupartition.Policy) {
				p.Sets[database] = cpus("4")
			},
			consumers:     []cpupartition.Consumer{running("db", database)},
			expectedSteps: []string{`set slice database to "4"`},
		},
		{
			name: "expansion into CPUs the shared slice gives up: shared shrinks first",
			desired: func(p *cpupartition.Policy) {
				p.Sets[database] = cpus("4-6")
				p.Sets[shared] = cpus("7")
			},
			consumers: []cpupartition.Consumer{running("db", database), running("web", shared)},
			expectedSteps: []string{
				`set shared slice to "7"`,
				`set slice database to "4-6"`,
			},
		},
		{
			name: "virtual machine root grows before its children and shrinks after them",
			desired: func(p *cpupartition.Policy) {
				p.Sets[kubepods] = cpus("2")
				p.Sets[vmRoot] = cpus("3-7")
				p.Sets[shared] = cpus("3,6-7")
				p.KubeletReservation = cpusPtr("0-1,3-7")
			},
			consumers: []cpupartition.Consumer{running("db", database), running("web", shared)},
			expectedSteps: []string{
				`publish kubelet reservation "0-1,3-7"`,
				`await kubepods release of "3"`,
				`set root kubepods to "2"`,
				`set root virtualMachines to "3-7"`,
				`set shared slice to "3,6-7"`,
			},
		},
		{
			name: "released slice moves to disjoint CPUs the occupied slice gives up through a non-empty intermediate",
			desired: func(p *cpupartition.Policy) {
				p.Sets[database] = cpus("6")
				p.Sets[shared] = cpus("4-5,7")
			},
			consumers: []cpupartition.Consumer{
				{Name: "db", Desired: database, State: cpupartition.ConsumerReleased},
				running("web", shared),
			},
			expectedSteps: []string{
				`set shared slice to "7"`,
				`set shared slice to "4-5,7"`,
				`set slice database to "6"`,
			},
		},
		{
			name: "occupied swap staged through non-empty intermediates",
			desired: func(p *cpupartition.Policy) {
				p.Sets[database] = cpus("5-6")
				p.Sets[shared] = cpus("4,7")
			},
			consumers: []cpupartition.Consumer{running("db", database), running("web", shared)},
			expectedSteps: []string{
				`set shared slice to "7"`,
				`set slice database to "5"`,
				`set shared slice to "4,7"`,
				`set slice database to "5-6"`,
			},
		},
		{
			name: "non-exclusive slices may overlap live",
			desired: func(p *cpupartition.Policy) {
				p.Exclusive = nil
				p.Sets[cache] = cpus("5-6")
			},
			consumers: []cpupartition.Consumer{running("db", database), running("web", shared)},
			expectedSteps: []string{
				`set slice cache to "5-6"`,
			},
		},
		{
			name: "mixed kubepods shrink and expansion widens, drains, then caps",
			desired: func(p *cpupartition.Policy) {
				p.Sets[initRoot] = cpus("0")
				p.Sets[kubepods] = cpus("1,3")
				p.KubeletReservation = cpusPtr("0,2,4-7")
			},
			consumers: []cpupartition.Consumer{running("db", database)},
			expectedSteps: []string{
				`set root init to "0"`,
				`set root kubepods to "1-3"`,
				`publish kubelet reservation "0,2,4-7"`,
				`await kubepods release of "2"`,
				`set root kubepods to "1,3"`,
			},
		},
		{
			name: "disjoint move of an unoccupied slice while the shared slice keeps a CPU",
			desired: func(p *cpupartition.Policy) {
				p.Sets[database] = cpus("6")
				p.Sets[shared] = cpus("4-5,7")
			},
			consumers: []cpupartition.Consumer{running("web", shared)},
			expectedSteps: []string{
				`set shared slice to "7"`,
				`set shared slice to "4-5,7"`,
				`set slice database to "6"`,
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			desired := accepted()
			test.desired(&desired)

			result := cpupartition.Plan(cpupartition.Input{
				Applied:   accepted(),
				Desired:   desired,
				Consumers: test.consumers,
				Online:    cpus("0-7"),
			})

			require.Empty(t, blocks(result))
			assert.Equal(t, test.expectedSteps, steps(result))
		})
	}
}

func TestPlanBlocked(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name      string
		applied   func(p *cpupartition.Policy)
		desired   func(p *cpupartition.Policy)
		consumers []cpupartition.Consumer
		online    string

		expectedBlocks []string
	}{
		{
			name: "fully occupied swap",
			desired: func(p *cpupartition.Policy) {
				p.Sets[database] = cpus("6-7")
				p.Sets[shared] = cpus("4-5")
			},
			consumers: []cpupartition.Consumer{running("db", database), running("web", shared)},
			expectedBlocks: []string{
				`shared slice: moving an occupied cgroup to disjoint CPUs is not supported live (live probe gate) (virtual machines: web) (cpus: 4-5)`,
				`slice database: moving an occupied cgroup to disjoint CPUs is not supported live (live probe gate) (virtual machines: db) (cpus: 6-7)`,
			},
		},
		{
			// Shared keeps 5 while database still needs it, so the exclusive owner would share a CPU
			// with a competitor that has tasks during the shrink phase.
			name: "occupied exclusive slice cannot yield a CPU the shared slice takes before the root shrinks",
			desired: func(p *cpupartition.Policy) {
				p.Sets[database] = cpus("4-5")
				p.Sets[shared] = cpus("5-7")
				p.Exclusive = map[string]bool{"database": true}
			},
			consumers: []cpupartition.Consumer{running("db", database), running("web", shared)},
			expectedBlocks: []string{
				`after "set shared slice to \"5-7\"", exclusive slice database would share CPUs with shared slice; no live ordering keeps them apart (virtual machines: db, web) (cpus: 5)`,
			},
		},
		{
			name: "desired stopped is not released",
			desired: func(p *cpupartition.Policy) {
				p.Sets[database] = cpus("6-7")
				p.Sets[shared] = cpus("4-5")
			},
			consumers: []cpupartition.Consumer{
				running("db", database),
				{Name: "web", Applied: shared, Desired: shared, State: cpupartition.ConsumerUnreleased},
			},
			expectedBlocks: []string{
				`shared slice: moving an occupied cgroup to disjoint CPUs is not supported live (live probe gate) (virtual machines: web) (cpus: 4-5)`,
				`slice database: moving an occupied cgroup to disjoint CPUs is not supported live (live probe gate) (virtual machines: db) (cpus: 6-7)`,
			},
		},
		{
			name: "expansion into an occupied shared slice which does not give the CPUs up",
			desired: func(p *cpupartition.Policy) {
				p.Sets[database] = cpus("4-6")
			},
			consumers: []cpupartition.Consumer{running("db", database), running("web", shared)},
			expectedBlocks: []string{
				`after "set slice database to \"4-6\"", exclusive slice database would share CPUs with shared slice; no live ordering keeps them apart (virtual machines: db, web) (cpus: 6)`,
			},
		},
		{
			name: "placement change: slice retarget",
			desired: func(p *cpupartition.Policy) {
				p.Sets[cache] = cpus("6")
				p.Sets[shared] = cpus("7")
			},
			consumers: []cpupartition.Consumer{
				{Name: "web", Applied: shared, Desired: cache, State: cpupartition.ConsumerRunning},
			},
			expectedBlocks: []string{
				"placement change requires the virtual machine to be stopped and released first (virtual machines: web)",
			},
		},
		{
			name: "placement change: policy introduction moves root machines into the shared slice",
			applied: func(p *cpupartition.Policy) {
				p.Sets = nil
				p.Exclusive = nil
				p.KubeletReservation = nil
			},
			consumers: []cpupartition.Consumer{
				{Name: "web", Applied: vmRoot, Desired: shared, State: cpupartition.ConsumerRunning},
				{Name: "db", Applied: vmRoot, Desired: database, State: cpupartition.ConsumerUnreleased},
			},
			expectedBlocks: []string{
				"placement change requires the virtual machine to be stopped and released first (virtual machines: db, web)",
			},
		},
		{
			name: "placement change: policy removal",
			desired: func(p *cpupartition.Policy) {
				p.Sets = nil
				p.Exclusive = nil
				p.KubeletReservation = nil
			},
			consumers: []cpupartition.Consumer{
				{Name: "db", Applied: database, Desired: vmRoot, State: cpupartition.ConsumerRunning},
			},
			expectedBlocks: []string{
				"placement change requires the virtual machine to be stopped and released first (virtual machines: db)",
			},
		},
		{
			name: "pins of the running domain outside the shrunk slice",
			desired: func(p *cpupartition.Policy) {
				p.Sets[database] = cpus("4")
			},
			consumers: []cpupartition.Consumer{
				{Name: "db", Applied: database, Desired: database, Pins: cpus("5"), State: cpupartition.ConsumerRunning},
			},
			expectedBlocks: []string{
				"host CPU pins of the running domain fall outside the new allocation; stop and release the virtual machine first (virtual machines: db)",
			},
		},
		{
			name: "offline CPUs in the desired policy",
			desired: func(p *cpupartition.Policy) {
				p.Sets[shared] = cpus("6-8")
			},
			online: "0-7",
			expectedBlocks: []string{
				"shared slice: CPUs are offline (cpus: 8)",
			},
		},
		{
			name: "empty set is never written",
			desired: func(p *cpupartition.Policy) {
				p.Sets[shared] = cpuset.New()
			},
			expectedBlocks: []string{
				"shared slice: an empty CPU set is never written",
			},
		},
		{
			name: "fixed root moved to disjoint CPUs",
			desired: func(p *cpupartition.Policy) {
				p.Sets[initRoot] = cpus("2")
				p.Sets[kubepods] = cpus("0-1,3")
				p.KubeletReservation = cpusPtr("2,4-7")
			},
			expectedBlocks: []string{
				`root init: moving an occupied cgroup to disjoint CPUs is not supported live (live probe gate) (cpus: 2)`,
			},
		},
		{
			name: "virtual machine running directly in the root competes with an exclusive slice",
			desired: func(p *cpupartition.Policy) {
				p.Sets[database] = cpus("4-6")
				p.Sets[shared] = cpus("7")
			},
			consumers: []cpupartition.Consumer{running("db", database), running("legacy", vmRoot)},
			expectedBlocks: []string{
				`after "set shared slice to \"7\"", exclusive slice database would share CPUs with root virtualMachines; no live ordering keeps them apart (virtual machines: db, legacy) (cpus: 4-5)`,
			},
		},
		{
			// The desired placement of the root machine is the root itself (no slice, no VM root
			// change), so only the carving of the exclusive slice under it can block this.
			name: "exclusive slice carved under a machine running in the root",
			applied: func(p *cpupartition.Policy) {
				delete(p.Sets, database)
				delete(p.Sets, shared)
				p.Exclusive = nil
			},
			consumers: []cpupartition.Consumer{running("legacy", vmRoot)},
			expectedBlocks: []string{
				`after "set slice database to \"4-5\"", exclusive slice database would share CPUs with root virtualMachines; no live ordering keeps them apart (virtual machines: legacy) (cpus: 4-5)`,
			},
		},
		{
			name: "child leaving the virtual machine root",
			desired: func(p *cpupartition.Policy) {
				p.Sets[shared] = cpus("3,6-7")
			},
			expectedBlocks: []string{
				`shared slice would leave the virtual machine root "4-7" (cpus: 3)`,
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			applied, desired := accepted(), accepted()

			if test.applied != nil {
				test.applied(&applied)
			}

			if test.desired != nil {
				test.desired(&desired)
			}

			online := "0-7"
			if test.online != "" {
				online = test.online
			}

			result := cpupartition.Plan(cpupartition.Input{
				Applied:   applied,
				Desired:   desired,
				Consumers: test.consumers,
				Online:    cpus(online),
			})

			assert.Equal(t, test.expectedBlocks, blocks(result))
			assert.True(t, result.IsBlocked())
			assert.Empty(t, result.Steps, "a blocked transition must carry no steps")
		})
	}
}

// Enforcement loss is a safety condition of the applied policy, reported whatever the transition.
func TestPlanEnforcementLoss(t *testing.T) {
	t.Parallel()

	result := cpupartition.Plan(cpupartition.Input{
		Applied:   accepted(),
		Desired:   accepted(),
		Consumers: []cpupartition.Consumer{running("db", database)},
		Online:    cpus("0-4,6-7"),
	})

	assert.Equal(t, []cpupartition.Target{vmRoot, database}, result.EnforcementLoss)
	assert.True(t, result.IsBlocked(), "the desired policy names the offline CPU too")
}

func TestPlanAbsentPolicy(t *testing.T) {
	t.Parallel()

	// Nothing applied, nothing desired: the only command is an unmanaged kubelet reservation
	// when one was previously staged.
	result := cpupartition.Plan(cpupartition.Input{
		Applied: cpupartition.Policy{KubeletReservation: cpusPtr("0-1")},
		Online:  cpus("0-7"),
	})

	assert.False(t, result.IsBlocked())
	assert.Equal(t, []string{"publish kubelet reservation unmanaged"}, steps(result))

	result = cpupartition.Plan(cpupartition.Input{Online: cpus("0-7")})
	assert.False(t, result.IsBlocked())
	assert.Empty(t, result.Steps)
}

func TestTargetKey(t *testing.T) {
	t.Parallel()

	for _, target := range []cpupartition.Target{initRoot, kubepods, vmRoot, database, shared} {
		parsed, ok := cpupartition.ParseKey(target.Key())
		require.True(t, ok, target.Key())
		assert.Equal(t, target, parsed)
	}

	_, ok := cpupartition.ParseKey("nothing")
	assert.False(t, ok)
	assert.Equal(t, "virtualMachines/database", database.Key())
	assert.Equal(t, "virtualMachines/shared", shared.Key())
}

func TestTargetPartition(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "/virtualmachines.partition/shared.partition", shared.Partition())
	assert.Equal(t, "/virtualmachines.partition/database.partition", database.Partition())
	assert.Equal(t, "/virtualmachines.partition", vmRoot.Partition())
}
