// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package installer contains terminal UI based talos interactive installer parts.
package installer

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/talos-systems/talos/internal/pkg/tui/components"
	machineapi "github.com/talos-systems/talos/pkg/machinery/api/machine"
	clientconfig "github.com/talos-systems/talos/pkg/machinery/client/config"
	"github.com/talos-systems/talos/pkg/version"
)

// NewPage creates a new installer page.
func NewPage(name string, items ...*components.Item) *Page {
	return &Page{
		name:  name,
		items: items,
	}
}

// Page represents a single installer page.
type Page struct {
	name  string
	items []*components.Item
}

// Installer interactive installer text based UI.
type Installer struct {
	pages      *tview.Pages
	app        *tview.Application
	wg         sync.WaitGroup
	err        error
	ctx        context.Context
	cancel     context.CancelFunc
	addedPages map[string]bool
	state      *State
}

// NewInstaller creates a new text based installer.
func NewInstaller() *Installer {
	ctx, cancel := context.WithCancel(context.Background())

	return &Installer{
		pages:  tview.NewPages(),
		ctx:    ctx,
		cancel: cancel,
	}
}

const (
	color         = tcell.Color238
	frameBGColor  = tcell.Color235
	inactiveColor = tcell.Color236
)

var spinner = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

const (
	phaseInit = iota
	phaseConfigure
	phaseApply
)

// Run starts interactive installer.
func (installer *Installer) Run(conn *Connection) error {
	installer.startApp()
	defer installer.stopApp()

	var (
		err         error
		description string
	)

	for phase := phaseInit; phase <= phaseApply; {
		switch phase {
		case phaseInit:
			description = "get the node information"
			err = installer.init(conn)
		case phaseConfigure:
			description = "generate the configuration"
			err = installer.configure()
		case phaseApply:
			description = "apply the configuration"
			err = installer.apply(conn)
		}

		if err != nil && err != context.Canceled {
			choice := installer.showModal(
				fmt.Sprintf("Failed to %s", description),
				err.Error(),
				"Quit", "Retry",
			)

			if choice == 1 {
				// apply should be retried from configure
				if phase == phaseApply {
					phase = phaseConfigure
				}

				continue
			}
		}

		if err != nil {
			return err
		}

		phase++
	}

	return nil
}

func (installer *Installer) startApp() {
	if installer.app != nil {
		return
	}

	installer.wg.Add(1)
	installer.app = tview.NewApplication()

	go func() {
		defer installer.wg.Done()
		defer installer.cancel()

		if err := installer.app.SetRoot(installer.pages, true).EnableMouse(true).Run(); err != nil {
			installer.err = err
		}
	}()
}

func (installer *Installer) stopApp() {
	if installer.app == nil {
		return
	}

	installer.app.Stop()
	installer.wg.Wait()
	installer.app = nil
}

func (installer *Installer) init(conn *Connection) (err error) {
	installer.startApp()

	s := components.NewSpinner(
		fmt.Sprintf("Connecting to the maintenance service at [green::]%s[white::]", conn.nodeEndpoint),
		spinner,
		installer.app,
	)

	s.SetBackgroundColor(color)
	installer.addPage("Gathering the node information", s, true, nil)

	installer.state, err = NewState(
		installer.ctx,
		installer,
		conn,
	)

	select {
	case <-s.Stop(err == nil):
	case <-installer.ctx.Done():
		return context.Canceled
	}

	return err
}

