// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package configdiff provides a way to compare two config trees.
package configdiff

import (
	"fmt"
	"io"
	"strings"

	"github.com/fatih/color"
	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"

	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
)

// Diff outputs (optionally) colorized diff between two machine configurations.
//
// One of the resources might be nil.
func Diff(w io.Writer, oldCfg, newCfg config.Encoder) error {
	var (
		oldYaml, newYaml []byte
		err              error
	)

	if oldCfg != nil {
		oldYaml, err = oldCfg.EncodeBytes(encoder.WithComments(encoder.CommentsDisabled))
		if err != nil {
			return err
		}
	}

	if newCfg != nil {
		newYaml, err = newCfg.EncodeBytes(encoder.WithComments(encoder.CommentsDisabled))
		if err != nil {
			return err
		}
	}

	edits := myers.ComputeEdits(span.URIFromPath("a"), string(oldYaml), string(newYaml))
	diff := gotextdiff.ToUnified("a", "b", string(oldYaml), edits)

	outputDiff(w, diff, true)

	return nil
}

// DiffToString returns a string representation of the diff between two machine configurations.
func DiffToString(oldCfg, newCfg config.Encoder) (string, error) {
	var sb strings.Builder

	err := Diff(&sb, oldCfg, newCfg)

	return sb.String(), err
}

//nolint:gocyclo
func outputDiff(w io.Writer, u gotextdiff.Unified, noColor bool) {
	if len(u.Hunks) == 0 {
		return
	}

	bold := color.New(color.Bold)
	cyan := color.New(color.FgCyan)
	red := color.New(color.FgRed)
	green := color.New(color.FgGreen)

	if noColor {
		bold.DisableColor()
		cyan.DisableColor()
		red.DisableColor()
		green.DisableColor()
	}

	bold.Fprintf(w, "--- %s\n", u.From) //nolint:errcheck
	bold.Fprintf(w, "+++ %s\n", u.To)   //nolint:errcheck

	for _, hunk := range u.Hunks {
		fromCount, toCount := 0, 0

		for _, l := range hunk.Lines {
			switch l.Kind { //nolint:exhaustive
			case gotextdiff.Delete:
				fromCount++
			case gotextdiff.Insert:
				toCount++
			default:
				fromCount++
				toCount++
			}
		}

		cyan.Fprintf(w, "@@") //nolint:errcheck

		if fromCount > 1 {
			cyan.Fprintf(w, " -%d,%d", hunk.FromLine, fromCount) //nolint:errcheck
		} else {
			cyan.Fprintf(w, " -%d", hunk.FromLine) //nolint:errcheck
		}

		if toCount > 1 {
			cyan.Fprintf(w, " +%d,%d", hunk.ToLine, toCount) //nolint:errcheck
		} else {
			cyan.Fprintf(w, " +%d", hunk.ToLine) //nolint:errcheck
		}

		cyan.Fprintf(w, " @@\n") //nolint:errcheck

		for _, l := range hunk.Lines {
			switch l.Kind { //nolint:exhaustive
			case gotextdiff.Delete:
				red.Fprintf(w, "-%s", l.Content) //nolint:errcheck
			case gotextdiff.Insert:
				green.Fprintf(w, "+%s", l.Content) //nolint:errcheck
			default:
				fmt.Fprintf(w, " %s", l.Content)
			}

			if !strings.HasSuffix(l.Content, "\n") {
				red.Fprintf(w, "\n\\ No newline at end of file\n") //nolint:errcheck
			}
		}
	}
}
