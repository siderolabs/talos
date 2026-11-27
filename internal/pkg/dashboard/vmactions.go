// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dashboard

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"google.golang.org/grpc/codes"

	"github.com/siderolabs/talos/internal/pkg/dashboard/utils"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
)

const (
	pageVMConfirm = "vm-confirm"

	// vmActionTimeout bounds a power request: the API returns before the VM changes state, so it
	// only has to cover the request itself.
	vmActionTimeout = 30 * time.Second

	// vmMessageTTL is how long the outcome of an action stays on screen.
	vmMessageTTL = 8 * time.Second
)

// vmAction is a power operation the screen can ask the hypervisor for.
type vmAction int

// VM power operations.
const (
	vmActionStart vmAction = iota
	vmActionStop
	vmActionForceStop
	vmActionReboot
)

// String renders the action as a verb phrase.
func (action vmAction) String() string {
	switch action {
	case vmActionStart:
		return "start"
	case vmActionStop:
		return "stop"
	case vmActionForceStop:
		return "force stop"
	case vmActionReboot:
		return "reboot"
	default:
		return "unknown"
	}
}

// label renders the action as a capitalized button label.
func (action vmAction) label() string {
	name := action.String()

	return strings.ToUpper(name[:1]) + name[1:]
}

// needsConfirmation reports whether the action interrupts a running guest, as opposed to starting one.
func (action vmAction) needsConfirmation() bool {
	return action != vmActionStart
}

// vmActionForKey maps a key to the action it triggers.
func vmActionForKey(key rune) (vmAction, bool) {
	switch key {
	case 'S':
		return vmActionStart, true
	case 'T':
		return vmActionStop, true
	case 'K':
		return vmActionForceStop, true
	case 'R':
		return vmActionReboot, true
	default:
		return 0, false
	}
}

// vmActionAvailability reports whether the action makes sense for the VM right now, and if not,
// why. The API accepts most of these anyway; asking for what cannot happen is only noise.
func vmActionAvailability(res *vmResources, name string, action vmAction) (bool, string) {
	switch {
	case res.specs[name] == nil && res.statuses[name] == nil:
		return false, "the VM is no longer present"
	case res.shuttingDown():
		return false, "the host is shutting down"
	}

	if reason := vmActionStateObstacle(buildVMRow(res, name).Health, action); reason != "" {
		return false, reason
	}

	return true, ""
}

// vmActionStateObstacle returns why the action is pointless for a VM of the given health, or an
// empty string when it is not.
func vmActionStateObstacle(health vmHealth, action vmAction) string {
	switch action {
	case vmActionStart:
		if health == vmHealthRunning {
			return "the VM is already running"
		}
	case vmActionStop, vmActionForceStop:
		if health == vmHealthStopped {
			return "the VM is already stopped"
		}
	case vmActionReboot:
		if health != vmHealthRunning {
			return "only a running VM that is ready can be rebooted"
		}
	}

	return ""
}

// vmActionPrompt is the question asked before the action is carried out.
func vmActionPrompt(action vmAction, name, node string) string {
	target := fmt.Sprintf("VM %q", name)
	if node != "" {
		target += " on node " + node
	}

	switch action {
	case vmActionStop:
		return fmt.Sprintf("Ask the guest of %s to power off?", target)
	case vmActionForceStop:
		return fmt.Sprintf("Force %s off? The guest gets no chance to shut down, like pulling the plug.", target)
	case vmActionReboot:
		return fmt.Sprintf("Reboot %s?", target)
	case vmActionStart:
		return fmt.Sprintf("Start %s?", target)
	default:
		return ""
	}
}

// vmActionResult is the message shown once the request has been made.
func vmActionResult(action vmAction, name string, err error) string {
	if err == nil {
		return colored("green", fmt.Sprintf("%s requested for %s", action, tview.Escape(name)))
	}

	return colored("red", fmt.Sprintf("%s %s failed: %s", action, tview.Escape(name), tview.Escape(formatActionError(err))))
}

// formatActionError renders an error of a power request.
func formatActionError(err error) string {
	if client.StatusCode(err) == codes.PermissionDenied {
		return "permission denied: power operations require the os:operator or os:admin role"
	}

	return err.Error()
}

// requestVMPower makes the request to the hypervisor.
//
// The request returns before the VM reaches the state, which shows up in the watched status.
func requestVMPower(ctx context.Context, cli *client.Client, node, name string, action vmAction) error {
	ctx, cancel := context.WithTimeout(utils.NodeContext(ctx, node), vmActionTimeout)
	defer cancel()

	service := cli.HypervisorService()

	var err error

	switch action {
	case vmActionStart:
		_, err = service.Start(ctx, &machine.VirtualMachineStartRequest{Name: name})
	case vmActionStop:
		_, err = service.Stop(ctx, &machine.VirtualMachineStopRequest{Name: name})
	case vmActionForceStop:
		_, err = service.Stop(ctx, &machine.VirtualMachineStopRequest{Name: name, Force: true})
	case vmActionReboot:
		_, err = service.Reboot(ctx, &machine.VirtualMachineRebootRequest{Name: name})
	}

	return err
}

