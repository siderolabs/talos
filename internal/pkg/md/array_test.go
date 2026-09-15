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
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
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

func TestInSyncMembers(t *testing.T) {
	oldSysBlockDir := sysBlockDir
	sysBlockDir = t.TempDir()
	t.Cleanup(func() { sysBlockDir = oldSysBlockDir })

	mdDir := filepath.Join(sysBlockDir, "md0", "md")

	for member, state := range map[string]string{
		"sdb":     "in_sync\n",
		"sda":     "in_sync,writemostly\n",
		"sdc":     "spare\n",
		"nvme0n1": "faulty\n",
	} {
		require.NoError(t, os.MkdirAll(filepath.Join(mdDir, "dev-"+member), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(mdDir, "dev-"+member, "state"), []byte(state), 0o644))
	}

	require.NoError(t, os.MkdirAll(filepath.Join(sysBlockDir, "sda"), 0o755))

	// the device node is looked up in sysfs by the name it resolves to
	devDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(devDir, "md0"), nil, 0o644))
	require.NoError(t, os.Symlink(filepath.Join(devDir, "md0"), filepath.Join(devDir, "md-name-talos:boot")))

	for _, device := range []string{filepath.Join(devDir, "md0"), filepath.Join(devDir, "md-name-talos:boot")} {
		members, err := InSyncMembers(device)
		require.NoError(t, err)
		assert.Equal(t, []string{"/dev/sda", "/dev/sdb"}, members)
	}

	require.NoError(t, os.WriteFile(filepath.Join(devDir, "sda"), nil, 0o644))

	_, err := InSyncMembers(filepath.Join(devDir, "sda"))
	assert.ErrorIs(t, err, ErrNotFound)

	_, err = InSyncMembers(filepath.Join(devDir, "missing"))
	assert.Error(t, err)
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

// stderrMonitorScript emits recognized events and one ordinary line on fd 2,
// then blocks until the test writes a terminal action to the control FIFO.
// Only shell builtins are used: the producer stays alive in read, with no
// sleep descendant holding its output pipes during cancellation.
const stderrMonitorScript = `#!/bin/sh
printf '%s\n' "$*" > "$MDADM_ARGS"
exec 3< "$MDADM_CONTROL"
printf 'mdadm: PRIVATE_NON_EVENT_CANARY\n' >&2
printf 'mdadm: NewArray event detected on md device /dev/md0\n' >&2
printf 'mdadm: RebuildStarted event detected on md device /dev/md0\n' >&2
printf 'mdadm: RebuildFinished event det' >&2
printf 'ected on md device /dev/md0\n' >&2
read -r terminal <&3
case "$terminal" in
  command-error) printf 'mdadm: invalid option PRIVATE_ERROR_CANARY\n' >&2; exit 17 ;;
  exit) exit 0 ;;
  *) exit 18 ;;
esac
`

type stderrMonitorRun struct {
	dir     string
	control *os.File
	cancel  context.CancelFunc
	events  chan string
	done    chan struct{}
	err     error
}

func startStderrMonitor(t *testing.T) *stderrMonitorRun {
	t.Helper()

	run := &stderrMonitorRun{
		dir:    t.TempDir(),
		events: make(chan string, 8),
		done:   make(chan struct{}),
	}

	controlPath := filepath.Join(run.dir, "control")
	require.NoError(t, unix.Mkfifo(controlPath, 0o600))

	control, err := os.OpenFile(controlPath, os.O_RDWR, 0)
	require.NoError(t, err)

	run.control = control

	t.Cleanup(func() { require.NoError(t, control.Close()) })
	t.Setenv("MDADM_CONTROL", controlPath)
	t.Setenv("MDADM_ARGS", filepath.Join(run.dir, "args"))

	script := filepath.Join(run.dir, "mdadm")
	require.NoError(t, os.WriteFile(script, []byte(stderrMonitorScript), 0o700))

	monitor, err := New(WithMdadmPath(script))
	require.NoError(t, err)

	var ctx context.Context

	ctx, run.cancel = context.WithCancel(context.Background())

	go func() {
		defer close(run.done)

		run.err = monitor.Monitor(ctx, func(event string) { run.events <- event })
	}()

	t.Cleanup(func() {
		run.cancel()

		select {
		case <-run.done:
		case <-time.After(3 * time.Second):
			t.Error("monitor did not stop after cancellation")
		}
	})

	return run
}

func (run *stderrMonitorRun) requireLiveEvents(t *testing.T) {
	t.Helper()

	for _, event := range []string{"NewArray", "RebuildStarted", "RebuildFinished"} {
		select {
		case actual := <-run.events:
			require.Equal(t, "mdadm: "+event+" event detected on md device /dev/md0", actual)
		case <-run.done:
			t.Fatal("producer exited before release")
		case <-time.After(3 * time.Second):
			t.Fatal("stderr event was not delivered while producer remained alive")
		}
	}

	select {
	case <-run.done:
		t.Fatal("producer must still be blocked before release")
	default:
	}
}

