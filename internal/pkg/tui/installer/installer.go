// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package installer contains terminal UI based talos interactive installer parts.
package installer

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"gopkg.in/yaml.v3"

	"github.com/talos-systems/talos/internal/pkg/tui/components"
	machineapi "github.com/talos-systems/talos/pkg/machinery/api/machine"
	clientconfig "github.com/talos-systems/talos/pkg/machinery/client/config"
	"github.com/talos-systems/talos/pkg/version"
)

// NewPage creates a new installer page.
func NewPage(name string, items ...*Item) *Page {
	return &Page{
		name:  name,
		items: items,
	}
}

// Page represents a single installer page.
type Page struct {
	name  string
	items []*Item
}

// Item represents a single form item.
type Item struct {
	name        string
	description string
	dest        interface{}
	options     []interface{}
}

// TableHeaders represents table headers list for item options which are using table representation.
type TableHeaders []interface{}

// NewTableHeaders creates TableHeaders object.
func NewTableHeaders(headers ...interface{}) TableHeaders {
	return TableHeaders(headers)
}

// NewItem creates new form item.
func NewItem(name, description string, dest interface{}, options ...interface{}) *Item {
	return &Item{
		dest:        dest,
		name:        name,
		description: description,
		options:     options,
	}
}

func (item *Item) assign(value string) error {
	// rely on yaml parser to decode value into the right type
	return yaml.Unmarshal([]byte(value), item.dest)
}

// createFormItems dynamically creates tview.FormItem list based on the wrapped type.
// nolint:gocyclo
func (item *Item) createFormItems() ([]tview.Primitive, error) {
	res := []tview.Primitive{}

	v := reflect.ValueOf(item.dest)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if item.description != "" {
		parts := strings.Split(item.description, "\n")
		for _, part := range parts {
			res = append(res, components.NewFormLabel(part))
		}
	}

	var formItem tview.Primitive

	// nolint:exhaustive
	switch v.Kind() {
	case reflect.Bool:
		// use checkbox for boolean fields
		checkbox := tview.NewCheckbox()
		checkbox.SetChangedFunc(func(checked bool) {
			reflect.ValueOf(item.dest).Set(reflect.ValueOf(checked))
		})
		checkbox.SetChecked(v.Bool())
		checkbox.SetLabel(item.name)
		formItem = checkbox
	default:
		if len(item.options) > 0 {
			tableHeaders, ok := item.options[0].(TableHeaders)
			if ok {
				table := components.NewTable()
				table.SetHeader(tableHeaders...)

				data := item.options[1:]
				numColumns := len(tableHeaders)

				if len(data)%numColumns != 0 {
					return nil, fmt.Errorf("incorrect amount of data provided for the table")
				}

				selected := -1

				for i := 0; i < len(data); i += numColumns {
					table.AddRow(data[i : i+numColumns]...)

					if v.Interface() == data[i] {
						selected = i / numColumns
					}
				}

				if selected != -1 {
					table.SelectRow(selected)
				}

				formItem = table
				table.SetRowSelectedFunc(func(row int) {
					v.Set(reflect.ValueOf(table.GetValue(row, 0))) // always pick the first column
				})
			} else {
				dropdown := tview.NewDropDown()

				if len(item.options)%2 != 0 {
					return nil, fmt.Errorf("wrong amount of arguments for options: should be even amount of key, value pairs")
				}

				for i := 0; i < len(item.options); i += 2 {
					if optionName, ok := item.options[i].(string); ok {
						selected := -1

						func(index int) {
							dropdown.AddOption(optionName, func() {
								v.Set(reflect.ValueOf(item.options[index]))
							})

							if v.Interface() == item.options[index] {
								selected = i / 2
							}
						}(i + 1)

						if selected != -1 {
							dropdown.SetCurrentOption(selected)
						}
					} else {
						return nil, fmt.Errorf("expected string option name, got %s", item.options[i])
					}
				}

				dropdown.SetLabel(item.name)

				formItem = dropdown
			}
		} else {
			input := tview.NewInputField()
			formItem = input
			input.SetLabel(item.name)
			text, err := yaml.Marshal(item.dest)
			if err != nil {
				return nil, err
			}
			input.SetText(string(text))
			input.SetChangedFunc(func(text string) {
				if err := item.assign(text); err != nil {
					// TODO: highlight red
					return
				}
			})
		}
	}

	res = append(res, formItem)

	return res, nil
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
		conn,
	)

	select {
	case <-s.Stop(err == nil):
	case <-installer.ctx.Done():
		return context.Canceled
	}

	return err
}

