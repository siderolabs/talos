// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package version defines version information.
package version

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"runtime"
	"text/template"

	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/gendata"
)

var (
	// Name is set at build time.
	Name = gendata.VersionName
	// Tag is set at build time.
	Tag = gendata.VersionTag
	// SHA is set at build time.
	SHA = gendata.VersionSHA
	// Built is set at build time.
	// TODO: its not.
	Built string
	// PkgsVersion is set at build time.
	PkgsVersion = gendata.VersionPkgs
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
	printLong(os.Stdout, NewVersion())
}

// PrintLongVersionFromExisting prints verbose version information.
func PrintLongVersionFromExisting(v *machineapi.VersionInfo) {
	printLong(os.Stdout, v)
}

// WriteLongVersionFromExisting writes verbose version to io.Writer.
func WriteLongVersionFromExisting(w io.Writer, v *machineapi.VersionInfo) {
	printLong(w, v)
}

func printLong(w io.Writer, v *machineapi.VersionInfo) {
	tmpl, err := template.New("version").Parse(versionTemplate)
	if err != nil {
		return
	}

	err = tmpl.Execute(w, v)
	if err != nil {
		return
	}
}

// PrintShortVersion prints the tag and SHA.
func PrintShortVersion() {
	fmt.Println(Short())
}

// Short returns the short version string consist of name, tag and SHA.
func Short() string {
	return fmt.Sprintf("%s %s", Name, Tag)
}

// Trim removes anything extra after semantic version core, `v0.3.2-1-abcd` -> `v0.3.2`.
func Trim(version string) string {
	return regexp.MustCompile(`(-\d+(-g[0-9a-f]+)?(-dirty)?)$`).ReplaceAllString(version, "")
}
