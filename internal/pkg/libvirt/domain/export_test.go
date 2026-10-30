// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package domain

// DefinitionDigests is the full and profile-independent digests of a rendered domain.
type DefinitionDigests struct {
	Full          string
	Configuration string
}

// DomainDefinition renders the definition of d and returns its text and digests.
func DomainDefinition(d Domain, renderedXML string) (string, DefinitionDigests, error) {
	def, err := domainDefinition(d, renderedXML)
	if err != nil {
		return "", DefinitionDigests{}, err
	}

	return def.text, DefinitionDigests{Full: def.full, Configuration: def.configuration}, nil
}

var (
	OpenVNCConn = (*Connector).openVNCConn
	VNCSocket   = vncSocket
)
