// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// The container list is a join across three watched resources and a polled API, so the join and
// the formatting live in free functions and are tested here without a terminal.
//
//nolint:testpackage
package dashboard

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/rivo/tview"
	"github.com/siderolabs/gen/xslices"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/resources/containers"
)

func newStatus(id string, spec containers.ContainerStatusSpec) *containers.ContainerStatus {
	status := containers.NewContainerStatus(containers.NamespaceName, id)
	*status.TypedSpec() = spec

	return status
}

func newInstanceStatus(id string, spec containers.ContainerInstanceStatusSpec) *containers.ContainerInstanceStatus {
	instance := containers.NewContainerInstanceStatus(containers.NamespaceName, id)
	*instance.TypedSpec() = spec

	return instance
}

func TestBuildContainerRows(t *testing.T) {
	statuses := map[string]*containers.ContainerStatus{
		"director": newStatus("director", containers.ContainerStatusSpec{
			State:        containers.ContainerStateRunning,
			Health:       containers.ContainerHealthHealthy,
			Image:        "ghcr.io/siderolabs/director@sha256:abcd",
			PID:          1423,
			RestartCount: 0,
		}),
		"collector": newStatus("collector", containers.ContainerStatusSpec{
			State:      containers.ContainerStatePending,
			Health:     containers.ContainerHealthPending,
			Image:      "docker.io/library/nginx:latest",
			WaitingFor: []string{"volume web-content", "network addresses"},
		}),
		"relay": newStatus("relay", containers.ContainerStatusSpec{
			State:        containers.ContainerStateRunning,
			Health:       containers.ContainerHealthHealthy,
			Image:        "ghcr.io/siderolabs/relay@sha256:ef01",
			PID:          1990,
			RestartCount: 3,
		}),
	}

	// The metrics are keyed by the containerd container ID, i.e. by the running instance rather
	// than by the container: a container on its fourth generation reports under "<name>-3".
	stats := map[string]*machine.Stat{
		"director-0": {Id: "director-0", MemoryUsage: 48 << 20},
		"relay-3":    {Id: "relay-3", MemoryUsage: 12 << 20},
	}

	// One second of CPU time over a five second interval is 20% of a core.
	cpuDiff := map[string]uint64{"director-0": uint64(time.Second), "relay-3": uint64(time.Second / 2)}

	rows := buildContainerRows(statuses, stats, cpuDiff, 5*time.Second, "", containerSortName)
	require.Len(t, rows, 3)

	assert.Equal(t, "collector", rows[0].Name)
	assert.Equal(t, "pending (volume web-content +1)", rows[0].State)
	assert.False(t, rows[0].MemoryKnown)
	assert.True(t, math.IsNaN(rows[0].CPUPercent))
	assert.Equal(t, metricUnknown, formatCPUPercent(rows[0].CPUPercent))
	assert.Equal(t, metricUnknown, formatMemory(rows[0]))
	assert.Equal(t, metricUnknown, formatPID(rows[0].PID))

	assert.Equal(t, "director", rows[1].Name)
	assert.Equal(t, "running", rows[1].State)
	assert.InDelta(t, 20.0, rows[1].CPUPercent, 0.001)
	assert.Equal(t, "20.0%", formatCPUPercent(rows[1].CPUPercent))
	assert.Equal(t, uint64(48<<20), rows[1].Memory)
	assert.Equal(t, "1423", formatPID(rows[1].PID))

	// The generation suffix is what makes the join work: a restarted container keeps its metrics.
	assert.Equal(t, "relay", rows[2].Name)
	assert.InDelta(t, 10.0, rows[2].CPUPercent, 0.001)
	assert.Equal(t, uint64(12<<20), rows[2].Memory)
}

