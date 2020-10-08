// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package version

import (
	"bytes"
	"fmt"
	"runtime"
	"text/template"

	machineapi "github.com/talos-systems/talos/pkg/machinery/api/machine"
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
	// PkgsVersion is set at build time.
	PkgsVersion string
	// ExtrasVersion is set at build time.
	ExtrasVersion string
)

const versionTemplate = `	Tag:         {{ .Tag }}
	SHA:         {{ .Sha }}
	Built:       {{ .Built }}
	Go version:  {{ .GoVersion }}
	OS/Arch:     {{ .Os }}/{{ .Arch }}
`

// NewVersion prints verbose version information.
func NewVersion() *machineapi.VersionInfo {
	return &machineapi.VersionInfo{
		Tag:       Tag,
		Sha:       SHA,
		Built:     Built,
		GoVersion: runtime.Version(),
		Os:        runtime.GOOS,
		Arch:      runtime.GOARCH,
	}
}

// PrintLongVersion prints verbose version information.
func PrintLongVersion() {
	printLong(NewVersion())
}

// PrintLongVersionFromExisting prints verbose version information.
func PrintLongVersionFromExisting(v *machineapi.VersionInfo) {
	printLong(v)
}

func printLong(v *machineapi.VersionInfo) {
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
}

// PrintShortVersion prints the tag and SHA.
func PrintShortVersion() {
	fmt.Printf("%s %s-%s\n", Name, Tag, SHA)
}
