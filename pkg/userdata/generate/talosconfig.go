/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package generate

// Talosconfig returns the talos admin Talos config.
func Talosconfig(in *Input) (string, error) {
	return renderTemplate(in, talosconfigTempl)
}

const talosconfigTempl = `context: {{ .ClusterName }}
contexts:
  {{ .ClusterName }}:
    target: {{ index .MasterIPs 0 }}
    ca: {{ .Certs.OsCert }}
    crt: {{ .Certs.AdminCert }}
    key: {{ .Certs.AdminKey }}
`
