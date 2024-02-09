// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package components

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// NewSpinner creates a new spinner.
func NewSpinner(label string, spinner []string, app *tview.Application) *Spinner {
	s := &Spinner{
		Box:      tview.NewBox(),
		spinner:  spinner,
		label:    label,
		ticker:   time.NewTicker(time.Millisecond * 50),
		shutdown: make(chan struct{}),
		stopped:  make(chan struct{}),
	}

	go func() {
		defer func() {
			app.Draw()
			close(s.stopped)
		}()

		for {
			select {
			case <-s.shutdown:
				return
			case <-s.ticker.C:
				app.Draw()
			}
		}
	}()

	return s
}

const (
	spinnerStateInProgress = iota
	spinnerStateComplete
	spinnerStateFailed
)

var spinnerStatusSymbols = []string{
	"",
	"[green::]✓[-::]",
	"[red::]✖[-::]",
}

// Spinner a unicode spinner primitive.
type Spinner struct {
	*tview.Box
	spinner  []string
	label    string
	index    int
	shutdown chan struct{}
	stopped  chan struct{}
	ticker   *time.Ticker
	state    int
}

// Draw draws this primitive onto the screen.
func (s *Spinner) Draw(screen tcell.Screen) {
	s.Box.Draw(screen)
	x, y, width, _ := s.GetInnerRect()

	var spinner string

	if s.state == spinnerStateInProgress {
		spinner = s.spinner[s.index]
	} else {
		spinner = spinnerStatusSymbols[s.state]
	}

	line := fmt.Sprintf(s.label+" %s", spinner)
	tview.Print(screen, line, x, y, width, tview.AlignLeft, tcell.ColorWhite)

	s.index++

	if s.index == len(s.spinner) {
		s.index = 0
	}
}

// Stop should be always called to stop the spinner background routine.
func (s *Spinner) Stop(success bool) <-chan struct{} {
	s.state = spinnerStateInProgress
	if success {
		s.state = spinnerStateComplete
	} else {
		s.state = spinnerStateFailed
	}

	s.ticker.Stop()
	s.shutdown <- struct{}{}

	return s.stopped
}
