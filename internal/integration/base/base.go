// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration

// Package base provides shared definition of base suites for tests
package base

// TalosSuite defines most common settings for integration test suites
type TalosSuite struct {
	// Target is address of master node, if not set config is used
	Target string
	// TalosConfig is a path to talosconfig
	TalosConfig string
	// Version is the (expected) version of Talos tests are running against
	Version string
	// OsctlPath is path to osctl binary
	OsctlPath string
}

// ConfiguredSuite expects config to be set before running
type ConfiguredSuite interface {
	SetConfig(config TalosSuite)
}

// SetConfig implements ConfiguredSuite
func (suite *TalosSuite) SetConfig(config TalosSuite) {
	*suite = config
}

// NamedSuite interface provides names for test suites
type NamedSuite interface {
	SuiteName() string
}
