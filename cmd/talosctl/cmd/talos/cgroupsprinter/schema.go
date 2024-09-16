// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cgroupsprinter

import (
	"io"
	"strings"
	"text/template"
)

// Schema defines columns for cgroups printer.
type Schema struct {
	Columns []*Column `yaml:"columns"`
}

// Compile compiles the templates.
func (s *Schema) Compile() error {
	for _, c := range s.Columns {
		tmpl, err := template.New(c.Name).Parse(c.Template)
		if err != nil {
			return err
		}

		c.tmpl = tmpl
	}

	return nil
}

// HeaderLine returns the header line.
func (s *Schema) HeaderLine() string {
	var headerLine strings.Builder

	for i, c := range s.Columns {
		if i > 0 {
			headerLine.WriteString("\t")
		}

		headerLine.WriteString(c.Name)
	}

	return headerLine.String()
}

// Render returns the row line.
func (s *Schema) Render(data any) (string, error) {
	var rowLine strings.Builder

	for i, c := range s.Columns {
		if i > 0 {
			rowLine.WriteString("\t")
		}

		if err := c.Render(&rowLine, data); err != nil {
			return "", err
		}
	}

	return rowLine.String(), nil
}

// Column defines a column for cgroups printer.
type Column struct {
	Name string `yaml:"name"`

	Template string `yaml:"template"` // Template is a Go template string.

	tmpl *template.Template
}

// Render the template with the data.
func (c *Column) Render(out io.Writer, data any) error {
	if c.tmpl == nil {
		panic("template is not compiled")
	}

	return c.tmpl.Execute(out, data)
}
