// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package check

import (
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/mattn/go-isatty"
	"golang.org/x/crypto/ssh/terminal"

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

//nolint: gocyclo
func (wr *writerReporter) Update(condition conditions.Condition) {
	line := strings.TrimSpace(fmt.Sprintf("waiting for %s", condition))

	if !wr.colorized {
		if line != wr.lastLine {
			fmt.Fprintln(wr.w, line)
			wr.lastLine = line
		}

		return
	}

	var coloredLine string

	showSpinner := false
	prevLineTemporary := wr.lastLineTemporary

	switch {
	case strings.HasSuffix(line, "..."):
		coloredLine = color.YellowString("%s %s", spinner[wr.spinnerIdx], line)
		wr.lastLineTemporary = true
		showSpinner = true
	case strings.HasSuffix(line, "OK"):
		coloredLine = line
		wr.lastLineTemporary = false
	default:
		coloredLine = color.RedString("%s %s", spinner[wr.spinnerIdx], line)
		wr.lastLineTemporary = true
		showSpinner = true
	}

	if !showSpinner && line == wr.lastLine {
		return
	}

	if showSpinner {
		wr.spinnerIdx = (wr.spinnerIdx + 1) % len(spinner)
	}

	if prevLineTemporary {
		w, _, _ := terminal.GetSize(int(wr.w.Fd())) //nolint: errcheck
		if w <= 0 {
			w = 80
		}

		for _, outputLine := range strings.Split(wr.lastLine, "\n") {
			for i := 0; i < (len(outputLine)+w-1)/w; i++ {
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
