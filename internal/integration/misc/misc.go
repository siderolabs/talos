// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration

// Package misc provides miscellaneous integration tests which are only run
// against special cluster configurations (e.g. fully ephemeral nodes).
package misc

import "github.com/stretchr/testify/suite"

var allSuites []suite.TestingSuite

// GetAllSuites returns all the suites for the misc package.
//
// Depending on build tags, this might return different lists.
func GetAllSuites() []suite.TestingSuite {
	return allSuites
}
