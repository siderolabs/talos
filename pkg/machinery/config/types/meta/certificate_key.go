// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package meta

// CertificateAndKey represents a PEM-encoded certificate and key pair.
type CertificateAndKey struct {
	Cert string `yaml:"cert,omitempty"`
	Key  string `yaml:"key,omitempty"`
}
