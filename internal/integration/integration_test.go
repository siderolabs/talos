// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration

// Package integration_test contains core runners for integration tests
package integration_test

import (
	"flag"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/talos-systems/talos/internal/integration/api"
	"github.com/talos-systems/talos/internal/integration/base"
	"github.com/talos-systems/talos/internal/integration/cli"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/version"
)

// Accumulated list of all the suites to run
var allSuites []suite.TestingSuite

// Flag values
var (
	talosConfig     string
	target          string
	expectedVersion string
	osctlPath       string
)

func TestIntegration(t *testing.T) {
	if talosConfig == "" {
		t.Error("--talos.config is not provided")
	}

	for _, s := range allSuites {
		if configuredSuite, ok := s.(base.ConfiguredSuite); ok {
			configuredSuite.SetConfig(base.TalosSuite{
				Target:      target,
				TalosConfig: talosConfig,
				Version:     expectedVersion,
				OsctlPath:   osctlPath,
			})
		}

		var suiteName string
		if namedSuite, ok := s.(base.NamedSuite); ok {
			suiteName = namedSuite.SuiteName()
		}

		t.Run(suiteName, func(tt *testing.T) {
			suite.Run(tt, s) //nolint: scopelint
		})
	}
}

func init() {
	var (
		defaultTalosConfig string
		ok                 bool
	)

	if defaultTalosConfig, ok = os.LookupEnv(constants.TalosConfigEnvVar); !ok {
		home, err := os.UserHomeDir()
		if err == nil {
			defaultTalosConfig = path.Join(home, ".talos", "config")
		}
	}

	flag.StringVar(&talosConfig, "talos.config", defaultTalosConfig, "The path to the Talos configuration file")
	flag.StringVar(&target, "talos.target", "", "target the specificed node")
	flag.StringVar(&expectedVersion, "talos.version", version.Tag, "expected Talos version")
	flag.StringVar(&osctlPath, "talos.osctlpath", "osctl", "The path to 'osctl' binary")

	allSuites = append(allSuites, api.GetAllSuites()...)
	allSuites = append(allSuites, cli.GetAllSuites()...)
}
