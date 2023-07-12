// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package version

import (
	"bytes"
	"strings"
	"text/template"

	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// OSRelease returns the contents of /etc/os-release.
func OSRelease() ([]byte, error) {
	var (
		v    string
		tmpl *template.Template
	)

	switch Tag {
	case "none":
		v = SHA
	default:
		v = Tag
	}

	data := struct {
		Name    string
		ID      string
		Version string
	}{
		Name:    Name,
		ID:      strings.ToLower(Name),
		Version: v,
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
