// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package invalid is the input for the redactgen error handling tests.
package invalid

// UnsupportedSpec marks a field of a type which can't be replaced.
//
//redactgen:gen
type UnsupportedSpec struct {
	Timeout int `redact:"replace"`
}

// DeepCopy is a stub: redactgen only checks that the method is there.
func (spec UnsupportedSpec) DeepCopy() UnsupportedSpec {
	return spec
}
