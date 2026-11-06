// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package celmatch_test

import (
	"reflect"
	"sync"
	"testing"

	celgo "cel.dev/cel-go/cel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/celmatch"
	blockpb "github.com/siderolabs/talos/pkg/machinery/api/resource/definitions/block"
	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
)

// astPointer returns the pointer to the parsed AST of the expression.
func astPointer(expr cel.Expression) uintptr {
	return reflect.ValueOf(expr).FieldByName("ast").Pointer()
}

func TestExpressions(t *testing.T) {
	t.Parallel()

	volumes := []*blockpb.DiscoveredVolumeSpec{
		{PartitionLabel: "META", Name: "", Size: 1048576},
		{PartitionLabel: "META", Name: "talosmeta", Size: 1048576},
		{PartitionLabel: "META", Name: "talosmeta", Size: 2097152},
		{PartitionLabel: "META", Name: "xfs", Size: 1048576},
		{PartitionLabel: "STATE", Name: "xfs"},
		{PartitionLabel: "STATE", Name: ""},
		{PartitionLabel: "EPHEMERAL", Name: "xfs"},
		{Name: "iso9660", Label: "TALOS_V1_14"},
		{Name: "vfat", Label: "TALOS_V1_14"},
		{Name: "vfat", Label: "EFI"},
		{ParentDevPath: "/dev/sda", PartitionIndex: 1},
		{ParentDevPath: "/dev/sda", PartitionIndex: 2},
		{ParentDevPath: "/dev/sdb", PartitionIndex: 1},
	}

	disks := []map[string]any{
		{"disk": &blockpb.DiskSpec{DevPath: "/dev/sda"}, "system_disk": true},
		{"disk": &blockpb.DiskSpec{DevPath: "/dev/sdb"}, "system_disk": false},
	}

	for _, test := range []struct {
		name string
		expr cel.Expression
		env  *celgo.Env

		// the expression formerly built by formatting and parsing a string
		expected string
	}{
		{
			name:     "no match",
			expr:     celmatch.NoMatch(),
			env:      celenv.Empty(),
			expected: `false`,
		},
		{
			name:     "system disk",
			expr:     celmatch.SystemDisk(),
			env:      celenv.DiskLocator(),
			expected: `system_disk`,
		},
		{
			name:     "meta",
			expr:     celmatch.Meta(),
			env:      celenv.VolumeLocator(),
			expected: `volume.partition_label == "META" && volume.name in ["", "talosmeta"] && volume.size == 1048576u`,
		},
		{
			name:     "image cache ISO",
			expr:     celmatch.ImageCacheISO(),
			env:      celenv.VolumeLocator(),
			expected: `volume.name in ["iso9660", "vfat"] && volume.label.startsWith("TALOS_")`,
		},
		{
			name:     "partition label",
			expr:     celmatch.VolumePartitionLabel("STATE"),
			env:      celenv.VolumeLocator(),
			expected: `volume.partition_label == "STATE"`,
		},
		{
			name:     "partition label non-empty",
			expr:     celmatch.VolumePartitionLabelNonEmpty("STATE"),
			env:      celenv.VolumeLocator(),
			expected: `volume.partition_label == "STATE" && volume.name != ""`,
		},
		{
			name:     "partition index",
			expr:     celmatch.VolumePartitionIndex("/dev/sda", 2),
			env:      celenv.VolumeLocator(),
			expected: `volume.parent_dev_path == "/dev/sda" && volume.partition_index == 2u`,
		},
		{
			name:     "disk dev path",
			expr:     celmatch.DiskDevPath("/dev/sda"),
			env:      celenv.DiskLocator(),
			expected: `disk.dev_path == "/dev/sda"`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.expected, test.expr.String())

			// the built expression behaves exactly as the parsed one
			parsed, err := cel.ParseBooleanExpression(test.expected, test.env)
			require.NoError(t, err)

			var contexts []map[string]any

			switch test.env {
			case celenv.VolumeLocator():
				for _, volume := range volumes {
					contexts = append(contexts, map[string]any{"volume": volume})
				}
			case celenv.DiskLocator():
				contexts = disks
			default:
				contexts = []map[string]any{{}}
			}

			matched := 0

			for _, ctx := range contexts {
				expected, err := parsed.EvalBool(test.env, ctx)
				require.NoError(t, err)

				actual, err := test.expr.EvalBool(test.env, ctx)
				require.NoError(t, err)

				assert.Equal(t, expected, actual, "context %v", ctx)

				if actual {
					matched++
				}
			}

			if test.name != "no match" {
				assert.Positive(t, matched, "the test contexts should cover the matching case")
			}
		})
	}
}

func TestShared(t *testing.T) {
	t.Parallel()

	// the same expression is built once, and the AST is shared
	for _, build := range []func() cel.Expression{
		celmatch.NoMatch,
		celmatch.SystemDisk,
		celmatch.Meta,
		celmatch.ImageCacheISO,
		func() cel.Expression { return celmatch.VolumePartitionLabel("ETCD") },
		func() cel.Expression { return celmatch.VolumePartitionLabelNonEmpty("ETCD") },
		func() cel.Expression { return celmatch.VolumePartitionIndex("/dev/vda", 3) },
		func() cel.Expression { return celmatch.DiskDevPath("/dev/vda") },
	} {
		assert.Equal(t, astPointer(build()), astPointer(build()))
	}

	// different parameters build different expressions
	assert.NotEqual(t, astPointer(celmatch.VolumePartitionLabel("A")), astPointer(celmatch.VolumePartitionLabel("B")))
	assert.NotEqual(t, astPointer(celmatch.VolumePartitionLabel("A")), astPointer(celmatch.VolumePartitionLabelNonEmpty("A")))
	assert.NotEqual(t, astPointer(celmatch.VolumePartitionIndex("/dev/vda", 1)), astPointer(celmatch.VolumePartitionIndex("/dev/vda", 2)))

	// concurrent callers get the same expression
	var (
		wg       sync.WaitGroup
		pointers [16]uintptr
	)

	for i := range pointers {
		wg.Go(func() {
			pointers[i] = astPointer(celmatch.VolumePartitionLabel("concurrent"))
		})
	}

	wg.Wait()

	for _, p := range pointers {
		assert.Equal(t, pointers[0], p)
	}
}

func TestQuoting(t *testing.T) {
	t.Parallel()

	// values are embedded as literals, so they can't change the expression
	label := `it's" || true || "`

	expr := celmatch.VolumePartitionLabel(label)

	for _, test := range []struct {
		label    string
		expected bool
	}{
		{label: label, expected: true},
		{label: "it's", expected: false},
		{label: "OTHER", expected: false},
	} {
		matches, err := expr.EvalBool(celenv.VolumeLocator(), map[string]any{
			"volume": &blockpb.DiscoveredVolumeSpec{PartitionLabel: test.label},
		})
		require.NoError(t, err)

		assert.Equal(t, test.expected, matches, test.label)
	}
}

func TestAllocs(t *testing.T) {
	celmatch.VolumePartitionLabel("ALLOCS")

	allocs := testing.AllocsPerRun(100, func() {
		celmatch.VolumePartitionLabel("ALLOCS")
	})

	assert.Zero(t, allocs, "cached expressions should not allocate")
}
