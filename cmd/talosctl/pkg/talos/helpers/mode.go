// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package helpers

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/siderolabs/gen/maps"
	"github.com/spf13/cobra"

	"github.com/siderolabs/talos/pkg/cli"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
)

// InteractiveMode fake mode value for the interactive config mode.
// Should be never passed to the API.
const InteractiveMode machine.ApplyConfigurationRequest_Mode = -1

// Mode apply, patch, edit config update mode.
type Mode struct {
	options map[string]machine.ApplyConfigurationRequest_Mode
	Mode    machine.ApplyConfigurationRequest_Mode
}

func (m Mode) String() string {
	switch m.Mode {
	case machine.ApplyConfigurationRequest_TRY:
		return modeTry
	case machine.ApplyConfigurationRequest_AUTO:
		return modeAuto
	case machine.ApplyConfigurationRequest_NO_REBOOT:
		return modeNoReboot
	case machine.ApplyConfigurationRequest_REBOOT:
		return modeReboot
	case machine.ApplyConfigurationRequest_STAGED:
		return modeStaged
	case InteractiveMode:
		return modeInteractive
	default:
		return modeAuto
	}
}

// Set implements Flag interface.
func (m *Mode) Set(value string) error {
	mode, ok := m.options[value]
	if !ok {
		return fmt.Errorf("possible options are: %s", m.Type())
	}

	m.Mode = mode

	return nil
}

// Type implements Flag interface.
func (m *Mode) Type() string {
	options := maps.Keys(m.options)
	slices.Sort(options)

	return strings.Join(options, ", ")
}

const (
	modeAuto        = "auto"
	modeNoReboot    = "no-reboot"
	modeReboot      = "reboot"
	modeStaged      = "staged"
	modeInteractive = "interactive"
	modeTry         = "try"
)

// AddModeFlags adds deprecated flags to the command and registers mode flag with it's parser.
func AddModeFlags(mode *Mode, command *cobra.Command) {
	modes := map[string]machine.ApplyConfigurationRequest_Mode{
		modeAuto:     machine.ApplyConfigurationRequest_AUTO,
		modeNoReboot: machine.ApplyConfigurationRequest_NO_REBOOT,
		modeReboot:   machine.ApplyConfigurationRequest_REBOOT,
		modeStaged:   machine.ApplyConfigurationRequest_STAGED,
		modeTry:      machine.ApplyConfigurationRequest_TRY,
	}

	if command.Use == "apply-config" {
		modes[modeInteractive] = InteractiveMode
	}

	mode.Mode = machine.ApplyConfigurationRequest_AUTO
	mode.options = modes

	command.Flags().VarP(mode, "mode", "m", "apply config mode")
}

// PrintApplyResults prints out all warnings and auto apply results.
func PrintApplyResults(resp *machine.ApplyConfigurationResponse) {
	for _, m := range resp.GetMessages() {
		for _, w := range m.GetWarnings() {
			cli.Warning("%s", w)
		}

		if m.ModeDetails != "" {
			fmt.Fprintln(os.Stderr, m.ModeDetails)
		}
	}
}
