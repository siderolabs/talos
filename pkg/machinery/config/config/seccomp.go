// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

// SeccompProfile is a named seccomp profile, in the OCI runtime-spec LinuxSeccomp format.
//
// Profiles come from SeccompProfileConfig documents and from the deprecated
// `.machine.seccompProfiles` field.
type SeccompProfile interface {
	Name() string
	Value() map[string]any
}

// SeccompProfileConfig is a SeccompProfileConfig document.
type SeccompProfileConfig interface {
	NamedDocument
	SeccompProfile

	SeccompProfileConfigSignal()
}
