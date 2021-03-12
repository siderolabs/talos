// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration

// Package integration_test contains core runners for integration tests
package integration_test

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/talos-systems/talos/internal/integration/api"
	"github.com/talos-systems/talos/internal/integration/base"
	"github.com/talos-systems/talos/internal/integration/cli"
	"github.com/talos-systems/talos/internal/integration/k8s"
	provision_test "github.com/talos-systems/talos/internal/integration/provision"
	"github.com/talos-systems/talos/pkg/machinery/client/config"
	"github.com/talos-systems/talos/pkg/provision"
	"github.com/talos-systems/talos/pkg/provision/providers"
	"github.com/talos-systems/talos/pkg/version"
)

// Accumulated list of all the suites to run.
var allSuites []suite.TestingSuite

// Flag values.
var (
	failFast         bool
	crashdumpEnabled bool
	talosConfig      string
	endpoint         string
	k8sEndpoint      string
	expectedVersion  string
	talosctlPath     string
	kubectlPath      string
	provisionerName  string
	clusterName      string
	stateDir         string
)

// TestIntegration ...
//
//nolint:gocyclo
func TestIntegration(t *testing.T) {
	if talosConfig == "" {
		t.Error("--talos.config is not provided")
	}

	var (
		cluster     provision.Cluster
		provisioner provision.Provisioner
		err         error
	)

	if provisionerName != "" {
		// use provisioned cluster state as discovery source
		ctx := context.Background()

		provisioner, err = providers.Factory(ctx, provisionerName)
		if err != nil {
			t.Error("error initializing provisioner", err)
		}

		defer provisioner.Close() //nolint:errcheck

		cluster, err = provisioner.Reflect(ctx, clusterName, stateDir)
		if err != nil {
			t.Error("error reflecting cluster via provisioner", err)
		}
	}

	provision_test.DefaultSettings.CurrentVersion = expectedVersion
	provision_test.DefaultSettings.CrashdumpEnabled = crashdumpEnabled

	for _, s := range allSuites {
		if configuredSuite, ok := s.(base.ConfiguredSuite); ok {
			configuredSuite.SetConfig(base.TalosSuite{
				Endpoint:     endpoint,
				K8sEndpoint:  k8sEndpoint,
				Cluster:      cluster,
				TalosConfig:  talosConfig,
				Version:      expectedVersion,
				TalosctlPath: talosctlPath,
				KubectlPath:  kubectlPath,
			})
		}

		var suiteName string
		if namedSuite, ok := s.(base.NamedSuite); ok {
			suiteName = namedSuite.SuiteName()
		}

		t.Run(suiteName, func(tt *testing.T) {
			suite.Run(tt, s) //nolint:scopelint
		})

		if failFast && t.Failed() {
			t.Log("fastfail mode enabled, aborting on first failure")

			break
		}
	}

	if t.Failed() && crashdumpEnabled && cluster != nil && provisioner != nil {
		// if provisioner & cluster are available,
		// debugging failed test is easier with crashdump
		provisioner.CrashDump(context.Background(), cluster, os.Stderr)
	}
}

func init() {
	defaultTalosConfig, _ := config.GetDefaultPath() //nolint:errcheck

	defaultStateDir, err := config.GetTalosDirectory()
	if err == nil {
		defaultStateDir = filepath.Join(defaultStateDir, "clusters")
	}

	flag.BoolVar(&failFast, "talos.failfast", false, "fail the test run on the first failed test")
	flag.BoolVar(&crashdumpEnabled, "talos.crashdump", true, "print crashdump on test failure (only if provisioner is enabled)")

	flag.StringVar(&talosConfig, "talos.config", defaultTalosConfig, "The path to the Talos configuration file")
	flag.StringVar(&endpoint, "talos.endpoint", "", "endpoint to use (overrides config)")
	flag.StringVar(&k8sEndpoint, "talos.k8sendpoint", "", "Kubernetes endpoint to use (overrides kubeconfig)")
	flag.StringVar(&provisionerName, "talos.provisioner", "", "Talos cluster provisioner to use, if not set cluster state is disabled")
	flag.StringVar(&stateDir, "talos.state", defaultStateDir, "directory path to store cluster state")
	flag.StringVar(&clusterName, "talos.name", "talos-default", "the name of the cluster")
	flag.StringVar(&expectedVersion, "talos.version", version.Tag, "expected Talos version")
	flag.StringVar(&talosctlPath, "talos.talosctlpath", "talosctl", "The path to 'talosctl' binary")
	flag.StringVar(&kubectlPath, "talos.kubectlpath", "kubectl", "The path to 'kubectl' binary")

	flag.StringVar(&provision_test.DefaultSettings.CIDR, "talos.provision.cidr", provision_test.DefaultSettings.CIDR, "CIDR to use to provision clusters (provision tests only)")
	flag.Var(&provision_test.DefaultSettings.RegistryMirrors, "talos.provision.registry-mirror", "registry mirrors to use (provision tests only)")
	flag.IntVar(&provision_test.DefaultSettings.MTU, "talos.provision.mtu", provision_test.DefaultSettings.MTU, "MTU to use for cluster network (provision tests only)")
	flag.Int64Var(&provision_test.DefaultSettings.CPUs, "talos.provision.cpu", provision_test.DefaultSettings.CPUs, "CPU count for each VM (provision tests only)")
	flag.Int64Var(&provision_test.DefaultSettings.MemMB, "talos.provision.mem", provision_test.DefaultSettings.MemMB, "memory (in MiB) for each VM (provision tests only)")
	flag.Uint64Var(&provision_test.DefaultSettings.DiskGB, "talos.provision.disk", provision_test.DefaultSettings.DiskGB, "disk size (in GiB) for each VM (provision tests only)")
	flag.IntVar(&provision_test.DefaultSettings.MasterNodes, "talos.provision.masters", provision_test.DefaultSettings.MasterNodes, "master node count (provision tests only)")
	flag.IntVar(&provision_test.DefaultSettings.WorkerNodes, "talos.provision.workers", provision_test.DefaultSettings.WorkerNodes, "worker node count (provision tests only)")
	flag.StringVar(&provision_test.DefaultSettings.TargetInstallImageRegistry, "talos.provision.target-installer-registry",
		provision_test.DefaultSettings.TargetInstallImageRegistry, "image registry for target installer image (provision tests only)")
	flag.StringVar(&provision_test.DefaultSettings.CustomCNIURL, "talos.provision.custom-cni-url", provision_test.DefaultSettings.CustomCNIURL, "custom CNI URL for the cluster (provision tests only)")
	flag.StringVar(&provision_test.DefaultSettings.CNIBundleURL, "talos.provision.cni-bundle-url", provision_test.DefaultSettings.CNIBundleURL, "URL to download CNI bundle from")

	allSuites = append(allSuites, api.GetAllSuites()...)
	allSuites = append(allSuites, cli.GetAllSuites()...)
	allSuites = append(allSuites, k8s.GetAllSuites()...)
	allSuites = append(allSuites, provision_test.GetAllSuites()...)
}
