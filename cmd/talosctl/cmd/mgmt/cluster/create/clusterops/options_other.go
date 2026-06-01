// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build !linux

package clusterops

var defaultNameservers = []string{"8.8.8.8", "1.1.1.1", "2001:4860:4860::8888", "2606:4700:4700::1111"}
