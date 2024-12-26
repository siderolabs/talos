// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package version

//go:generate go run ./gen.go

import (
	"bytes"
	"strings"
	"text/template"

	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// OSRelease returns the contents of /etc/os-release.
func OSRelease() ([]byte, error) {
	var v string

	switch Tag {
	case "none":
		v = SHA
	default:
		v = Tag
	}

	return OSReleaseFor(Name, v)
}

// OSReleaseFor returns the contents of /etc/os-release for a given name and version.
func OSReleaseFor(name, version string) ([]byte, error) {
	data := struct {
		Name    string
		ID      string
		Version string
	}{
		Name:    name,
		ID:      strings.ToLower(name),
		Version: version,
	}

	tmpl, err := template.New("").Parse(constants.OSReleaseTemplate)
	if err != nil {
		return nil, err
	}

	var writer bytes.Buffer

	err = tmpl.Execute(&writer, data)
	if err != nil {
		return nil, err
	}

	return writer.Bytes(), nil
}
