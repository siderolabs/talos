// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:testpackage
package dashboard

import (
	"errors"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

func TestVMActionForKey(t *testing.T) {
	for key, want := range map[rune]vmAction{
		'S': vmActionStart,
		'T': vmActionStop,
		'K': vmActionForceStop,
		'R': vmActionReboot,
	} {
		got, ok := vmActionForKey(key)

		assert.True(t, ok, string(key))
		assert.Equal(t, want, got, string(key))
	}

	// The lowercase keys belong to sorting and the like.
	for _, key := range []rune{'s', 't', 'k', 'r', 'y'} {
		_, ok := vmActionForKey(key)

		assert.False(t, ok, string(key))
	}
}

func TestVMActionNeedsConfirmation(t *testing.T) {
	assert.False(t, vmActionStart.needsConfirmation())
	assert.True(t, vmActionStop.needsConfirmation())
	assert.True(t, vmActionForceStop.needsConfirmation())
	assert.True(t, vmActionReboot.needsConfirmation())
}

func TestVMActionAvailability(t *testing.T) {
	res := testVMResources() // web is running and ready, db is stopped and ready

	for _, test := range []struct {
		name   string
		vm     string
		action vmAction
		ok     bool
		reason string
	}{
		{"start running", "web", vmActionStart, false, "the VM is already running"},
		{"start stopped", "db", vmActionStart, true, ""},
		{"stop running", "web", vmActionStop, true, ""},
		{"stop stopped", "db", vmActionStop, false, "the VM is already stopped"},
		{"force stop running", "web", vmActionForceStop, true, ""},
		{"force stop stopped", "db", vmActionForceStop, false, "the VM is already stopped"},
		{"reboot running", "web", vmActionReboot, true, ""},
		{"reboot stopped", "db", vmActionReboot, false, "only a running VM that is ready can be rebooted"},
		{"gone", "gone", vmActionStart, false, "the VM is no longer present"},
	} {
		ok, reason := vmActionAvailability(res, test.vm, test.action)

		assert.Equal(t, test.ok, ok, test.name)
		assert.Equal(t, test.reason, reason, test.name)
	}
}

func TestVMActionAvailabilityStates(t *testing.T) {
	res := newVMResources()
	res.specs["vm"] = newVMSpec("vm", hypervisor.VirtualMachineSpecSpec{})

	// Not observed yet: it can be started, but there is nothing to stop or reboot.
	ok, _ := vmActionAvailability(res, "vm", vmActionStart)
	assert.True(t, ok)

	ok, _ = vmActionAvailability(res, "vm", vmActionReboot)
	assert.False(t, ok)

	// A running VM stuck on a graceful stop must still be possible to force off.
	res.statuses["vm"] = newVMStatus("vm", hypervisor.VirtualMachineStatusSpec{
		PowerState: hypervisorhelpers.VirtualMachinePowerStateRunning,
		Stage:      hypervisorhelpers.VirtualMachineStageError,
		Error:      hypervisor.GracefulStopPendingError,
	})

	ok, _ = vmActionAvailability(res, "vm", vmActionForceStop)
	assert.True(t, ok)

	ok, _ = vmActionAvailability(res, "vm", vmActionReboot)
	assert.False(t, ok, "a VM with an error is not ready to be rebooted")

	// Nothing works while the host takes the VMs down.
	res.lifecycle = hypervisor.NewVirtualMachineLifecycle(hypervisor.NamespaceName, hypervisor.VirtualMachineLifecycleID)
	res.lifecycle.TypedSpec().ShutdownDeadline = time.Now().Add(time.Minute)

	for _, action := range []vmAction{vmActionStart, vmActionStop, vmActionForceStop, vmActionReboot} {
		ok, reason := vmActionAvailability(res, "vm", action)

		assert.False(t, ok, action.String())
		assert.Equal(t, "the host is shutting down", reason)
	}
}

func TestVMActionPrompt(t *testing.T) {
	assert.Contains(t, vmActionPrompt(vmActionStop, "web", "10.0.0.1"), `VM "web" on node 10.0.0.1`)
	assert.Contains(t, vmActionPrompt(vmActionForceStop, "web", ""), "pulling the plug")
	assert.NotContains(t, vmActionPrompt(vmActionReboot, "web", ""), "on node")
}

func TestVMActionResult(t *testing.T) {
	assert.Equal(t, "[green]stop requested for web[-]", vmActionResult(vmActionStop, "web", nil))

	assert.Contains(t, vmActionResult(vmActionReboot, "web", errors.New("boom")), "reboot web failed: boom")

	denied := vmActionResult(vmActionStart, "web", status.Error(codes.PermissionDenied, "nope"))
	assert.Contains(t, denied, "os:operator or os:admin")
	assert.NotContains(t, denied, "sensitive", "this is not about a resource")
}

func TestJoinMessage(t *testing.T) {
	assert.Equal(t, "2 VMs", joinMessage("2 VMs", ""))
	assert.Equal(t, "2 VMs · done", joinMessage("2 VMs", "done"))
}

func TestVMActionLabel(t *testing.T) {
	assert.Equal(t, "Start", vmActionStart.label())
	assert.Equal(t, "Stop", vmActionStop.label())
	assert.Equal(t, "Force stop", vmActionForceStop.label())
	assert.Equal(t, "Reboot", vmActionReboot.label())
}

// reversedColumns draws the modal and returns the columns of the cells drawn reversed, which is
// how the focused button is marked, and whether every cell kept to the terminal colors.
func reversedColumns(t *testing.T, modal *tview.Modal) ([]int, bool) {
	t.Helper()

	screen := tcell.NewSimulationScreen("UTF-8")
	require.NoError(t, screen.Init())
	t.Cleanup(screen.Fini)

	screen.SetSize(100, 30)
	modal.Draw(screen)
	screen.Show()

	cells, width, _ := screen.GetContents()

	var (
		columns      []int
		terminalOnly = true
	)

	for i, cell := range cells {
		fg, bg, attrs := cell.Style.Decompose()

		if fg != tcell.ColorDefault || bg != tcell.ColorDefault {
			terminalOnly = false
		}

		if attrs&tcell.AttrReverse != 0 {
			columns = append(columns, i%width)
		}
	}

	return columns, terminalOnly
}

func TestVMConfirmModal(t *testing.T) {
	modal := newVMConfirmModal("Stop the VM?", vmActionStop.label())

	app := tview.NewApplication()
	app.SetFocus(modal)

	send := func(key tcell.Key) {
		modal.InputHandler()(tcell.NewEventKey(key, 0, tcell.ModNone), func(p tview.Primitive) { app.SetFocus(p) })
	}

	cancel, terminalOnly := reversedColumns(t, modal)

	assert.True(t, terminalOnly, "the modal must follow the terminal colors")
	require.NotEmpty(t, cancel, "the focused button must be visibly marked")

	var done []int

	modal.SetDoneFunc(func(index int, _ string) { done = append(done, index) })

	send(tcell.KeyRight)

	confirm, _ := reversedColumns(t, modal)

	require.NotEmpty(t, confirm)
	assert.Greater(t, confirm[0], cancel[len(cancel)-1], "Right must move the mark to the confirm button")

	send(tcell.KeyRight)

	wrapped, _ := reversedColumns(t, modal)

	assert.Equal(t, cancel, wrapped, "Right on the last button wraps around")

	send(tcell.KeyLeft)

	left, _ := reversedColumns(t, modal)

	assert.Equal(t, confirm, left, "Left moves back")

	send(tcell.KeyEnter)

	assert.Equal(t, []int{1}, done, "Enter picks the focused button")
}

func TestOwnsNavigationKeys(t *testing.T) {
	assert.True(t, ownsNavigationKeys(tview.NewInputField()), "a filter field types the keys")
	assert.True(t, ownsNavigationKeys(tview.NewButton("OK")), "a dialog moves between buttons with the arrows")
	assert.False(t, ownsNavigationKeys(tview.NewTable()), "elsewhere the arrows and h/l switch nodes")
	assert.False(t, ownsNavigationKeys(nil))
}
