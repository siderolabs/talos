// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:testpackage // We want to test unexported functions, and the test code is not large enough to justify a separate package.
package lvm

import (
	_ "embed"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//go:embed testdata/lvs.json
var lvsOut string

//go:embed testdata/lvs.empty.json
var lvsEmptyOut string

func TestParseLVS(t *testing.T) {
	t.Parallel()

	t.Run("real data", func(t *testing.T) {
		t.Parallel()

		lvs, err := parseLVS(lvsOut)
		require.NoError(t, err)
		require.Len(t, lvs, 1)

		// Sample captured against a real cluster: vgdata has a single
		// inactive linear LV "lvdata" — kernel major/minor and several
		// tri-state columns therefore come back as "-1" / "unknown" / "auto".
		lv := lvs[0]
		assert.Equal(t, "lvdata", lv.Name)
		assert.Equal(t, "vgdata/lvdata", lv.FullName)
		assert.Equal(t, "/dev/vgdata/lvdata", lv.Path)
		assert.Equal(t, "/dev/mapper/vgdata-lvdata", lv.DMPath)
		assert.Equal(t, "vgdata", lv.VGName)
		assert.Equal(t, "CzCFrN-thB1-xeiZ-41NV-yrbt-IK8e-t5T6uQ", lv.UUID)
		assert.Equal(t, "linear", lv.Layout)
		assert.Equal(t, "public", lv.Role)
		assert.Equal(t, "unknown", lv.Permissions)
		assert.Equal(t, "inherit", lv.AllocationPolicy)
		assert.Equal(t, "32199671808", lv.Size)
		// "auto" / "-1" / "unknown" sentinels preserved verbatim.
		assert.Equal(t, "auto", lv.ReadAhead)
		assert.Equal(t, "-1", lv.KernelMajor)
		assert.Equal(t, "-1", lv.KernelMinor)
		assert.Equal(t, "unknown", lv.Active)
		assert.Equal(t, "unknown", lv.ActiveLocally)
		assert.Equal(t, "unknown", lv.ActiveRemotely)
		assert.Equal(t, "unknown", lv.ActiveExclusively)
		assert.Equal(t, "unknown", lv.Suspended)
		assert.Equal(t, "unknown", lv.DeviceOpen)
		assert.Empty(t, lv.AllocationLocked)
		assert.Empty(t, lv.FixedMinor)
		assert.Empty(t, lv.SkipActivation)
		assert.Empty(t, lv.Merging)
		assert.Empty(t, lv.Converting)
		assert.Empty(t, lv.Origin)
		assert.Empty(t, lv.PoolLV)
		assert.Empty(t, lv.WhenFull)
		assert.Empty(t, lv.MetadataSize)
		assert.Nil(t, lv.Tags)
	})

	t.Run("empty data", func(t *testing.T) {
		t.Parallel()

		lvs, err := parseLVS(lvsEmptyOut)
		require.NoError(t, err)
		assert.Empty(t, lvs)
	})

	t.Run("invalid data", func(t *testing.T) {
		t.Parallel()

		_, err := parseLVS("invalid json")
		require.Error(t, err)
	})
}

//go:embed testdata/vgs.json
var vgsOut string

//go:embed testdata/vgs.empty.json
var vgsEmptyOut string

func TestParseVGS(t *testing.T) {
	t.Parallel()

	t.Run("real data", func(t *testing.T) {
		t.Parallel()

		vgs, err := parseVGS(vgsOut)
		require.NoError(t, err)
		require.Len(t, vgs, 1)

		vg := vgs[0]
		assert.Equal(t, "vgdata", vg.Name)
		assert.Equal(t, "3vd0QP-LDvZ-2CU7-6iwx-wCUR-opYk-zphuYg", vg.UUID)
		assert.Equal(t, "lvm2", vg.Format)
		assert.Equal(t, "writeable", vg.Permissions)
		assert.Equal(t, "extendable", vg.Extendable)
		assert.Empty(t, vg.Exported)
		assert.Empty(t, vg.Partial)
		assert.Equal(t, "normal", vg.AllocationPolicy)
		assert.Empty(t, vg.Clustered)
		assert.Empty(t, vg.Shared)
		assert.Equal(t, "32199671808", vg.Size)
		assert.Equal(t, "0", vg.Free)
		assert.Equal(t, "4194304", vg.ExtentSize)
		assert.Equal(t, "7677", vg.ExtentCount)
		assert.Equal(t, "0", vg.FreeExtentCount)
		assert.Equal(t, "0", vg.MaxLV)
		assert.Equal(t, "0", vg.MaxPV)
		assert.Equal(t, "1", vg.LVCount)
		assert.Equal(t, "3", vg.PVCount)
		assert.Equal(t, "0", vg.SnapCount)
		assert.Equal(t, "0", vg.MissingPVCount)
		assert.Equal(t, "2", vg.SeqNo)
		assert.Empty(t, vg.LockType)
		assert.Empty(t, vg.SystemID)
		assert.Nil(t, vg.Tags)
	})

	t.Run("empty data", func(t *testing.T) {
		t.Parallel()

		vgs, err := parseVGS(vgsEmptyOut)
		require.NoError(t, err)
		assert.Empty(t, vgs)
	})

	t.Run("invalid data", func(t *testing.T) {
		t.Parallel()

		_, err := parseVGS("invalid json")
		require.Error(t, err)
	})
}

//go:embed testdata/pvs.json
var pvsOut string

//go:embed testdata/pvs.empty.json
var pvsEmptyOut string

func TestParsePVS(t *testing.T) {
	t.Parallel()

	t.Run("real data", func(t *testing.T) {
		t.Parallel()

		pvs, err := parsePVS(pvsOut)
		require.NoError(t, err)
		// `pvs -a` enumerates every block device, not only PVs: the sample
		// has 4 non-LVM block devices (loop0, vda1, vda3, vda4) plus 3 real
		// PVs (vdb, vdc, vdd) backing vgdata.
		require.Len(t, pvs, 7)

		// Bare block device: empty UUID/Format/VGName, Allocatable/InUse blank.
		bare := pvs[0]
		assert.Equal(t, "/dev/loop0", bare.Device)
		assert.Empty(t, bare.UUID)
		assert.Empty(t, bare.Format)
		assert.Empty(t, bare.VGName)
		assert.Equal(t, "0", bare.Size)
		assert.Equal(t, "84463616", bare.DeviceSize)
		assert.Empty(t, bare.Allocatable)
		assert.Empty(t, bare.InUse)

		// Real PV backing vgdata.
		actual := pvs[4]
		assert.Equal(t, "/dev/vdb", actual.Device)
		assert.Equal(t, "vgdata", actual.VGName)
		assert.Equal(t, "lvm2", actual.Format)
		assert.Equal(t, "QnHa9I-VntT-FHR1-oqhL-qj1i-eEpD-dvRwtw", actual.UUID)
		assert.Equal(t, "allocatable", actual.Allocatable)
		assert.Empty(t, actual.Exported)
		assert.Empty(t, actual.Missing)
		assert.Equal(t, "used", actual.InUse)
		assert.Equal(t, "10733223936", actual.Size)
		assert.Equal(t, "10737418240", actual.DeviceSize)
		assert.Equal(t, "0", actual.Free)
		assert.Equal(t, "10733223936", actual.Used)
		assert.Equal(t, "2559", actual.PECount)
		assert.Equal(t, "2559", actual.PEAllocCount)
		assert.Equal(t, "251", actual.Major)
		assert.Equal(t, "16", actual.Minor)
		assert.Nil(t, actual.Tags)

		// All 4 non-LVM rows must be filterable on UUID alone — the
		// controller relies on this to avoid emitting a resource per disk.
		nonLVM := 0

		for _, pv := range pvs {
			if pv.UUID == "" {
				nonLVM++
			}
		}

		assert.Equal(t, 4, nonLVM)
	})

	t.Run("empty data", func(t *testing.T) {
		t.Parallel()

		// pvs.empty.json captures a node with no LVM PVs but several
		// non-LVM block devices, so the slice is non-empty yet every entry
		// has an empty UUID.
		pvs, err := parsePVS(pvsEmptyOut)
		require.NoError(t, err)
		require.NotEmpty(t, pvs)

		for _, pv := range pvs {
			assert.Empty(t, pv.UUID, "device %s should not be a real PV", pv.Device)
			assert.Empty(t, pv.Format, "device %s should not be a real PV", pv.Device)
		}
	})

	t.Run("invalid data", func(t *testing.T) {
		t.Parallel()

		_, err := parsePVS("invalid json")
		require.Error(t, err)
	})
}

func TestTagsUnmarshal(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		in   string
		want []string
	}{
		{`""`, nil},
		{`"a"`, []string{"a"}},
		{`"a,b"`, []string{"a", "b"}},
		{`" a , b , "`, []string{"a", "b"}},
		{`null`, nil},
	} {
		var v Tags

		require.NoError(t, json.Unmarshal([]byte(tt.in), &v), "input %q", tt.in)
		assert.Equal(t, tt.want, []string(v), "input %q", tt.in)
	}
}
