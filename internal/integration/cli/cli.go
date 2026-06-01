// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration

// Package cli provides CLI (talosctl) integration tests for Talos
package cli

import "github.com/stretchr/testify/suite"

var allSuites []suite.TestingSuite

// GetAllSuites returns all the suites for CLI test.
//
// Depending on build tags, this might return different lists.
func GetAllSuites() []suite.TestingSuite {
	return allSuites
}
