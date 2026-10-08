// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package domain

var (
	OpenVNCConn = (*Connector).openVNCConn
	VNCSocket   = vncSocket
)

// DefinitionDigests renders a definition and reports its full and profile-independent digests.
func DefinitionDigests(d Domain, renderedXML string) (full, configuration string, err error) {
	desired, err := domainDefinition(d, renderedXML)

	return desired.full, desired.configuration, err
}

// DefinitionMarker renders a definition and reports the ownership marker Talos would record for it.
func DefinitionMarker(d Domain, renderedXML string) (string, error) {
	desired, err := domainDefinition(d, renderedXML)
	if err != nil {
		return "", err
	}

	return definitionMarker(desired)
}
