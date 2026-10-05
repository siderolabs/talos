// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cpupartition_test

import (
	"maps"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/cpuset"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime/internal/cpupartition"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

var (
	initRoot   = cpupartition.Root(constants.CgroupInit)
	systemRoot = cpupartition.Root(constants.CgroupSystem)
	podRuntime = cpupartition.Root(constants.CgroupPodRuntimeRoot)
	kubepods   = cpupartition.Root(constants.CgroupKubepods)
	containers = cpupartition.Root(constants.CgroupTalosContainersRoot)
	vmRoot     = cpupartition.Root(constants.CgroupVirtualMachinesRoot)
	database   = cpupartition.Slice("database")
	cache      = cpupartition.Slice("cache")
	shared     = cpupartition.Shared
)

func cpus(list string) cpuset.CPUSet {
	set, err := cpuset.Parse(list)
	if err != nil {
		panic(err)
	}

	return set
}

// accepted is a policy for an 8-CPU host: host roots on 0-1, kubepods on 2-3, virtual machines on
// 4-7 with the exclusive slice database on 4-5 and the shared remainder on 6-7.
func accepted() cpupartition.Policy {
	return cpupartition.Policy{
		Sets: map[cpupartition.Target]cpuset.CPUSet{
			initRoot:   cpus("0-1"),
			systemRoot: cpus("0-1"),
			podRuntime: cpus("0-1"),
			kubepods:   cpus("2-3"),
			containers: cpus("0-1"),
			vmRoot:     cpus("4-7"),
			database:   cpus("4-5"),
			shared:     cpus("6-7"),
		},
		Exclusive:          map[string]bool{"database": true},
		KubeletReservation: new(cpus("0-1,4-7")),
	}
}

func unmanaged() cpupartition.Policy {
	return cpupartition.Policy{}
}

func occupying(name string, target cpupartition.Target) cpupartition.Occupant {
	return cpupartition.Occupant{Name: name, Target: target}
}

func stepStrings(result cpupartition.Result) []string {
	if len(result.Steps) == 0 {
		return nil
	}

	out := make([]string, 0, len(result.Steps))

	for _, step := range result.Steps {
		out = append(out, step.String())
	}

	return out
}

func blockStrings(result cpupartition.Result) []string {
	if len(result.Blocked) == 0 {
		return nil
	}

	out := make([]string, 0, len(result.Blocked))

	for _, block := range result.Blocked {
		out = append(out, block.String())
	}

	return out
}

type planCase struct {
	name      string
	applied   func(p *cpupartition.Policy)
	desired   func(p *cpupartition.Policy)
	occupants []cpupartition.Occupant
	online    string

	expectedSteps  []string
	expectedBlocks []string
}

func (c planCase) input() cpupartition.Input {
	applied, desired := accepted(), accepted()

	if c.applied != nil {
		c.applied(&applied)
	}

	if c.desired != nil {
		c.desired(&desired)
	}

	online := "0-7"
	if c.online != "" {
		online = c.online
	}

	return cpupartition.Input{Applied: applied, Desired: desired, Occupants: c.occupants, Online: cpus(online)}
}

func TestPlanMovingDomainKeepsActivePins(t *testing.T) {
	t.Parallel()

	in := planCase{
		desired: func(p *cpupartition.Policy) {
			p.Sets[cache] = cpus("6")
			p.Sets[shared] = cpus("7")
		},
		// The replacement wants cache, but the ACTIVE domain or held start still
		// occupies shared. Desired placement is deliberately absent from the API.
		occupants: []cpupartition.Occupant{{Name: "web", Target: shared, Pins: cpus("6")}},
	}.input()

	result := cpupartition.Plan(in)
	require.True(t, result.IsBlocked(), "desired placement does not prove the old domain was removed")
	assert.Empty(t, result.Steps)
	assert.Contains(t, blockStrings(result)[0], "host CPU pins")

	// Only domain removal plus claim/task release permits dropping the occupant.
	in.Occupants = nil
	result = cpupartition.Plan(in)
	require.False(t, result.IsBlocked(), blockStrings(result))
	assertSafe(t, in, result.Steps)
}

func TestPlanInheritedOccupants(t *testing.T) {
	t.Parallel()

	for _, target := range []cpupartition.Target{shared, vmRoot, initRoot, systemRoot, podRuntime, containers, kubepods} {
		t.Run(target.Key(), func(t *testing.T) {
			t.Parallel()

			in := planCase{occupants: []cpupartition.Occupant{occupying("db", database)}}.input()
			delete(in.Applied.Sets, target)
			delete(in.Desired.Sets, target)

			if target == shared || target == vmRoot {
				in.Occupants = append(in.Occupants, occupying("legacy", target))
			}

			result := cpupartition.Plan(in)
			require.True(t, result.IsBlocked(), "an absent configured mask inherits CPUs; it does not remove a competitor")
			assert.Empty(t, result.Steps)
			assert.Contains(t, blockStrings(result)[0], "would share CPUs")
		})
	}
}

func TestPlanFirstKubepodsBoundAwaitsRelease(t *testing.T) {
	t.Parallel()

	in := planCase{
		applied: func(p *cpupartition.Policy) {
			delete(p.Sets, kubepods)
			p.KubeletReservation = nil
		},
	}.input()

	result := cpupartition.Plan(in)
	require.False(t, result.IsBlocked(), blockStrings(result))
	assert.Equal(t, []string{
		`publish kubelet reservation "0-1,4-7"`,
		`await kubepods release of "0-1,4-7"`,
		`set root kubepods to "2-3"`,
	}, stepStrings(result), "first management may cap an existing inherited parent with pinned pod leaves")
	assertSafe(t, in, result.Steps)

	// A crash after publication cannot turn the next reconcile into an unguarded cap.
	in.Applied.KubeletReservation = in.Desired.KubeletReservation
	result = cpupartition.Plan(in)
	require.False(t, result.IsBlocked(), blockStrings(result))
	assert.Equal(t, []string{
		`await kubepods release of "0-1,4-7"`,
		`set root kubepods to "2-3"`,
	}, stepStrings(result))
	assertSafe(t, in, result.Steps)
}

// Both effective endpoints are isolated. At the root's intersection {1}, however,
// the omitted child's configured {0,2} falls back onto the exclusive owner's {1}.
// This state is reachable by first shrinking root {0,1,2} to {0,1} while omitting C.
func TestPlanMixedRootOmittedChildNeedsSimulation(t *testing.T) {
	t.Parallel()

	applied := cpupartition.Policy{
		Sets: map[cpupartition.Target]cpuset.CPUSet{
			initRoot: cpus("3"), systemRoot: cpus("3"), podRuntime: cpus("3"),
			containers: cpus("3"), kubepods: cpus("3"),
			vmRoot: cpus("0-1"), database: cpus("1"), cache: cpus("0,2"),
		},
		Exclusive: map[string]bool{"database": true},
	}
	desired := applied
	desired.Sets = maps.Clone(applied.Sets)
	desired.Sets[vmRoot] = cpus("1-2")
	delete(desired.Sets, cache)

	in := cpupartition.Input{
		Applied: applied, Desired: desired, Online: cpus("0-3"),
		Occupants: []cpupartition.Occupant{occupying("db", database), occupying("redis", cache)},
	}

	// Establish reachability from a policy whose configured children fit the root.
	prior := in
	prior.Applied.Sets = maps.Clone(applied.Sets)
	prior.Applied.Sets[vmRoot] = cpus("0-2")
	prior.Desired.Sets = maps.Clone(applied.Sets)
	delete(prior.Desired.Sets, cache)
	previous := cpupartition.Plan(prior)
	require.False(t, previous.IsBlocked(), blockStrings(previous))
	assert.Equal(t, []string{`set root virtualmachines to "0-1"`}, stepStrings(previous))
	assertSafe(t, prior, previous.Steps)

	// Explicit endpoint witnesses, independent of the planner's simulation.
	assert.True(t, cpus("0,2").Intersection(applied.Sets[vmRoot]).Intersection(cpus("1")).IsEmpty())
	assert.True(t, cpus("0,2").Intersection(desired.Sets[vmRoot]).Intersection(cpus("1")).IsEmpty())

	result := cpupartition.Plan(in)
	require.True(t, result.IsBlocked())
	assert.Empty(t, result.Steps)
	assert.Contains(t, blockStrings(result)[0], `after "set root virtualmachines to \"1\""`)
}

func TestPlanLive(t *testing.T) {
	t.Parallel()

	for _, test := range []planCase{
		{
			name:      "unchanged policy",
			occupants: []cpupartition.Occupant{occupying("db", database), occupying("web", shared)},
		},
		{
			name: "occupied exclusive slice shrinks in place",
			desired: func(p *cpupartition.Policy) {
				p.Sets[database] = cpus("4")
				p.Sets[shared] = cpus("5-7")
			},
			occupants: []cpupartition.Occupant{occupying("db", database), occupying("web", shared)},
			expectedSteps: []string{
				`set slice database to "4"`,
				`set shared slice to "5-7"`,
			},
		},
		{
			name: "exclusive slice grows into CPUs the shared remainder gives up first",
			desired: func(p *cpupartition.Policy) {
				p.Sets[database] = cpus("4-6")
				p.Sets[shared] = cpus("7")
			},
			occupants: []cpupartition.Occupant{occupying("db", database), occupying("web", shared)},
			expectedSteps: []string{
				`set shared slice to "7"`,
				`set slice database to "4-6"`,
			},
		},
		{
			name: "occupied swap staged through non-empty intermediates",
			desired: func(p *cpupartition.Policy) {
				p.Sets[database] = cpus("5-6")
				p.Sets[shared] = cpus("4,7")
			},
			occupants: []cpupartition.Occupant{occupying("db", database), occupying("web", shared)},
			expectedSteps: []string{
				`set shared slice to "7"`,
				`set slice database to "5"`,
				`set shared slice to "4,7"`,
				`set slice database to "5-6"`,
			},
		},
		{
			name: "released exclusive slice moves to disjoint CPUs after the shared remainder shrinks",
			desired: func(p *cpupartition.Policy) {
				p.Sets[database] = cpus("6")
				p.Sets[shared] = cpus("4-5,7")
			},
			occupants: []cpupartition.Occupant{
				occupying("web", shared),
			},
			expectedSteps: []string{
				`set shared slice to "7"`,
				`set shared slice to "4-5,7"`,
				`set slice database to "6"`,
			},
		},
		{
			name: "virtual machine root grows before its children and shrinks after them",
			desired: func(p *cpupartition.Policy) {
				p.Sets[kubepods] = cpus("2")
				p.Sets[vmRoot] = cpus("3-7")
				p.Sets[shared] = cpus("3,6-7")
				p.KubeletReservation = new(cpus("0-1,3-7"))
			},
			occupants: []cpupartition.Occupant{occupying("db", database), occupying("web", shared)},
			expectedSteps: []string{
				`publish kubelet reservation "0-1,3-7"`,
				`await kubepods release of "3"`,
				`set root kubepods to "2"`,
				`set root virtualmachines to "3-7"`,
				`set shared slice to "3,6-7"`,
			},
		},
		{
			name: "children shrink before the virtual machine root, which shrinks before roots grow",
			desired: func(p *cpupartition.Policy) {
				p.Sets[vmRoot] = cpus("5-7")
				p.Sets[database] = cpus("5")
				p.Sets[containers] = cpus("0-1,4")
			},
			occupants: []cpupartition.Occupant{occupying("db", database), occupying("web", shared)},
			expectedSteps: []string{
				`set slice database to "5"`,
				`set root virtualmachines to "5-7"`,
				`set root taloscontainers to "0-1,4"`,
			},
		},
		{
			name: "kubepods both gains and loses CPUs: widen, publish, drain, cap",
			desired: func(p *cpupartition.Policy) {
				p.Sets[initRoot] = cpus("0")
				p.Sets[kubepods] = cpus("1,3")
				p.KubeletReservation = new(cpus("0,2,4-7"))
			},
			occupants: []cpupartition.Occupant{occupying("db", database)},
			expectedSteps: []string{
				`set root init to "0"`,
				`set root kubepods to "1-3"`,
				`publish kubelet reservation "0,2,4-7"`,
				`await kubepods release of "2"`,
				`set root kubepods to "1,3"`,
			},
		},
		{
			name: "kubepods takes a CPU the virtual machine root gives up and returns one to the host roots",
			desired: func(p *cpupartition.Policy) {
				p.Sets[initRoot] = cpus("0-2")
				p.Sets[kubepods] = cpus("3-4")
				p.Sets[vmRoot] = cpus("5-7")
				p.Sets[database] = cpus("5")
				p.KubeletReservation = new(cpus("0-2,5-7"))
			},
			occupants: []cpupartition.Occupant{occupying("db", database), occupying("web", shared)},
			expectedSteps: []string{
				`set slice database to "5"`,
				`set root virtualmachines to "5-7"`,
				`set root kubepods to "2-4"`,
				`publish kubelet reservation "0-2,5-7"`,
				`await kubepods release of "2"`,
				`set root kubepods to "3-4"`,
				`set root init to "0-2"`,
			},
		},
		{
			name: "kubepods only grows: widen then publish",
			desired: func(p *cpupartition.Policy) {
				p.Sets[kubepods] = cpus("2-3,7")
				p.Sets[vmRoot] = cpus("4-6")
				p.Sets[shared] = cpus("6")
				p.KubeletReservation = new(cpus("0-1,4-6"))
			},
			occupants: []cpupartition.Occupant{occupying("db", database), occupying("web", shared)},
			expectedSteps: []string{
				`set shared slice to "6"`,
				`set root virtualmachines to "4-6"`,
				`set root kubepods to "2-3,7"`,
				`publish kubelet reservation "0-1,4-6"`,
			},
		},
		{
			name: "kubepods bounded for the first time after the reservation",
			applied: func(p *cpupartition.Policy) {
				delete(p.Sets, kubepods)
				p.KubeletReservation = nil
			},
			occupants: []cpupartition.Occupant{occupying("db", database)},
			expectedSteps: []string{
				`publish kubelet reservation "0-1,4-7"`,
				`await kubepods release of "0-1,4-7"`,
				`set root kubepods to "2-3"`,
			},
		},
		{
			name: "dropping exclusivity permits sharing under the desired policy",
			desired: func(p *cpupartition.Policy) {
				p.Exclusive = nil
				p.Sets[shared] = cpus("5-7")
			},
			occupants:     []cpupartition.Occupant{occupying("db", database), occupying("web", shared)},
			expectedSteps: []string{`set shared slice to "5-7"`},
		},
		{
			name: "non-exclusive slices may share CPUs",
			desired: func(p *cpupartition.Policy) {
				p.Exclusive = nil
				p.Sets[cache] = cpus("5-6")
			},
			occupants:     []cpupartition.Occupant{occupying("db", database), occupying("web", shared)},
			expectedSteps: []string{`set slice cache to "5-6"`},
		},
		{
			name: "unoccupied exclusive slice is carved from the shared remainder",
			desired: func(p *cpupartition.Policy) {
				p.Sets[cache] = cpus("7")
				p.Sets[shared] = cpus("6")
				p.Exclusive["cache"] = true
			},
			occupants: []cpupartition.Occupant{occupying("db", database), occupying("web", shared)},
			expectedSteps: []string{
				`set shared slice to "6"`,
				`set slice cache to "7"`,
			},
		},
		{
			// The domain moving to cache is replaced by the virtual machine runtime: the mask plan
			// carves the destination and keeps accounting for the old occupancy of shared.
			name: "placement change plans masks only",
			desired: func(p *cpupartition.Policy) {
				p.Sets[cache] = cpus("6")
				p.Sets[shared] = cpus("7")
			},
			occupants: []cpupartition.Occupant{occupying("web", shared)},
			expectedSteps: []string{
				`set shared slice to "7"`,
				`set slice cache to "6"`,
			},
		},
		{
			name: "policy introduction with only a shared remainder over machines in the root",
			applied: func(p *cpupartition.Policy) {
				*p = unmanaged()
			},
			desired: func(p *cpupartition.Policy) {
				delete(p.Sets, database)
				p.Sets[shared] = cpus("4-7")
				p.Exclusive = nil
			},
			occupants: []cpupartition.Occupant{occupying("web", vmRoot)},
			expectedSteps: []string{
				`publish kubelet reservation "0-1,4-7"`,
				`await kubepods release of "0-1,4-7"`,
				`set root kubepods to "2-3"`,
				`set root init to "0-1"`,
				`set root podruntime to "0-1"`,
				`set root system to "0-1"`,
				`set root taloscontainers to "0-1"`,
				`set root virtualmachines to "4-7"`,
				`set shared slice to "4-7"`,
			},
		},
		{
			name: "removing the policy publishes an unmanaged reservation and leaves masks for restoration",
			desired: func(p *cpupartition.Policy) {
				*p = unmanaged()
			},
			occupants:     []cpupartition.Occupant{occupying("db", database)},
			expectedSteps: []string{"publish kubelet reservation unmanaged"},
		},
		{
			name:    "nothing applied, nothing desired",
			applied: func(p *cpupartition.Policy) { *p = unmanaged() },
			desired: func(p *cpupartition.Policy) { *p = unmanaged() },
		},
		{
			name: "pins inside the new allocation",
			desired: func(p *cpupartition.Policy) {
				p.Sets[database] = cpus("4")
				p.Sets[shared] = cpus("5-7")
			},
			occupants: []cpupartition.Occupant{
				{Name: "db", Target: database, Pins: cpus("4")},
			},
			expectedSteps: []string{
				`set slice database to "4"`,
				`set shared slice to "5-7"`,
			},
		},
		{
			name:   "masks within the online CPUs while others are offline",
			online: "0-6",
			applied: func(p *cpupartition.Policy) {
				p.Sets[vmRoot] = cpus("4-6")
				p.Sets[shared] = cpus("6")
				p.KubeletReservation = new(cpus("0-1,4-6"))
			},
			desired: func(p *cpupartition.Policy) {
				p.Sets[vmRoot] = cpus("4-6")
				p.Sets[database] = cpus("4")
				p.Sets[shared] = cpus("5-6")
				p.KubeletReservation = new(cpus("0-1,4-6"))
			},
			occupants: []cpupartition.Occupant{occupying("db", database), occupying("web", shared)},
			expectedSteps: []string{
				`set slice database to "4"`,
				`set shared slice to "5-6"`,
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			in := test.input()
			result := cpupartition.Plan(in)

			require.Empty(t, blockStrings(result))
			assert.Equal(t, test.expectedSteps, stepStrings(result))
			assertSafe(t, in, result.Steps)
		})
	}
}

func TestPlanBlocked(t *testing.T) {
	t.Parallel()

	for _, test := range []planCase{
		{
			name: "fully occupied disjoint swap",
			desired: func(p *cpupartition.Policy) {
				p.Sets[database] = cpus("6-7")
				p.Sets[shared] = cpus("4-5")
			},
			occupants: []cpupartition.Occupant{occupying("db", database), occupying("web", shared)},
			expectedBlocks: []string{
				`shared slice: a cgroup with tasks cannot move to disjoint CPUs; keep a CPU in common with "6-7" (virtual machines: web) (cpus: 4-5)`,
				`slice database: a cgroup with tasks cannot move to disjoint CPUs; keep a CPU in common with "4-5" (virtual machines: db) (cpus: 6-7)`,
			},
		},
		{
			name: "in-flight start occupies like a running domain",
			desired: func(p *cpupartition.Policy) {
				p.Sets[database] = cpus("6")
				p.Sets[shared] = cpus("4-5,7")
			},
			occupants: []cpupartition.Occupant{
				occupying("db", database),
				occupying("web", shared),
			},
			expectedBlocks: []string{
				`slice database: a cgroup with tasks cannot move to disjoint CPUs; keep a CPU in common with "4-5" (virtual machines: db) (cpus: 6)`,
			},
		},
		{
			name: "unreleased consumer occupies like a running domain",
			desired: func(p *cpupartition.Policy) {
				p.Sets[database] = cpus("6")
				p.Sets[shared] = cpus("4-5,7")
			},
			occupants: []cpupartition.Occupant{
				occupying("db", database),
				occupying("web", shared),
			},
			expectedBlocks: []string{
				`slice database: a cgroup with tasks cannot move to disjoint CPUs; keep a CPU in common with "4-5" (virtual machines: db) (cpus: 6)`,
			},
		},
		{
			name: "fixed root moved to disjoint CPUs",
			desired: func(p *cpupartition.Policy) {
				p.Sets[initRoot] = cpus("2")
				p.Sets[kubepods] = cpus("0-1,3")
				p.KubeletReservation = new(cpus("2,4-7"))
			},
			expectedBlocks: []string{
				`root init: a cgroup with tasks cannot move to disjoint CPUs; keep a CPU in common with "0-1" (cpus: 2)`,
			},
		},
		{
			name: "exclusive slice cannot grow into CPUs an occupied competitor keeps",
			desired: func(p *cpupartition.Policy) {
				p.Sets[database] = cpus("4-6")
			},
			occupants: []cpupartition.Occupant{occupying("db", database), occupying("web", shared)},
			expectedBlocks: []string{
				`after "set slice database to \"4-6\"", exclusive slice database would share CPUs with shared slice; the staged ordering does not keep them apart (virtual machines: db, web) (cpus: 6)`,
			},
		},
		{
			name: "competitor taking a CPU the occupied exclusive slice keeps",
			desired: func(p *cpupartition.Policy) {
				p.Sets[shared] = cpus("5-7")
			},
			occupants: []cpupartition.Occupant{occupying("db", database), occupying("web", shared)},
			expectedBlocks: []string{
				`after "set shared slice to \"5-7\"", exclusive slice database would share CPUs with shared slice; the staged ordering does not keep them apart (virtual machines: db, web) (cpus: 5)`,
			},
		},
		{
			name: "competing fixed root growing into an occupied exclusive slice",
			desired: func(p *cpupartition.Policy) {
				p.Sets[containers] = cpus("0-1,5")
			},
			occupants: []cpupartition.Occupant{occupying("db", database)},
			expectedBlocks: []string{
				`after "set root taloscontainers to \"0-1,5\"", exclusive slice database would share CPUs with root taloscontainers; the staged ordering does not keep them apart (virtual machines: db) (cpus: 5)`,
			},
		},
		{
			name: "virtual machine running directly in the root competes with an exclusive slice",
			desired: func(p *cpupartition.Policy) {
				p.Sets[database] = cpus("4-6")
				p.Sets[shared] = cpus("7")
			},
			occupants: []cpupartition.Occupant{occupying("db", database), occupying("legacy", vmRoot)},
			expectedBlocks: []string{
				`after "set shared slice to \"7\"", exclusive slice database would share CPUs with root virtualmachines; the staged ordering does not keep them apart (virtual machines: db, legacy) (cpus: 4-5)`,
			},
		},
		{
			name: "exclusive slice carved under machines still occupying the root",
			applied: func(p *cpupartition.Policy) {
				*p = unmanaged()
			},
			occupants: []cpupartition.Occupant{
				occupying("web", vmRoot),
				occupying("db", vmRoot),
			},
			expectedBlocks: []string{
				`after "set slice database to \"4-5\"", exclusive slice database would share CPUs with root virtualmachines; the staged ordering does not keep them apart (virtual machines: db, web) (cpus: 4-5)`,
			},
		},
		{
			name: "placement change keeps occupying the old partition",
			desired: func(p *cpupartition.Policy) {
				p.Sets[database] = cpus("6")
				p.Sets[shared] = cpus("4-5,7")
			},
			occupants: []cpupartition.Occupant{occupying("db", database)},
			expectedBlocks: []string{
				`slice database: a cgroup with tasks cannot move to disjoint CPUs; keep a CPU in common with "4-5" (virtual machines: db) (cpus: 6)`,
			},
		},
		{
			// cache is not in the desired policy and keeps its mask; once the root no longer covers
			// it, its effective set falls back to the whole root, including the exclusive slice.
			name: "occupied slice left outside the shrunk root falls back to the whole root",
			applied: func(p *cpupartition.Policy) {
				p.Sets[cache] = cpus("7")
				p.Sets[shared] = cpus("6")
			},
			desired: func(p *cpupartition.Policy) {
				p.Sets[vmRoot] = cpus("4-6")
				p.Sets[shared] = cpus("6")
				p.Sets[containers] = cpus("0-1,7")
			},
			occupants: []cpupartition.Occupant{occupying("db", database), occupying("redis", cache)},
			expectedBlocks: []string{
				`after "set root virtualmachines to \"4-6\"", exclusive slice database would share CPUs with slice cache; the staged ordering does not keep them apart (virtual machines: db, redis) (cpus: 4-5)`,
			},
		},
		{
			name: "marking an occupied slice exclusive while it shares CPUs",
			applied: func(p *cpupartition.Policy) {
				p.Sets[cache] = cpus("6")
			},
			desired: func(p *cpupartition.Policy) {
				p.Sets[cache] = cpus("6")
				p.Exclusive["cache"] = true
			},
			occupants: []cpupartition.Occupant{occupying("redis", cache), occupying("web", shared)},
			expectedBlocks: []string{
				`with the current masks, exclusive slice cache would share CPUs with shared slice; the staged ordering does not keep them apart (virtual machines: redis, web) (cpus: 6)`,
			},
		},
		{
			name: "pins outside the shrunk slice",
			desired: func(p *cpupartition.Policy) {
				p.Sets[database] = cpus("4")
				p.Sets[shared] = cpus("5-7")
			},
			occupants: []cpupartition.Occupant{
				{Name: "db", Target: database, Pins: cpus("4-5")},
			},
			expectedBlocks: []string{
				"host CPU pins of the domain fall outside the new allocation of slice database (virtual machines: db) (cpus: 5)",
			},
		},
		{
			name: "pins dropped through the virtual machine root",
			desired: func(p *cpupartition.Policy) {
				p.Sets[vmRoot] = cpus("4-6")
				delete(p.Sets, shared)
			},
			occupants: []cpupartition.Occupant{
				{Name: "web", Target: shared, Pins: cpus("7")},
			},
			expectedBlocks: []string{
				"host CPU pins of the domain fall outside the new allocation of shared slice (virtual machines: web) (cpus: 7)",
			},
		},
		{
			name: "pins of a machine in the unbounded root outside the new root",
			applied: func(p *cpupartition.Policy) {
				*p = unmanaged()
			},
			desired: func(p *cpupartition.Policy) {
				*p = cpupartition.Policy{Sets: map[cpupartition.Target]cpuset.CPUSet{vmRoot: cpus("4-7")}}
			},
			occupants: []cpupartition.Occupant{
				{Name: "legacy", Target: vmRoot, Pins: cpus("2-3")},
			},
			expectedBlocks: []string{
				"host CPU pins of the domain fall outside the new allocation of root virtualmachines (virtual machines: legacy) (cpus: 2-3)",
			},
		},
		{
			name:   "desired CPUs offline",
			online: "0-6",
			applied: func(p *cpupartition.Policy) {
				p.Sets[vmRoot] = cpus("4-6")
				p.Sets[shared] = cpus("6")
			},
			desired: func(p *cpupartition.Policy) {
				p.Sets[shared] = cpus("6-7")
			},
			expectedBlocks: []string{
				"root virtualmachines: CPUs are offline (cpus: 7)",
				"shared slice: CPUs are offline (cpus: 7)",
			},
		},
		{
			name: "empty set is never written",
			desired: func(p *cpupartition.Policy) {
				p.Sets[shared] = cpuset.New()
			},
			expectedBlocks: []string{"shared slice: an empty CPU set is never written"},
		},
		{
			name: "child leaving the virtual machine root",
			desired: func(p *cpupartition.Policy) {
				p.Sets[shared] = cpus("3,6-7")
			},
			expectedBlocks: []string{`shared slice would leave the virtual machine root "4-7" (cpus: 3)`},
		},
		{
			name: "target without a cgroup",
			desired: func(p *cpupartition.Policy) {
				p.Sets[cpupartition.Slice("../kubepods")] = cpus("6")
			},
			expectedBlocks: []string{"slice ../kubepods: not a CPU partition target"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			result := cpupartition.Plan(test.input())

			assert.True(t, result.IsBlocked())
			assert.Equal(t, test.expectedBlocks, blockStrings(result))
			assert.Empty(t, result.Steps, "a rejected transition carries no steps")
		})
	}
}

// TestPlanAssumesSuppliedOccupancy shows a plan is only valid for the occupants it was computed
// with: the same transition is rejected once a start into the moved slice is in flight.
func TestPlanAssumesSuppliedOccupancy(t *testing.T) {
	t.Parallel()

	transition := planCase{
		desired: func(p *cpupartition.Policy) {
			p.Sets[database] = cpus("6")
			p.Sets[shared] = cpus("4-5,7")
		},
	}

	released := transition
	released.occupants = []cpupartition.Occupant{occupying("web", shared)}

	in := released.input()
	result := cpupartition.Plan(in)
	require.False(t, result.IsBlocked(), blockStrings(result))
	require.NotEmpty(t, result.Steps)
	assertSafe(t, in, result.Steps)

	starting := transition
	starting.occupants = []cpupartition.Occupant{occupying("db", database), occupying("web", shared)}

	result = cpupartition.Plan(starting.input())
	assert.True(t, result.IsBlocked())
	assert.Empty(t, result.Steps)
}

func TestPlanDeterministic(t *testing.T) {
	t.Parallel()

	in := planCase{
		desired: func(p *cpupartition.Policy) {
			p.Sets[database] = cpus("5-6")
			p.Sets[shared] = cpus("4,7")
			p.Sets[kubepods] = cpus("1,3")
			p.Sets[initRoot] = cpus("0")
			p.KubeletReservation = new(cpus("0,2,4-7"))
		},
		occupants: []cpupartition.Occupant{occupying("db", database), occupying("web", shared), occupying("api", shared)},
	}.input()

	expected := cpupartition.Plan(in)
	require.False(t, expected.IsBlocked(), blockStrings(expected))
	assertSafe(t, in, expected.Steps)

	for range 50 {
		in.Occupants = slices.Clone(in.Occupants)
		slices.Reverse(in.Occupants)
		in.Applied.Sets, in.Desired.Sets = maps.Clone(in.Applied.Sets), maps.Clone(in.Desired.Sets)

		assert.Equal(t, stepStrings(expected), stepStrings(cpupartition.Plan(in)))
	}
}

// assertSafe replays the steps over the applied masks and checks every intermediate state, not
// only the final one, independently of the planner's own simulation.
//
//nolint:gocyclo,cyclop
func assertSafe(t *testing.T, in cpupartition.Input, steps []cpupartition.Step) {
	t.Helper()

	current := maps.Clone(in.Applied.Sets)
	if current == nil {
		current = map[cpupartition.Target]cpuset.CPUSet{}
	}

	occupied := map[cpupartition.Target][]string{}

	for _, c := range in.Occupants {
		occupied[c.Target] = append(occupied[c.Target], c.Name)
	}

	hasTasks := func(target cpupartition.Target) bool {
		if target.Kind == cpupartition.KindRoot && target != vmRoot {
			return true
		}

		for t, names := range occupied {
			if len(names) > 0 && (t == target || target == vmRoot && t.Kind != cpupartition.KindRoot) {
				return true
			}
		}

		return false
	}

	effective := func(target cpupartition.Target) cpuset.CPUSet {
		parent := in.Online
		if target.Kind != cpupartition.KindRoot {
			if root := current[vmRoot].Intersection(in.Online); !root.IsEmpty() {
				parent = root
			}
		}

		set := current[target].Intersection(parent)
		if set.IsEmpty() {
			return parent
		}

		return set
	}

	// Inventory is independent of configured masks: roots and actual occupants
	// remain competitors when their cpuset.cpus is inherited or not yet bounded.
	targets := map[cpupartition.Target]bool{}
	for _, target := range []cpupartition.Target{initRoot, systemRoot, podRuntime, containers, kubepods, vmRoot} {
		targets[target] = true
	}

	for target := range current {
		targets[target] = true
	}

	for target := range in.Desired.Sets {
		targets[target] = true
	}

	protectedPins := map[string]cpuset.CPUSet{}

	for _, occupant := range in.Occupants {
		targets[occupant.Target] = true
		protectedPins[occupant.Name] = occupant.Pins.Intersection(effective(occupant.Target))
	}

	assertIsolation := func() {
		for _, occupant := range in.Occupants {
			require.True(t, protectedPins[occupant.Name].IsSubsetOf(effective(occupant.Target)),
				"active/claimed pins of %s were displaced", occupant.Name)
		}

		for slice := range targets {
			if slice.Kind != cpupartition.KindSlice || !in.Desired.Exclusive[slice.Name] {
				continue
			}

			_, allocated := current[slice]
			if len(occupied[slice]) == 0 && (!allocated || len(occupied[vmRoot]) == 0) {
				continue
			}

			for competitor := range targets {
				if competitor == slice || competitor == vmRoot && len(occupied[vmRoot]) == 0 || competitor.Kind != cpupartition.KindRoot && len(occupied[competitor]) == 0 {
					continue
				}

				require.True(t, effective(slice).Intersection(effective(competitor)).IsEmpty(),
					"exclusive %s shares CPUs with %s", slice, competitor)
			}
		}
	}

	var (
		grown   bool
		drained = cpuset.New()
	)

	for i, step := range steps {
		if step.Kind == cpupartition.StepAwaitKubepodsRelease {
			drained = drained.Union(step.CPUs)
		}

		if step.Kind != cpupartition.StepSetCPUs {
			continue
		}

		old, existed := current[step.Target]

		require.False(t, step.CPUs.IsEmpty(), "step %d writes an empty mask: %s", i, step)
		require.True(t, step.CPUs.IsSubsetOf(in.Online), "step %d names offline CPUs: %s", i, step)

		if existed && hasTasks(step.Target) {
			require.False(t, old.Intersection(step.CPUs).IsEmpty(), "step %d moves a cgroup with tasks to disjoint CPUs: %s", i, step)
		}

		removes := existed && !old.Difference(step.CPUs).IsEmpty()
		adds := !existed || !step.CPUs.IsSubsetOf(old)

		if step.Target == kubepods {
			if !existed {
				old = in.Online
			}

			require.True(t, old.Difference(step.CPUs).IsSubsetOf(drained), "step %d caps kubepods before its pods drained: %s", i, step)
		} else {
			// A disjoint replace (of a target without tasks) is a grow-phase write.
			require.False(t, removes && !adds && grown, "step %d shrinks after a grow: %s", i, step)

			grown = grown || adds
		}

		current[step.Target] = step.CPUs

		assertIsolation()
	}

	assertIsolation()

	for target, set := range in.Desired.Sets {
		assert.True(t, set.Equals(current[target]), "final %s is %q, desired %q", target, current[target], set)
	}

	published := slices.IndexFunc(steps, func(s cpupartition.Step) bool { return s.Kind == cpupartition.StepPublishKubeletReservation })
	reservationChanged := (in.Applied.KubeletReservation == nil) != (in.Desired.KubeletReservation == nil) ||
		in.Applied.KubeletReservation != nil && !in.Applied.KubeletReservation.Equals(*in.Desired.KubeletReservation)

	assert.Equal(t, reservationChanged, published >= 0, "reservation published iff it changed")

	if published >= 0 {
		for _, step := range steps[published+1:] {
			assert.NotEqual(t, cpupartition.StepPublishKubeletReservation, step.Kind, "reservation published twice")
		}

		for _, step := range steps[:published] {
			assert.NotEqual(t, cpupartition.StepAwaitKubepodsRelease, step.Kind, "kubepods drained before the reservation moved pods")
		}
	}
}
