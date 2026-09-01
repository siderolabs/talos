// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dashboard

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/api/machine"
)

func TestBuildAPIContainerRows(t *testing.T) {
	infos := []*machine.ContainerInfo{
		{Namespace: "k8s.io", Id: "kube-system/coredns/coredns", PodId: "kube-system/coredns", Image: "coredns:1.11", Pid: 2010, Status: "RUNNING"},
		{Namespace: "k8s.io", Id: "kube-system/coredns", PodId: "kube-system/coredns", Image: "pause:3.8", Pid: 2000, Status: "SANDBOX_READY"},
	}

	stats := map[string]*machine.Stat{
		"kube-system/coredns/coredns": {Id: "kube-system/coredns/coredns", MemoryUsage: 16 << 20},
	}

	cpuPercent := map[string]float64{"kube-system/coredns/coredns": 1.5}

	rows := buildAPIContainerRows(infos, stats, cpuPercent, "")
	require.Len(t, rows, 2)

	// The sandbox sorts before the container it holds, and only the container is indented.
	assert.Equal(t, "kube-system/coredns", rows[0].ID)
	assert.False(t, rows[0].Nested)
	assert.Equal(t, "kube-system/coredns", rows[0].displayID())
	assert.Equal(t, metricUnknown, formatAPIMemory(rows[0]))
	assert.Equal(t, metricUnknown, formatCPUPercent(rows[0].CPUPercent))

	assert.True(t, rows[1].Nested)
	assert.Equal(t, "└─ kube-system/coredns/coredns", rows[1].displayID())
	assert.Equal(t, "1.5%", formatCPUPercent(rows[1].CPUPercent))
	assert.Equal(t, "17 MB", formatAPIMemory(rows[1]))
}

func TestBuildAPIContainerRowsFilter(t *testing.T) {
	infos := []*machine.ContainerInfo{
		{Id: "apid", Image: "talos:v1.13", Status: "RUNNING"},
		{Id: "trustd", Image: "talos:v1.13", Status: "STOPPED"},
	}

	rows := buildAPIContainerRows(infos, nil, nil, "stopped")
	require.Len(t, rows, 1)
	assert.Equal(t, "trustd", rows[0].ID)

	rows = buildAPIContainerRows(infos, nil, nil, "TALOS:")
	assert.Len(t, rows, 2)
}

func TestAPICPUPercent(t *testing.T) {
	previous := map[string]uint64{
		"steady":    uint64(10 * time.Second),
		"restarted": uint64(30 * time.Second),
	}

	current := map[string]uint64{
		"steady":    uint64(11 * time.Second),
		"restarted": uint64(time.Second),
		"new":       uint64(5 * time.Second),
	}

	percent := apiCPUPercent(previous, current, 5*time.Second)

	// One second of CPU time over five seconds is 20% of a core.
	assert.InDelta(t, 20.0, percent["steady"], 0.001)

	// A container whose counter went backwards was restarted, and one seen for the first time has
	// nothing to compare against: neither reports a usage rather than reporting a spike.
	_, ok := percent["restarted"]
	assert.False(t, ok)

	_, ok = percent["new"]
	assert.False(t, ok)

	assert.Nil(t, apiCPUPercent(previous, current, 0))
}

func TestFormatAPIContainerDetail(t *testing.T) {
	detail := formatAPIContainerDetail(apiContainerRow{
		Namespace:   "system",
		ID:          "apid",
		Image:       "talos:v1.13",
		Status:      "RUNNING",
		PID:         1200,
		CPUPercent:  math.NaN(),
		Memory:      32 << 20,
		MemoryKnown: true,
	})

	assert.Contains(t, detail, "system")
	assert.Contains(t, detail, "talos:v1.13")
	assert.Contains(t, detail, "1200")
	assert.Contains(t, detail, "34 MB")
	assert.Contains(t, detail, metricUnknown)
}