func (run *stderrMonitorRun) release(t *testing.T, terminal string) {
	t.Helper()

	if terminal == "cancel" {
		run.cancel()
	} else {
		_, err := run.control.WriteString(terminal + "\n")
		require.NoError(t, err)
	}

	select {
	case <-run.done:
	case <-time.After(3 * time.Second):
		t.Fatal("monitor did not return after release")
	}
}

func TestMonitorStreamsStderrBeforeExit(t *testing.T) {
	for _, terminal := range []string{"exit", "command-error", "cancel"} {
		t.Run(terminal, func(t *testing.T) {
			run := startStderrMonitor(t)
			run.requireLiveEvents(t)

			args, err := os.ReadFile(filepath.Join(run.dir, "args"))
			require.NoError(t, err)
			require.Equal(t, "--monitor --scan --mail=talos@local\n", string(args))

			run.release(t, terminal)

			if terminal == "command-error" {
				require.ErrorIs(t, run.err, ErrInvalidArgument)

				var classified *ExecError

				require.ErrorAs(t, run.err, &classified)
				assert.Equal(t, 17, classified.ExitCode)
				assert.Contains(t, string(classified.Stderr), "PRIVATE_ERROR_CANARY")
				assert.NotContains(t, run.err.Error(), "PRIVATE_ERROR_CANARY")
			} else {
				require.NoError(t, run.err)
			}

			assert.Empty(t, run.events, "ordinary stderr must not reach the callback")
		})
	}
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

func newScriptMonitor(t *testing.T, body string) *MD {
	t.Helper()

	script := filepath.Join(t.TempDir(), "mdadm")
	require.NoError(t, os.WriteFile(script, []byte("#!/bin/sh\n"+body), 0o700))

	m, err := New(WithMdadmPath(script))
	require.NoError(t, err)

	return m
}

func TestMonitorIgnoresMalformedStderr(t *testing.T) {
	t.Setenv("MDADM_LONG", strings.Repeat("x", 1024))

	// Only the well-formed DegradedArray line is an event. The overlong line
	// carries the token past the line limit, so it must not count either.
	m := newScriptMonitor(t, `
printf '\n' >&2
printf '\001\377\376garbage\033[0m\n' >&2
printf 'mdadm: event detected\n' >&2
printf 'mdadm:Fail event detected\n' >&2
printf 'mdadm: Fail EVENT DETECTED on md device /dev/md1\n' >&2
i=0
while [ $i -lt 64 ]; do printf '%s' "$MDADM_LONG" >&2; i=$((i+1)); done
printf ' event detected on md device /dev/md2\n' >&2
printf 'mdadm: DegradedArray event detected on md device /dev/md0\n' >&2
printf 'unterminated garbage' >&2
`)

	var events []string

	require.NoError(t, m.Monitor(context.Background(), func(event string) {
		events = append(events, event)
	}))

	assert.Equal(t, []string{"mdadm: DegradedArray event detected on md device /dev/md0"}, events)
}

func TestMonitorStderrTailIsBounded(t *testing.T) {
	t.Setenv("MDADM_LONG", strings.Repeat("y", 1023))

	m := newScriptMonitor(t, `
printf 'mdadm: PRIVATE_FIRST_LINE\n' >&2
i=0
while [ $i -lt 256 ]; do printf 'filler %s\n' "$MDADM_LONG" >&2; i=$((i+1)); done
printf 'mdadm: No array with redundancy detected, stopping\n' >&2
exit 1
`)

	called := false
	err := m.Monitor(context.Background(), func(string) { called = true })
	require.ErrorIs(t, err, ErrNotFound)
	assert.False(t, called)

	var classified *ExecError

	require.ErrorAs(t, err, &classified)
	assert.Equal(t, 1, classified.ExitCode)
	assert.LessOrEqual(t, len(classified.Stderr), 64*1024)
	assert.True(t, strings.HasPrefix(string(classified.Stderr), "filler "), "the tail must start on a line boundary")
	assert.True(t, strings.HasSuffix(string(classified.Stderr), "mdadm: No array with redundancy detected, stopping\n"))
	assert.NotContains(t, string(classified.Stderr), "PRIVATE_FIRST_LINE")
}

func TestMonitorRestartDoesNotRepeatEvents(t *testing.T) {
	t.Setenv("MDADM_STARTED", filepath.Join(t.TempDir(), "started"))

	// The first run ends inside a line. The second run must neither replay the
	// first run's events nor complete its partial line.
	m := newScriptMonitor(t, `
if [ ! -e "$MDADM_STARTED" ]; then
  : > "$MDADM_STARTED"
  printf 'mdadm: RebuildStarted event detected on md device /dev/md0\n' >&2
  printf 'mdadm: RebuildFinished event det' >&2
  exit 0
fi
printf 'ected on md device /dev/md0\n' >&2
printf 'mdadm: RebuildFinished event detected on md device /dev/md0\n' >&2
`)

	var events []string

	onEvent := func(event string) { events = append(events, event) }

	require.NoError(t, m.Monitor(context.Background(), onEvent))
	require.NoError(t, m.Monitor(context.Background(), onEvent))

	assert.Equal(t, []string{
		"mdadm: RebuildStarted event detected on md device /dev/md0",
		"mdadm: RebuildFinished event detected on md device /dev/md0",
	}, events)
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
