// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package reporter implements the console reporter.
package reporter

import (
	"fmt"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/fatih/color"
	"github.com/mattn/go-isatty"
	"golang.org/x/term"
)

// Status represents the status of an Update.
type Status int

const (
	// StatusError represents an error status.
	StatusError Status = iota
	// StatusRunning represents a running status.
	StatusRunning
	// StatusSucceeded represents a success status.
	StatusSucceeded
	// StatusSkip represents a skipped status.
	StatusSkip
)

var spinner = []string{"◰", "◳", "◲", "◱"}

// Update represents an update to be reported.
type Update struct {
	Message string
	Status  Status
}

// Reporter is a console reporter with stderr output.
type Reporter struct {
	w                 *os.File
	lastLine          string
	lastLineTemporary bool

	colorized  bool
	spinnerIdx int
}

// New returns a console reporter with stderr output.
func New() *Reporter {
	return &Reporter{
		w:         os.Stderr,
		colorized: isatty.IsTerminal(os.Stderr.Fd()),
	}
}

// IsColorized returns true if the reporter is colorized.
func (r *Reporter) IsColorized() bool {
	return r.colorized
}

// Report reports an update to the reporter.
//
//nolint:gocyclo
func (r *Reporter) Report(update Update) {
	line := strings.TrimSpace(update.Message)
	// replace tabs with spaces to get consistent output length
	line = strings.ReplaceAll(line, "\t", "    ")

	if !r.colorized {
		if line != r.lastLine {
			fmt.Fprintln(r.w, line)
			r.lastLine = line
		}

		return
	}

	w, _, _ := term.GetSize(int(r.w.Fd())) //nolint:errcheck
	if w <= 0 {
		w = 80
	}

	var coloredLine string

	showSpinner := false
	prevLineTemporary := r.lastLineTemporary

	switch update.Status {
	case StatusRunning:
		line = fmt.Sprintf("%s %s", spinner[r.spinnerIdx], line)
		coloredLine = color.YellowString("%s", line)
		r.lastLineTemporary = true
		showSpinner = true
	case StatusSucceeded:
		coloredLine = color.GreenString("%s", line)
		r.lastLineTemporary = false
	case StatusSkip:
		coloredLine = color.BlueString("%s", line)
		r.lastLineTemporary = false
	case StatusError:
		fallthrough
	default:
		line = fmt.Sprintf("%s %s", spinner[r.spinnerIdx], line)
		coloredLine = color.RedString("%s", line)
		r.lastLineTemporary = true
		showSpinner = true
	}

	if line == r.lastLine {
		return
	}

	if showSpinner {
		r.spinnerIdx = (r.spinnerIdx + 1) % len(spinner)
	}

	if prevLineTemporary {
		for outputLine := range strings.SplitSeq(r.lastLine, "\n") {
			for range (utf8.RuneCountInString(outputLine) + w - 1) / w {
				fmt.Fprint(r.w, "\033[A\033[K") // cursor up, clear line
			}
		}
	}

	fmt.Fprintln(r.w, coloredLine)
	r.lastLine = line
}
