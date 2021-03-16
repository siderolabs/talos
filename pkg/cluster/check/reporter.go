// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package check

import (
	"fmt"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/fatih/color"
	"github.com/mattn/go-isatty"
	"golang.org/x/term"

	"github.com/talos-systems/talos/pkg/conditions"
)

type writerReporter struct {
	w                 *os.File
	lastLine          string
	lastLineTemporary bool

	colorized  bool
	spinnerIdx int
}

var spinner = []string{"◰", "◳", "◲", "◱"}

//nolint:gocyclo
func (wr *writerReporter) Update(condition conditions.Condition) {
	line := strings.TrimSpace(fmt.Sprintf("waiting for %s", condition))
	// replace tabs with spaces to get consistent output length
	line = strings.ReplaceAll(line, "\t", "    ")

	if !wr.colorized {
		if line != wr.lastLine {
			fmt.Fprintln(wr.w, line)
			wr.lastLine = line
		}

		return
	}

	w, _, _ := term.GetSize(int(wr.w.Fd())) //nolint:errcheck
	if w <= 0 {
		w = 80
	}

	var coloredLine string

	showSpinner := false
	prevLineTemporary := wr.lastLineTemporary

	switch {
	case strings.HasSuffix(line, "..."):
		line = fmt.Sprintf("%s %s", spinner[wr.spinnerIdx], line)
		coloredLine = color.YellowString("%s", line)
		wr.lastLineTemporary = true
		showSpinner = true
	case strings.HasSuffix(line, conditions.OK):
		coloredLine = color.GreenString("%s", line)
		wr.lastLineTemporary = false
	case strings.HasSuffix(line, conditions.ErrSkipAssertion.Error()):
		coloredLine = color.BlueString("%s", line)
		wr.lastLineTemporary = false
	default:
		line = fmt.Sprintf("%s %s", spinner[wr.spinnerIdx], line)
		coloredLine = color.RedString("%s", line)
		wr.lastLineTemporary = true
		showSpinner = true
	}

	if line == wr.lastLine {
		return
	}

	if showSpinner {
		wr.spinnerIdx = (wr.spinnerIdx + 1) % len(spinner)
	}

	if prevLineTemporary {
		for _, outputLine := range strings.Split(wr.lastLine, "\n") {
			for i := 0; i < (utf8.RuneCountInString(outputLine)+w-1)/w; i++ {
				fmt.Fprint(wr.w, "\033[A\033[K") // cursor up, clear line
			}
		}
	}

	fmt.Fprintln(wr.w, coloredLine)
	wr.lastLine = line
}

// StderrReporter returns console reporter with stderr output.
func StderrReporter() Reporter {
	return &writerReporter{
		w:         os.Stderr,
		colorized: isatty.IsTerminal(os.Stderr.Fd()),
	}
}