// vmTargetName returns the VM the action keys apply to: the one drilled into, or the one under
// the cursor on the list.
func (widget *VMsGrid) vmTargetName() string {
	if widget.level == 1 {
		return widget.selected
	}

	return widget.selectedName()
}

// runAction handles an action key: it checks the action is available, asks for confirmation when
// it interrupts the guest, and then carries it out.
func (widget *VMsGrid) runAction(action vmAction) {
	name := widget.vmTargetName()
	if name == "" {
		return
	}

	if ok, reason := vmActionAvailability(widget.res, name, action); !ok {
		widget.setMessage(colored("yellow", fmt.Sprintf("cannot %s %s: %s", action, tview.Escape(name), reason)))

		return
	}

	if !action.needsConfirmation() {
		widget.performAction(action, name, widget.selectedNode)

		return
	}

	widget.confirmAction(action, name, widget.selectedNode)
}

// confirmAction shows a modal over the screen and carries the action out on the node it was asked on when it is accepted.
func (widget *VMsGrid) confirmAction(action vmAction, name, node string) {
	previous := widget.app.GetFocus()

	modal := newVMConfirmModal(vmActionPrompt(action, name, node), action.label())

	modal.SetDoneFunc(func(buttonIndex int, _ string) {
		widget.dismissConfirm()
		widget.app.SetFocus(previous)

		if buttonIndex == 1 {
			widget.performAction(action, name, node)
		}
	})

	widget.confirming = true
	widget.pages.AddPage(pageVMConfirm, modal, true, true)
	widget.app.SetFocus(modal)
}

// newVMConfirmModal builds the confirmation modal, with Cancel focused first so that a stray Enter
// does not interrupt a guest.
//
// tview's modal is blue, and marks the focused button with colors which turn invisible once the
// dashboard sets the primitive colors to the terminal defaults. This one keeps to the terminal
// colors and shows the focused button reversed, like the selected row of a table. Left and Right
// move between the buttons, as Tab and Shift-Tab already do.
func newVMConfirmModal(prompt, confirmLabel string) *tview.Modal {
	modal := tview.NewModal().
		SetText(tview.Escape(prompt)).
		SetTextColor(tcell.ColorDefault).
		SetBackgroundColor(tcell.ColorDefault).
		SetButtonStyle(tcell.StyleDefault).
		SetButtonActivatedStyle(tcell.StyleDefault.Attributes(tcell.AttrReverse)).
		AddButtons([]string{"Cancel", confirmLabel})

	modal.Box.SetBackgroundColor(tcell.ColorDefault)

	modal.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() { //nolint:exhaustive
		case tcell.KeyLeft:
			return tcell.NewEventKey(tcell.KeyBacktab, 0, tcell.ModNone)
		case tcell.KeyRight:
			return tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone)
		default:
			return event
		}
	})

	return modal
}

// dismissConfirm removes the confirmation modal, if shown.
func (widget *VMsGrid) dismissConfirm() {
	if widget.confirming {
		widget.confirming = false
		widget.pages.RemovePage(pageVMConfirm)
	}
}

// performAction makes the request in the background and reports the outcome on the screen.
func (widget *VMsGrid) performAction(action vmAction, name, node string) {
	widget.setMessage(fmt.Sprintf("%s requested for %s...", action, tview.Escape(name)))

	go func() {
		err := requestVMPower(widget.ctx, widget.dashboard.cli, node, name, action)

		widget.app.QueueUpdate(func() {
			// The user may have moved to another node meanwhile; the message is about this one.
			if node != widget.selectedNode {
				return
			}

			widget.setMessage(vmActionResult(action, name, err))
		})
	}()
}

// setMessage shows a message next to the summary and in the detail title for a while.
func (widget *VMsGrid) setMessage(message string) {
	widget.messageSeq++
	seq := widget.messageSeq

	widget.message = message
	widget.renderMessage()

	time.AfterFunc(vmMessageTTL, func() {
		widget.app.QueueUpdate(func() {
			if seq != widget.messageSeq {
				return
			}

			widget.message = ""
			widget.renderMessage()
		})
	})
}

// renderMessage redraws the parts of the screen that show the message.
func (widget *VMsGrid) renderMessage() {
	switch widget.level {
	case 0:
		widget.renderList()
	case 1:
		widget.renderDetailView()
	}
}

// joinMessage appends the message of the last action to a summary line.
func joinMessage(summary, message string) string {
	if message == "" {
		return summary
	}

	return summary + " · " + message
}
