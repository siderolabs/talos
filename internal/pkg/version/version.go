/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package version

import (
	"bytes"
	"fmt"
	"runtime"
	"text/template"
)

var (
	// Name is set at build time.
	Name string
	// Tag is set at build time.
	Tag string
	// SHA is set at build time.
	SHA string
	// Built is set at build time.
	Built string
)

const versionTemplate = `{{ .Name }}:
	Tag:         {{ .Tag }}
	SHA:         {{ .SHA }}
	Built:       {{ .Built }}
	Go version:  {{ .GoVersion }}
	OS/Arch:     {{ .Os }}/{{ .Arch }}
`

// Version contains verbose version information.
type Version struct {
	Name      string
	Tag       string
	SHA       string
	ID        string
	Built     string
	GoVersion string
	Os        string
	Arch      string
}

// NewVersion prints verbose version information.
func NewVersion() (version string, err error) {
	v := Version{
		Name:      Name,
		Tag:       Tag,
		SHA:       SHA,
		GoVersion: runtime.Version(),
		Os:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		Built:     Built,
	}

	var wr bytes.Buffer
	tmpl, err := template.New("version").Parse(versionTemplate)
	if err != nil {
		return
	}

	err = tmpl.Execute(&wr, v)
	if err != nil {
		return
	}

	version = wr.String()

	return version, err
}

// PrintLongVersion prints verbose version information.
func PrintLongVersion() (err error) {
	v := Version{
		Name:      Name,
		Tag:       Tag,
		SHA:       SHA,
		GoVersion: runtime.Version(),
		Os:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		Built:     Built,
	}

	var wr bytes.Buffer
	tmpl, err := template.New("version").Parse(versionTemplate)
	if err != nil {
		return
	}

	err = tmpl.Execute(&wr, v)
	if err != nil {
		return
	}

	fmt.Println(wr.String())

	return nil
}

// PrintShortVersion prints the tag and SHA.
func PrintShortVersion() {
	fmt.Println(fmt.Sprintf("%s %s-%s", Name, Tag, SHA))
}