//nolint:gocyclo
func (installer *Installer) configure() error {
	var (
		err   error
		forms []*components.Form
	)

	currentPage := 0
	menuButtons := []*components.MenuButton{}

	done := make(chan struct{})
	state := installer.state

	setPage := func(index int) {
		if index < 0 || index >= len(state.pages) {
			return
		}

		menuButtons[currentPage].SetActive(false)
		currentPage = index
		menuButtons[currentPage].SetActive(true)

		installer.pages.SwitchToPage(state.pages[currentPage].name)
		installer.app.SetFocus(forms[currentPage])
	}

	capture := installer.app.GetInputCapture()
	installer.app.SetInputCapture(func(e *tcell.EventKey) *tcell.EventKey {
		//nolint:exhaustive
		switch e.Key() {
		case tcell.KeyCtrlN:
			setPage(currentPage + 1)
		case tcell.KeyCtrlB:
			setPage(currentPage - 1)
		}

		// page jump by ctrl/alt + N
		if e.Rune() >= '1' && e.Rune() < '9' {
			if e.Modifiers()&(tcell.ModAlt|tcell.ModCtrl) != 0 {
				setPage(int(e.Rune()) - 49)

				return nil
			}
		}

		if capture != nil {
			return capture(e)
		}

		return e
	})

	defer installer.app.SetInputCapture(capture)

	menu := tview.NewFlex()
	menu.SetBackgroundColor(frameBGColor)

	addMenuItem := func(name string, index int) {
		button := components.NewMenuButton(name)
		button.SetActiveColors(color, tcell.ColorIvory)
		button.SetInactiveColors(inactiveColor, tcell.ColorIvory)

		func(page int) {
			button.SetSelectedFunc(func() {
				setPage(page)
			})
		}(index)

		menu.AddItem(button, len(name)+4, 1, false)
		menuButtons = append(menuButtons, button)

		if currentPage == index {
			button.SetActive(true)
		}
	}

	forms = make([]*components.Form, len(state.pages))

	for i, p := range state.pages {
		err = func(index int) error {
			form := components.NewForm(installer.app)
			form.SetBackgroundColor(color)
			forms[i] = form

			if e := form.AddFormItems(p.items); e != nil {
				return e
			}

			if index > 0 {
				back := form.AddMenuButton("[::u]B[::-]ack", false)
				back.SetSelectedFunc(func() {
					setPage(index - 1)
				})
			}

			addMenuItem(p.name, index)

			form.SetBackgroundColor(color)

			if index < len(state.pages)-1 {
				next := form.AddMenuButton("[::u]N[::-]ext", index == 0)
				next.SetSelectedFunc(func() {
					setPage(index + 1)
				})
			} else {
				install := form.AddMenuButton("Install", false)
				install.SetBackgroundColor(tcell.ColorGreen)
				install.SetSelectedFunc(func() {
					close(done)
				})
			}

			installer.addPage(p.name, form, index == 0, menu)

			return nil
		}(i)

		if err != nil {
			return err
		}
	}

	setPage(0)

	select {
	case <-installer.ctx.Done():
		return context.Canceled
	case <-done: // nothing here, just waiting
	}

	if err != nil {
		return err
	}

	return nil
}

func (installer *Installer) apply(conn *Connection) error {
	var (
		config      []byte
		talosconfig *clientconfig.Config
		err         error
		response    *machineapi.GenerateConfigurationResponse
	)

	list := tview.NewFlex().SetDirection(tview.FlexRow)
	list.SetBackgroundColor(color)
	installer.addPage("Installing Talos", list, true, nil)

	{
		s := components.NewSpinner(
			"Generating configuration...",
			spinner,
			installer.app,
		)
		s.SetBackgroundColor(color)

		list.AddItem(s, 1, 1, false)

		response, err = installer.state.GenConfig()

		s.Stop(err == nil)

		if err != nil {
			return err
		}

		config = response.Messages[0].Data[0]
		talosconfig, err = clientconfig.FromBytes(response.Messages[0].Talosconfig)

		if err != nil {
			return err
		}
	}

	{
		s := components.NewSpinner(
			"Applying configuration...",
			spinner,
			installer.app,
		)
		s.SetBackgroundColor(color)

		// TODO: progress bar, logs?
		list.AddItem(s, 1, 1, false)
		_, err = conn.ApplyConfiguration(&machineapi.ApplyConfigurationRequest{
			Data: config,
		})

		s.Stop(err == nil)

		if err != nil {
			return err
		}
	}

	if err != nil {
		return err
	}

	return installer.writeTalosconfig(list, talosconfig)
}

