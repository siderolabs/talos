/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package generate

import (
	"bytes"
	"text/template"
)

// Talosconfig returns the talos admin Talos config.
func Talosconfig(in *Input) (string, error) {
	return renderTemplate(in, talosconfigTempl)
}

const talosconfigTempl = `context: {{ .ClusterName }}
contexts:
  {{ .ClusterName }}:
    target: "{{ .GetAPIServerEndpoint "" }}"
    ca: {{ .Certs.OS }}
    crt: {{ .Certs.Admin.Crt }}
    key: {{ .Certs.Admin.Key }}
`

// renderTemplate will output a templated string.
func renderTemplate(in *Input, tmpl string) (string, error) {
	templ := template.Must(template.New("tmpl").Parse(tmpl))
	var buf bytes.Buffer
	if err := templ.Execute(&buf, in); err != nil {
		return "", err
	}

	return buf.String(), nil
}