func TestBuildContainerRowsFilter(t *testing.T) {
	statuses := map[string]*containers.ContainerStatus{
		"director": newStatus("director", containers.ContainerStatusSpec{
			State: containers.ContainerStateRunning,
			Image: "ghcr.io/siderolabs/director:v1",
		}),
		"nginx": newStatus("nginx", containers.ContainerStatusSpec{
			State: containers.ContainerStateBackoff,
			Image: "docker.io/library/nginx:latest",
		}),
	}

	for _, test := range []struct {
		name   string
		filter string
		expect []string
	}{
		{name: "by name", filter: "DIRE", expect: []string{"director"}},
		{name: "by image", filter: "docker.io", expect: []string{"nginx"}},
		{name: "by state", filter: "backoff", expect: []string{"nginx"}},
		{name: "no match", filter: "zzz", expect: nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			rows := buildContainerRows(statuses, nil, nil, time.Second, test.filter, containerSortName)

			names := make([]string, 0, len(rows))
			for _, row := range rows {
				names = append(names, row.Name)
			}

			assert.Equal(t, test.expect, nilIfEmpty(names))
		})
	}
}

func nilIfEmpty(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	return values
}

func TestSortContainerRows(t *testing.T) {
	rows := []containerRow{
		{Name: "healthy", Health: containers.ContainerHealthHealthy, Restarts: 1, CPUPercent: 1, Memory: 10, MemoryKnown: true},
		{Name: "degraded", Health: containers.ContainerHealthDegraded, Restarts: 9, CPUPercent: math.NaN()},
		{Name: "pulling", Health: containers.ContainerHealthPulling, Restarts: 3, CPUPercent: 5, Memory: 30, MemoryKnown: true},
	}

	for _, test := range []struct {
		name   string
		sortBy containerSort
		expect []string
	}{
		{name: "name", sortBy: containerSortName, expect: []string{"degraded", "healthy", "pulling"}},
		{name: "health", sortBy: containerSortHealth, expect: []string{"degraded", "pulling", "healthy"}},
		{name: "restarts", sortBy: containerSortRestarts, expect: []string{"degraded", "pulling", "healthy"}},
		// The unknown CPU sorts last rather than as zero.
		{name: "cpu", sortBy: containerSortCPU, expect: []string{"pulling", "healthy", "degraded"}},
		{name: "memory", sortBy: containerSortMemory, expect: []string{"pulling", "healthy", "degraded"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			sorted := append([]containerRow(nil), rows...)
			sortContainerRows(sorted, test.sortBy)

			names := make([]string, 0, len(sorted))
			for _, row := range sorted {
				names = append(names, row.Name)
			}

			assert.Equal(t, test.expect, names)
		})
	}
}

func TestContainerSortCycles(t *testing.T) {
	sortBy := containerSortName

	seen := make([]string, 0, 7)
	seen = append(seen, sortBy.String())

	for range 6 {
		sortBy = sortBy.next()
		seen = append(seen, sortBy.String())
	}

	assert.Equal(t, []string{"name", "health", "restarts", "cpu", "memory", "name", "health"}, seen)
}

func TestFormatContainerDetail(t *testing.T) {
	started := time.Date(2026, 8, 31, 12, 4, 11, 0, time.UTC)
	now := started.Add(3 * time.Minute)

	status := newStatus("director", containers.ContainerStatusSpec{
		State:        containers.ContainerStateRunning,
		Health:       containers.ContainerHealthHealthy,
		Image:        "ghcr.io/siderolabs/director@sha256:abcd",
		PID:          1423,
		RestartCount: 2,
	})

	spec := containers.NewContainerSpec(containers.NamespaceName, "director")
	*spec.TypedSpec() = containers.ContainerSpecSpec{
		Network:  containers.ContainerNetworkSpec{HostNetwork: true},
		Security: containers.ContainerSecuritySpec{Privileged: true},
		Resources: containers.ContainerResourcesSpec{
			CPULimit:    1500,
			MemoryLimit: 512 << 20,
		},
		Mounts: []containers.ContainerMountSpec{
			{Kind: containers.MountKindUserVolume, VolumeID: "u-web-content", Destination: "/var/www"},
			{Kind: containers.MountKindTmpfs, Size: 64 << 20, Destination: "/var/cache", Options: []string{"rw"}},
		},
		DependsOn: containers.ContainerDependsOnSpec{
			Networks:   []string{"addresses"},
			Time:       true,
			Containers: []string{"collector"},
		},
	}

	newest := newInstanceStatus("director-2", containers.ContainerInstanceStatusSpec{
		ContainerID: "director",
		Generation:  2,
		Phase:       containers.ContainerInstancePhaseRunning,
		PID:         1423,
		StartedAt:   started,
	})

	detail := formatContainerDetail(status, spec, newest, now)

	assert.Contains(t, detail, "ghcr.io/siderolabs/director@sha256:abcd")
	assert.Contains(t, detail, "pid 1423 · restarts 2")
	// The exit code of a still-running instance is not meaningful.
	assert.Contains(t, detail, "exit "+metricUnknown)
	assert.Contains(t, detail, "up 3m0s")
	assert.Contains(t, detail, "network host")
	assert.Contains(t, detail, "privileged")
	assert.Contains(t, detail, "cpu 1500m")
	assert.Contains(t, detail, "memory 512 MiB")
	assert.Contains(t, detail, "volume u-web-content to /var/www")
	assert.Contains(t, detail, "tmpfs 64 MiB to /var/cache (rw)")
	assert.Contains(t, detail, "network addresses · time · containers collector")
}