func (installer *Installer) writeTalosconfig(list *tview.Flex, talosconfig *clientconfig.Config) error {
	path, err := clientconfig.GetDefaultPath()
	if err != nil {
		return err
	}

	f, err := os.Open(path)

	var config *clientconfig.Config

	if err != nil && !os.IsNotExist(err) {
		return err
	}

	if err == nil {
		config, err = clientconfig.ReadFrom(f)
		if err != nil {
			return err
		}
	}

	text := tview.NewTextView()
	addLines := func(lines ...string) {
		t := text.GetText(false)
		t += strings.Join(lines, "\n")
		text.SetText(t)
		installer.app.Draw()
	}

	addLines(
		"",
		fmt.Sprintf("Merging talosconfig into %s...", path),
	)
	text.SetBackgroundColor(color)
	list.AddItem(text, 0, 1, false)

	renames := []clientconfig.Rename{}
	if config != nil {
		renames = config.Merge(talosconfig)
	} else {
		config = talosconfig
	}

	for _, rename := range renames {
		addLines(fmt.Sprintf("Renamed %s.", rename.String()))
	}

	context := talosconfig.Context
	if len(renames) != 0 {
		context = renames[0].To
	}

	config.Context = context
	addLines(fmt.Sprintf("Set current context to %q.", context))

	err = config.Save(path)
	if err != nil {
		return err
	}

	addLines(
		"",
		"Press any key to exit.",
	)

	installer.awaitKey()

	return nil
}

func (installer *Installer) awaitKey(keys ...tcell.Key) {
	done := make(chan struct{})

	installer.app.SetInputCapture(func(e *tcell.EventKey) *tcell.EventKey {
		for _, key := range keys {
			if e.Key() == key {
				close(done)
			}
		}

		if len(keys) == 0 {
			close(done)
		}

		return e
	})

	select {
	case <-done:
	case <-installer.ctx.Done():
	}
}

// showModal block execution and show modal window.
func (installer *Installer) showModal(title, text string, buttons ...string) int {
	done := make(chan struct{})

	index := -1

	modal := tview.NewModal().
		SetText(text).
		AddButtons(buttons).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			index = buttonIndex
			close(done)
		})

	installer.addPage(title, modal, true, nil)
	installer.app.SetFocus(modal)
	installer.app.Draw()

	select {
	case <-done:
	case <-installer.ctx.Done():
	}

	return index
}

func (installer *Installer) addPage(name string, primitive tview.Primitive, switchToPage bool, menu tview.Primitive) {
	if !installer.addedPages[name] {
		content := tview.NewFlex().SetDirection(tview.FlexRow)
		page := tview.NewFrame(primitive).SetBorders(1, 1, 1, 1, 2, 2)
		page.SetBackgroundColor(color)

		if menu != nil {
			content.AddItem(menu, 3, 1, false)
		}

		content.AddItem(page, 0, 1, false)

		frame := tview.NewFrame(content).SetBorders(1, 1, 1, 1, 2, 2).
			AddText(name, true, tview.AlignLeft, tcell.ColorWhite).
			AddText("Talos Interactive Installer", true, tview.AlignCenter, tcell.ColorWhite).
			AddText(version.Tag, true, tview.AlignRight, tcell.ColorIvory).
			AddText("<CTRL>+B/<CTRL>+N to switch tabs", false, tview.AlignLeft, tcell.ColorIvory).
			AddText("<TAB> for navigation", false, tview.AlignLeft, tcell.ColorIvory).
			AddText("[::b]Key Bindings[::-]", false, tview.AlignLeft, tcell.ColorIvory)

		frame.SetBackgroundColor(frameBGColor)

		if switchToPage {
			installer.pages.AddAndSwitchToPage(name, frame, true)
		} else {
			installer.pages.AddPage(name,
				frame, true, false)
		}
	} else if switchToPage {
		installer.pages.SwitchToPage(name)
	}

	installer.app.ForceDraw()
}
