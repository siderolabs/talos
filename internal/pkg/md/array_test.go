// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:testpackage
package md

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseDetailExport(t *testing.T) {
	detail := parseDetailExport(`MD_LEVEL=raid1
MD_DEVICES=2
MD_METADATA=1.2
MD_UUID=8d84d1c8:95d9e5bc:dc9fdd7d:51f91234
MD_DEVNAME=data
MD_NAME=talos:data
MD_RESHAPE_ACTIVE=False
MD_DEVICE_dev_sda_ROLE=0
MD_DEVICE_dev_sda_DEV=/dev/sda
MD_DEVICE_dev_sdb_ROLE=1
MD_DEVICE_dev_sdb_DEV=/dev/sdb
`)

	assert.Equal(t, "raid1", detail.Level)
	assert.Equal(t, 2, detail.RaidDevices)
	assert.Equal(t, "1.2", detail.Metadata)
	assert.Equal(t, "8d84d1c8:95d9e5bc:dc9fdd7d:51f91234", detail.UUID)
	assert.Equal(t, "data", detail.DevName)
	assert.Equal(t, "talos:data", detail.Name)
	assert.Equal(t, "False", detail.ReshapeActive)
	assert.Equal(t, []string{"/dev/sda", "/dev/sdb"}, detail.Members)
	assert.Equal(t, map[string]string{"/dev/sda": "0", "/dev/sdb": "1"}, detail.MemberRoles)
}

func TestSysfsHelpers(t *testing.T) {
	oldSysBlockDir := sysBlockDir
	sysBlockDir = t.TempDir()
	t.Cleanup(func() { sysBlockDir = oldSysBlockDir })

	require.NoError(t, os.MkdirAll(filepath.Join(sysBlockDir, "md0", "md", "dev-sda"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(sysBlockDir, "md0", "md", "array_state"), []byte("inactive\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(sysBlockDir, "md0", "md", "sync_action"), []byte("resync\n"), 0o644))

	dev, err := FindDeviceByMember("/dev/sda")
	require.NoError(t, err)
	assert.Equal(t, "/dev/md0", dev)

	inactive, err := InactiveArrays()
	require.NoError(t, err)
	assert.Equal(t, []string{"/dev/md0"}, inactive)

	state, err := ArrayStateForDevice("/dev/md0")
	require.NoError(t, err)
	assert.Equal(t, "inactive", state)

	action, err := SyncActionForDevice("/dev/md0")
	require.NoError(t, err)
	assert.Equal(t, SyncActionResync, action)

	syncing, err := IsSyncing("/dev/md0")
	require.NoError(t, err)
	assert.True(t, syncing)
}

func TestMonitorStreamsEvents(t *testing.T) {
	log := filepath.Join(t.TempDir(), "args")
	script := filepath.Join(t.TempDir(), "mdadm")
	scriptBody := `#!/bin/sh
printf '%s\n' "$*" >> "$MDADM_ARGS"
printf 'mdadm: NewArray event detected on md device /dev/md0\n'
printf 'mdadm: RebuildStarted event detected on md device /dev/md0\n'
printf 'mdadm: RebuildFinished event detected on md device /dev/md0\n'
`
	require.NoError(t, os.WriteFile(script, []byte(scriptBody), 0o755))
	t.Setenv("MDADM_ARGS", log)

	m, err := New(WithMdadmPath(script))
	require.NoError(t, err)

	var events []string

	require.NoError(t, m.Monitor(context.Background(), func(event string) {
		events = append(events, event)
	}))

	out, err := os.ReadFile(log)
	require.NoError(t, err)
	assert.Equal(t, "--monitor --scan --mail=talos@local\n", string(out))
	assert.NotContains(t, string(out), "--oneshot")
	assert.ElementsMatch(t, []string{
		"mdadm: NewArray event detected on md device /dev/md0",
		"mdadm: RebuildStarted event detected on md device /dev/md0",
		"mdadm: RebuildFinished event detected on md device /dev/md0",
	}, events)
}

func TestMonitorNoArrayIsNotFound(t *testing.T) {
	script := filepath.Join(t.TempDir(), "mdadm")
	scriptBody := `#!/bin/sh
printf 'mdadm: No array with redundancy detected, stopping\n' >&2
exit 1
`
	require.NoError(t, os.WriteFile(script, []byte(scriptBody), 0o755))

	m, err := New(WithMdadmPath(script))
	require.NoError(t, err)

	// the "no array" condition must surface only via the return value; the
	// callback must not fire for it, otherwise it gets logged as a warning on
	// every restart.
	called := false
	err = m.Monitor(context.Background(), func(string) { called = true })
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNotFound))
	assert.False(t, called, "callback must not fire for the no-array condition")
}

func TestCommandArguments(t *testing.T) {
	oldSysBlockDir := sysBlockDir
	sysBlockDir = t.TempDir()
	t.Cleanup(func() { sysBlockDir = oldSysBlockDir })

	log := filepath.Join(t.TempDir(), "args")
	script := filepath.Join(t.TempDir(), "mdadm")
	scriptBody := `#!/bin/sh
printf '%s\n' "$*" >> "$MDADM_ARGS"
case "$*" in *--detail*) printf 'MD_DEVICES=2\nMD_DEVICE_dev_sda_DEV=/dev/sda\nMD_DEVICE_dev_sdb_DEV=/dev/sdb\n' ;; esac
`
	require.NoError(t, os.WriteFile(script, []byte(scriptBody), 0o755))
	t.Setenv("MDADM_ARGS", log)

	m, err := New(WithMdadmPath(script))
	require.NoError(t, err)
	require.NoError(t, m.Add(context.Background(), "/dev/md0", "/dev/sdc"))
	require.NoError(t, m.Grow(context.Background(), "/dev/md0", 3))
	_, err = m.Create(context.Background(), "data", CreateOptions{Level: 1, RaidDevices: 2, Devices: []string{"/dev/sda", "/dev/sdb"}})
	require.NoError(t, err)

	out, err := os.ReadFile(log)
	require.NoError(t, err)
	assert.Contains(t, string(out), "--add /dev/md0 /dev/sdc")
	assert.Contains(t, string(out), "--grow /dev/md0 --raid-devices=3")
	assert.Contains(t, string(out), "--create /dev/md0 --name data --homehost=talos --run --assume-clean --level=1 --raid-devices=2 /dev/sda /dev/sdb")
}
