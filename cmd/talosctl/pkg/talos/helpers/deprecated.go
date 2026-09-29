// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package helpers

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/safeout"
)

type deprecatedFlag struct {
	flag    *pflag.Flag
	message string
}

// deprecatedFlags is filled in by [MarkFlagDeprecated] from the init functions registering the flags.
var deprecatedFlags []deprecatedFlag

func init() {
	// cobra runs the initializers once the command line is parsed and before the command itself
	// runs, so the warning comes out ahead of the command output.
	cobra.OnInitialize(func() { warnOnDeprecatedFlags(safeout.Stderr()) })
}

// MarkFlagDeprecated marks the named flag as deprecated: it is hidden from the help output, and
// using it prints a warning.
//
// It replaces pflag's own MarkDeprecated, which leaves the warning for cobra to print via
// cmd.Print, i.e. to the command's output stream. That stream is stdout for talosctl (the help
// output goes there), so the warning would land in the middle of the command output instead of
// on stderr.
func MarkFlagDeprecated(flags *pflag.FlagSet, name, message string) error {
	flag := flags.Lookup(name)
	if flag == nil {
		return fmt.Errorf("flag %q does not exist", name)
	}

	if message == "" {
		return fmt.Errorf("deprecation message for flag %q must be set", name)
	}

	flag.Hidden = true

	deprecatedFlags = append(deprecatedFlags, deprecatedFlag{flag: flag, message: message})

	return nil
}

// warnOnDeprecatedFlags prints a warning for every deprecated flag used on the command line.
//
// The wording matches pflag's own, as it is the message talosctl printed before the warnings were
// taken over here.
func warnOnDeprecatedFlags(w io.Writer) {
	for _, deprecated := range deprecatedFlags {
		if deprecated.flag.Changed {
			fmt.Fprintf(w, "Flag --%s has been deprecated, %s\n", deprecated.flag.Name, deprecated.message) //nolint:errcheck
		}
	}
}

// DeprecationMessage returns the message registered for the flag by [MarkFlagDeprecated], or an
// empty string if the flag is not deprecated.
//
// It stands in for pflag's own Flag.Deprecated field, which is left unset so that pflag does not
// print the warning itself.
func DeprecationMessage(flag *pflag.Flag) string {
	for _, deprecated := range deprecatedFlags {
		if deprecated.flag == flag {
			return deprecated.message
		}
	}

	return ""
}
