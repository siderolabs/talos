// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package components

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	yaml "gopkg.in/yaml.v3"
)

var hline = strings.Repeat(string(tcell.RuneHLine), 2000)

// Separator hline with a description below.
type Separator struct {
	*tview.TextView
}

// GetHeight implements Multiline interface.
func (s *Separator) GetHeight() int {
	return 2
}

// NewSeparator creates new form special form item which can be used as a sections Separator.
func NewSeparator(description string) *Item {
	return NewItem("", "", func(item *Item) tview.Primitive {
		s := &Separator{
			tview.NewTextView(),
		}
		s.SetText(hline + "\n" + description)
		s.SetWrap(false)

		return s
	})
}

// Item represents a single form item.
type Item struct {
	Name        string
	description string
	dest        any
	options     []any
}

// TableHeaders represents table headers list for item options which are using table representation.
type TableHeaders []any

// NewTableHeaders creates TableHeaders object.
func NewTableHeaders(headers ...any) TableHeaders {
	return TableHeaders(headers)
}

// NewItem creates new form item.
func NewItem(name, description string, dest any, options ...any) *Item {
	return &Item{
		Name:        name,
		dest:        dest,
		description: description,
		options:     options,
	}
}

func (item *Item) assign(value string) error {
	// rely on yaml parser to decode value into the right type
	return yaml.Unmarshal([]byte(value), item.dest)
}

// createFormItems dynamically creates tview.FormItem list based on the wrapped type.
//
//nolint:gocyclo,cyclop
func (item *Item) createFormItems() ([]tview.Primitive, error) {
	var res []tview.Primitive

	v := reflect.ValueOf(item.dest)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	var formItem tview.Primitive

	label := fmt.Sprintf("[::b]%s[::-]:", item.Name)
	addDescription := true

	//nolint:exhaustive
	switch v.Kind() {
	case reflect.Func:
		if f, ok := item.dest.(func(*Item) tview.Primitive); ok {
			formItem = f(item)
		}
	case reflect.Bool:
		// use checkbox for boolean fields
		checkbox := tview.NewCheckbox()
		checkbox.SetChangedFunc(func(checked bool) {
			v.Set(reflect.ValueOf(checked))
		})
		checkbox.SetChecked(v.Bool())
		checkbox.SetLabel(label)
		formItem = checkbox
	default:
		if len(item.options) > 0 {
			tableHeaders, ok := item.options[0].(TableHeaders)
			if ok {
				table := NewTable()
				table.SetHeader(tableHeaders...)

				addDescription = false

				data := item.options[1:]
				numColumns := len(tableHeaders)

				if len(data)%numColumns != 0 {
					return nil, errors.New("incorrect amount of data provided for the table")
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
					v.Set(reflect.ValueOf(table.GetValue(row-1, 0))) // always pick the second column
				})
			} else {
				dropdown := tview.NewDropDown()

				if len(item.options)%2 != 0 {
					return nil, errors.New("wrong amount of arguments for options: should be even amount of key, value pairs")
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

				dropdown.SetLabel(label)

				formItem = dropdown
			}
		} else {
			input := tview.NewInputField()
			formItem = input

			input.SetLabel(label)

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

	if item.description != "" && addDescription {
		parts := strings.Split(item.description, "\n")
		for _, part := range parts {
			desc := NewFormLabel(part)
			res = append(res, desc)
		}
	}

	res = append(res, NewFormLabel(""))

	return res, nil
}

// NewForm creates a new form.
func NewForm(app *tview.Application) *Form {
	f := &Form{
		Flex:      tview.NewFlex().SetDirection(tview.FlexRow),
		formItems: []tview.FormItem{},
		form:      tview.NewFlex().SetDirection(tview.FlexRow),
		buttons:   tview.NewFlex(),
		group:     NewGroup(app),
	}

	f.Flex.AddItem(f.form, 0, 1, false)
	f.Flex.AddItem(f.buttons, 0, 0, false)

	f.SetInputCapture(func(e *tcell.EventKey) *tcell.EventKey {
		//nolint:exhaustive
		switch e.Key() {
		case tcell.KeyTAB:
			f.group.NextFocus()
		case tcell.KeyBacktab:
			f.group.PrevFocus()
		}

		return e
	})

	return f
}

// Form is a more flexible form component for tview lib.
type Form struct {
	*tview.Flex
	form          *tview.Flex
	buttons       *tview.Flex
	formItems     []tview.FormItem
	maxLabelLen   int
	hasMenuButton bool
	group         *Group
}

// AddFormItem adds a new item to the form.
func (f *Form) AddFormItem(item tview.Primitive) {
	if formItem, ok := item.(tview.FormItem); ok {
		f.formItems = append(f.formItems, formItem)
		labelLen := tview.TaggedStringWidth(formItem.GetLabel()) + 1

		if labelLen > f.maxLabelLen {
			for _, item := range f.formItems[:len(f.formItems)-1] {
				item.SetFormAttributes(
					labelLen,
					tview.Styles.PrimaryTextColor,
					f.GetBackgroundColor(),
					tview.Styles.PrimaryTextColor,
					tview.Styles.ContrastBackgroundColor,
				)
			}

			f.maxLabelLen = labelLen
		}

		formItem.SetFormAttributes(
			f.maxLabelLen,
			tview.Styles.PrimaryTextColor,
			f.GetBackgroundColor(),
			tview.Styles.PrimaryTextColor,
			tview.Styles.ContrastBackgroundColor,
		)
	} else if box, ok := item.(Box); ok {
		box.SetBackgroundColor(f.GetBackgroundColor())
	}

	height := 1
	multiline, ok := item.(Multiline)

	if ok {
		height = multiline.GetHeight()
	}

	f.form.AddItem(item, height, 1, false)

	switch item.(type) {
	case *FormLabel:
	case *Separator:
	default:
		f.group.AddElement(item)
	}
}

// Focus overrides default focus behavior.
func (f *Form) Focus(delegate func(tview.Primitive)) {
	f.group.FocusFirst()
}

// AddMenuButton adds a button to the menu at the bottom of the form.
func (f *Form) AddMenuButton(label string, alignRight bool) *tview.Button {
	b := tview.NewButton(label)

	if f.hasMenuButton || alignRight {
		f.buttons.AddItem(
			tview.NewBox().SetBackgroundColor(f.GetBackgroundColor()),
			0,
			1,
			false,
		)
	}

	f.ResizeItem(f.buttons, 3, 0)

	f.buttons.AddItem(b, tview.TaggedStringWidth(label)+4, 0, false)
	f.hasMenuButton = true
	f.group.AddElement(b)

	return b
}

// AddFormItems constructs form from data represented as a list of Item objects.
func (f *Form) AddFormItems(items []*Item) error {
	for _, item := range items {
		formItems, e := item.createFormItems()
		if e != nil {
			return e
		}

		for _, formItem := range formItems {
			f.AddFormItem(formItem)
		}
	}

	return nil
}

// Multiline interface represents elements that can occupy more than one line.
type Multiline interface {
	GetHeight() int
}

// Box interface that has just SetBackgroundColor.
type Box interface {
	SetBackgroundColor(tcell.Color) *tview.Box
}