// nolint:gocyclo
func (installer *Installer) configure() error {
	var (
		currentGroup *components.Group
		err          error
	)

	groups := []*components.Group{}
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
		currentGroup = groups[currentPage]
	}

	capture := installer.app.GetInputCapture()
	installer.app.SetInputCapture(func(e *tcell.EventKey) *tcell.EventKey {
		if currentGroup == nil {
			return e
		}

		//nolint:exhaustive
		switch e.Key() {
		case tcell.KeyTAB:
			currentGroup.NextFocus()
		case tcell.KeyBacktab:
			currentGroup.PrevFocus()
		case tcell.KeyCtrlN:
			setPage(currentPage + 1)
		case tcell.KeyCtrlB:
			setPage(currentPage - 1)
		}

		// page jump by ctrl/alt + N
		if e.Rune() >= '1' && e.Rune() < '9' {
			if e.Modifiers()&(tcell.ModAlt|tcell.ModCtrl) != 0 {
				setPage(int(e.Rune()) - 49)
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

	for i, p := range state.pages {
		eg := components.NewGroup(installer.app)
		groups = append(groups, eg)

		err = func(index int) error {
			form := components.NewForm()
			form.SetBackgroundColor(color)

			for _, item := range p.items {
				formItems, e := item.createFormItems()
				if e != nil {
					return e
				}

				for _, formItem := range formItems {
					if _, ok := formItem.(*components.FormLabel); !ok {
						eg.AddElement(formItem)
					}

					form.AddFormItem(formItem)
				}
			}

			flex := tview.NewFlex().SetDirection(tview.FlexRow)
			flex.AddItem(form, 0, 1, false)

			content := tview.NewFlex()

			if index > 0 {
				back := tview.NewButton("[::u]B[::-]ack")
				back.SetSelectedFunc(func() {
					setPage(index - 1)
				})

				content.AddItem(eg.AddElement(back), 10, 1, false)
			}

			addMenuItem(p.name, index)

			content.AddItem(tview.NewBox().SetBackgroundColor(color), 0, 1, false)
			flex.SetBackgroundColor(color)
			form.SetBackgroundColor(color)
			content.SetBackgroundColor(color)

			if index < len(state.pages)-1 {
				next := tview.NewButton("[::u]N[::-]ext")
				next.SetSelectedFunc(func() {
					setPage(index + 1)
				})

				content.AddItem(eg.AddElement(next), 10, 1, false)
			} else {
				install := tview.NewButton("Install")
				install.SetBackgroundColor(tcell.ColorGreen)
				install.SetTitleAlign(tview.AlignCenter)
				install.SetSelectedFunc(func() {
					close(done)
				})
				content.AddItem(eg.AddElement(install), 11, 1, false)
			}

			flex.AddItem(content, 1, 1, false)

			installer.addPage(p.name, flex, index == 0, menu)

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

		config = response.Data[0]
		talosconfig, err = clientconfig.FromBytes(response.Talosconfig)

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
			AddText(version.Tag, true, tview.AlignRight, tcell.ColorIvory)

		frame.SetBackgroundColor(frameBGColor)

		installer.pages.AddPage(name,
			frame, true, false)
		installer.app.Draw()
	}

	if switchToPage {
		installer.pages.SwitchToPage(name)
	}
}