func TestFormatContainerDetailPartial(t *testing.T) {
	// The status, the spec and the instance are produced by different controllers, so any of them
	// may be missing while the rest render.
	detail := formatContainerDetail(nil, nil, nil, time.Now())

	assert.Contains(t, detail, "Started")
	assert.Contains(t, detail, metricUnknown)

	status := newStatus("broken", containers.ContainerStatusSpec{
		State:      containers.ContainerStatePending,
		Health:     containers.ContainerHealthPending,
		WaitingFor: []string{"image"},
		Error:      "pull: unauthorized",
	})

	detail = formatContainerDetail(status, nil, nil, time.Now())

	assert.Contains(t, detail, "Waiting for")
	assert.Contains(t, detail, "pull: unauthorized")
}

func TestFormatContainerDetailExitCodeAcrossRestartGap(t *testing.T) {
	// There is no instance status between generations, nor while a restart waits on a gate:
	// StatusController carries the last exit code on the status itself, and that window is exactly
	// where an operator is looking.
	status := newStatus("crasher", containers.ContainerStatusSpec{
		State:        containers.ContainerStatePending,
		Health:       containers.ContainerHealthDegraded,
		ExitCode:     137,
		RestartCount: 4,
	})

	assert.Contains(t, formatContainerDetail(status, nil, nil, time.Now()), "exit 137")

	// A container that has never run reports nothing, rather than reading back as a clean exit.
	fresh := newStatus("fresh", containers.ContainerStatusSpec{
		State:  containers.ContainerStatePending,
		Health: containers.ContainerHealthPending,
	})

	assert.Contains(t, formatContainerDetail(fresh, nil, nil, time.Now()), "exit "+metricUnknown)
}

func TestInstanceRowCells(t *testing.T) {
	started := time.Date(2026, 8, 31, 12, 3, 45, 0, time.UTC)

	cells := instanceRowCells(&containers.ContainerInstanceStatusSpec{
		Generation: 116,
		Phase:      containers.ContainerInstancePhaseFailed,
		ExitCode:   1,
		Error:      "pull: unauthorized",
		StartedAt:  started,
		FinishedAt: started.Add(time.Second),
	})

	assert.Equal(t, []string{
		"116",
		"failed",
		metricUnknown,
		"1",
		"2026-08-31 12:03:45",
		"2026-08-31 12:03:46",
		"pull: unauthorized",
	}, cells)

	// A running instance has neither an exit code nor a finish time yet.
	cells = instanceRowCells(&containers.ContainerInstanceStatusSpec{
		Generation: 117,
		Phase:      containers.ContainerInstancePhaseRunning,
		PID:        99,
		StartedAt:  started,
	})

	assert.Equal(t, []string{"117", "running", "99", metricUnknown, "2026-08-31 12:03:45", metricUnknown, metricUnknown}, cells)
}

