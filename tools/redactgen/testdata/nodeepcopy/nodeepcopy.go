// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package nodeepcopy is the input for the redactgen error handling tests.
package nodeepcopy

// NoCopySpec has a sensitive field, but no DeepCopy method.
//
//redactgen:gen
type NoCopySpec struct {
	Token string `redact:"replace"`
}
