// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package celenv_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

func TestDiskLocator(t *testing.T) {
	t.Parallel()

	env := celenv.DiskLocator()

	for _, test := range []struct {
		name       string
		expression string
	}{
		{
			name:       "system disk",
			expression: "system_disk",
		},
		{
			name:       "disk size",
			expression: "disk.size > 1000u * GiB && !disk.rotational",
		},
		{
			name:       "glob",
			expression: "glob('sd[a-z]', disk.dev_path)",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := cel.ParseBooleanExpression(test.expression, env)
			require.NoError(t, err)
		})
	}
}

func TestVolumeLocator(t *testing.T) {
	t.Parallel()

	env := celenv.VolumeLocator()

	for _, test := range []struct {
		name       string
		expression string
	}{
		{
			name:       "by label",
			expression: "volume.label == 'EPHEMERAL'",
		},
		{
			name:       "by filesystem and size",
			expression: "volume.name == 'ext4' && volume.size > 1000u * TB",
		},
		{
			name:       "by filesystem and disk transport",
			expression: "volume.name == 'ext4' && disk.transport == 'nvme'",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := cel.ParseBooleanExpression(test.expression, env)
			require.NoError(t, err)
		})
	}
}

func TestOOMCgroupScoring(t *testing.T) {
	t.Parallel()

	env := celenv.OOMCgroupScoring()

	for _, test := range []struct {
		name       string
		expression string
	}{
		{
			name:       "example 1",
			expression: constants.DefaultOOMCgroupRankingExpression,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := cel.ParseDoubleExpression(test.expression, env)
			require.NoError(t, err)
		})
	}
}

func TestOOMTrigger(t *testing.T) {
	t.Parallel()

	env := celenv.OOMTrigger()

	for _, test := range []struct {
		name       string
		expression string
	}{
		{
			name:       "example 1",
			expression: constants.DefaultOOMTriggerExpression,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := cel.ParseBooleanExpression(test.expression, env)
			require.NoError(t, err)
		})
	}
}

func TestLinkLocator(t *testing.T) {
	t.Parallel()

	env := celenv.LinkLocator()

	for _, test := range []struct {
		name       string
		expression string
	}{
		{
			name:       "by driver",
			expression: `link.driver == "i1000e"`,
		},
		{
			name:       "by mac",
			expression: `mac(link.hardware_addr) == "00:1a:2b:3c:4d:5e"`,
		},
		{
			name:       "by mac partial",
			expression: `glob(mac(link.hardware_addr), "00:1a:2b:*")`,
		},
		{
			name:       "by altnames",
			expression: `"enx728c41bfd443" in link.alt_names`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := cel.ParseBooleanExpression(test.expression, env)
			require.NoError(t, err)
		})
	}
}