func TestContainerHealthSummary(t *testing.T) {
	assert.Equal(t, "no containers", containerHealthSummary(nil))

	summary := containerHealthSummary([]containerRow{
		{Health: containers.ContainerHealthHealthy},
		{Health: containers.ContainerHealthHealthy},
		{Health: containers.ContainerHealthDegraded},
	})

	assert.Contains(t, summary, "3 containers")
	assert.Contains(t, summary, "2 healthy")
	assert.Contains(t, summary, "1 degraded")
}

// The dashboard draws through tcell, which never lets a terminal control character reach the
// terminal. What it does read out of the text is tview's own markup, so a container whose name,
// image or mount path carries "[red]" repaints or hides part of the operator's dashboard unless
// every node-supplied string passes through tview.Escape first.
//
// tview.Escape turns "[red]" into "[red[]", which prints as the literal text.
const (
	markupPayload = "ctr[red]INJECTED[-]name"
	markupEscaped = "ctr[red[]INJECTED[-[]name"
)

// assertInertMarkup checks that text carries the payload only in its escaped form, i.e. tview
// prints it rather than acting on it.
//
// The raw payload is what is asserted absent, rather than "[red]" outright: the panes emit color
// tags of their own, and those are ours to emit.
func assertInertMarkup(t *testing.T, text string) {
	t.Helper()

	require.Contains(t, text, "INJECTED", "the payload did not reach the text, so this test proves nothing")
	assert.Contains(t, text, markupEscaped)
	assert.NotContains(t, text, markupPayload)
}

func TestContainerPaneTitlesEscapeNodeData(t *testing.T) {
	t.Parallel()

	assertInertMarkup(t, apiDetailTitle(markupPayload))
	assertInertMarkup(t, detailTitle(markupPayload, nil))
	assertInertMarkup(t, yamlTitle(markupPayload, "ContainerStatus"))

	// The log label is the one sink that must not be escaped: HorizontalLine paints its runes onto
	// the screen one cell at a time and parses no markup, so an escape would only be visible.
	assert.Contains(t, logLabel(markupPayload, []string{"/: filter"}), markupPayload)

	assertInertMarkup(t, detailTitle(markupPayload, newStatus(markupPayload, containers.ContainerStatusSpec{
		State:  containers.ContainerStateRunning,
		Health: containers.ContainerHealthHealthy,
	})))
}

func TestFormatContainerDetailEscapesNodeData(t *testing.T) {
	t.Parallel()

	status := newStatus("director", containers.ContainerStatusSpec{
		State:      containers.ContainerStatePending,
		Health:     containers.ContainerHealthPending,
		Image:      markupPayload,
		WaitingFor: []string{markupPayload},
		Error:      markupPayload,
	})

	spec := containers.NewContainerSpec(containers.NamespaceName, "director")
	*spec.TypedSpec() = containers.ContainerSpecSpec{
		Mounts: []containers.ContainerMountSpec{
			{Kind: containers.MountKindHostPath, Source: markupPayload, Destination: markupPayload, Options: []string{markupPayload}},
			{Kind: containers.MountKindUserVolume, VolumeID: markupPayload, Destination: "/var/www"},
		},
		DependsOn: containers.ContainerDependsOnSpec{
			Paths:      []string{markupPayload},
			Networks:   []string{markupPayload},
			Containers: []string{markupPayload},
		},
	}

	assertInertMarkup(t, formatContainerDetail(status, spec, nil, time.Now()))
}

func TestInstanceRowCellsEscapeAtRenderSite(t *testing.T) {
	t.Parallel()

	// instanceRowCells returns the values raw; the table escapes them as it fills the cells, the
	// same way the list rows are escaped. Assert the escape survives that mapping.
	cells := xslices.Map(instanceRowCells(&containers.ContainerInstanceStatusSpec{
		Generation: 1,
		Phase:      containers.ContainerInstancePhaseFailed,
		Error:      markupPayload,
	}), tview.Escape)

	assertInertMarkup(t, strings.Join(cells, " "))
}
