// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration

// Package integration_test contains core runners for integration tests
package integration_test

import (
	"context"
	"flag"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/talos-systems/talos/cmd/osctl/pkg/client/config"
	"github.com/talos-systems/talos/internal/integration/api"
	"github.com/talos-systems/talos/internal/integration/base"
	"github.com/talos-systems/talos/internal/integration/cli"
	"github.com/talos-systems/talos/internal/integration/k8s"
	"github.com/talos-systems/talos/internal/pkg/provision"
	"github.com/talos-systems/talos/internal/pkg/provision/providers"
	"github.com/talos-systems/talos/pkg/version"
)

// Accumulated list of all the suites to run
var allSuites []suite.TestingSuite

// Flag values
var (
	talosConfig     string
	endpoint        string
	k8sEndpoint     string
	expectedVersion string
	osctlPath       string
	provisionerName string
	clusterName     string
	stateDir        string
)

func TestIntegration(t *testing.T) {
	if talosConfig == "" {
		t.Error("--talos.config is not provided")
	}

	var cluster provision.Cluster

	if provisionerName != "" {
		// use provisioned cluster state as discovery source
		ctx := context.Background()

		provisioner, err := providers.Factory(ctx, provisionerName)
		if err != nil {
			t.Error("error iniitalizing provisioner", err)
		}

		defer provisioner.Close() //nolint: errcheck

		cluster, err = provisioner.Reflect(ctx, clusterName, stateDir)
		if err != nil {
			t.Error("error reflecting cluster via provisioner", err)
		}
	}

	for _, s := range allSuites {
		if configuredSuite, ok := s.(base.ConfiguredSuite); ok {
			configuredSuite.SetConfig(base.TalosSuite{
				Endpoint:    endpoint,
				K8sEndpoint: k8sEndpoint,
				Cluster:     cluster,
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
	defaultTalosConfig, _ := config.GetDefaultPath() //nolint: errcheck

	defaultStateDir, err := config.GetTalosDirectory()
	if err == nil {
		defaultStateDir = filepath.Join(defaultStateDir, "clusters")
	}

	flag.StringVar(&talosConfig, "talos.config", defaultTalosConfig, "The path to the Talos configuration file")
	flag.StringVar(&endpoint, "talos.endpoint", "", "endpoint to use (overrides config)")
	flag.StringVar(&k8sEndpoint, "talos.k8sendpoint", "", "Kubernetes endpoint to use (overrides kubeconfig)")
	flag.StringVar(&provisionerName, "talos.provisioner", "", "Talos cluster provisioner to use, if not set cluster state is disabled")
	flag.StringVar(&stateDir, "talos.state", defaultStateDir, "directory path to store cluster state")
	flag.StringVar(&clusterName, "talos.name", "talos-default", "the name of the cluster")
	flag.StringVar(&expectedVersion, "talos.version", version.Tag, "expected Talos version")
	flag.StringVar(&osctlPath, "talos.osctlpath", "osctl", "The path to 'osctl' binary")

	allSuites = append(allSuites, api.GetAllSuites()...)
	allSuites = append(allSuites, cli.GetAllSuites()...)
	allSuites = append(allSuites, k8s.GetAllSuites()...)
}
