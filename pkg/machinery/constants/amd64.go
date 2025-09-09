// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build !amd64.v1

package constants

const (
	// MinimumGOAMD64Level is the minimum x86_64 microarchitecture level required by Talos.
	MinimumGOAMD64Level = 2
)
