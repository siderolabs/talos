// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration

// Package integration_test contains core runners for integration tests
package integration_test

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/integration/api"
	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/internal/integration/cli"
	"github.com/siderolabs/talos/internal/integration/k8s"
	provision_test "github.com/siderolabs/talos/internal/integration/provision"
	"github.com/siderolabs/talos/pkg/images"
	clientconfig "github.com/siderolabs/talos/pkg/machinery/client/config"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/version"
	"github.com/siderolabs/talos/pkg/provision"
	"github.com/siderolabs/talos/pkg/provision/providers"
)

// Accumulated list of all the suites to run.
var allSuites []suite.TestingSuite

// Flag values.
var (
	failFast         bool
	crashdumpEnabled bool
	trustedBoot      bool
	extensionsQEMU   bool
	extensionsNvidia bool

	talosConfig       string
	endpoint          string
	k8sEndpoint       string
	expectedVersion   string
	expectedGoVersion string
	talosctlPath      string
	kubectlPath       string
	helmPath          string
	kubeStrPath       string
	provisionerName   string
	clusterName       string
	stateDir          string
	talosImage        string
	csiTestName       string
	csiTestTimeout    string
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

		if k8sEndpoint == "" && provisionerName == "docker" {
			k8sEndpoint = cluster.Info().KubernetesEndpoint
		}
	}

	provision_test.DefaultSettings.CurrentVersion = expectedVersion
	provision_test.DefaultSettings.CrashdumpEnabled = crashdumpEnabled

	for _, s := range allSuites {
		if configuredSuite, ok := s.(base.ConfiguredSuite); ok {
			configuredSuite.SetConfig(base.TalosSuite{
				Endpoint:         endpoint,
				K8sEndpoint:      k8sEndpoint,
				Cluster:          cluster,
				TalosConfig:      talosConfig,
				Version:          expectedVersion,
				GoVersion:        expectedGoVersion,
				TalosctlPath:     talosctlPath,
				KubectlPath:      kubectlPath,
				HelmPath:         helmPath,
				KubeStrPath:      kubeStrPath,
				ExtensionsQEMU:   extensionsQEMU,
				ExtensionsNvidia: extensionsNvidia,
				TrustedBoot:      trustedBoot,
				TalosImage:       talosImage,
				CSITestName:      csiTestName,
				CSITestTimeout:   csiTestTimeout,
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
	defaultTalosConfigs, _ := clientconfig.GetDefaultPaths() //nolint:errcheck

	defaultStateDir, err := clientconfig.GetTalosDirectory()
	if err == nil {
		defaultStateDir = filepath.Join(defaultStateDir, "clusters")
	}

	flag.BoolVar(&failFast, "talos.failfast", false, "fail the test run on the first failed test")
	flag.BoolVar(&crashdumpEnabled, "talos.crashdump", false, "print crashdump on test failure (only if provisioner is enabled)")
	flag.BoolVar(&trustedBoot, "talos.trustedboot", false, "enable tests for trusted boot mode")
	flag.BoolVar(&extensionsQEMU, "talos.extensions.qemu", false, "enable tests for qemu extensions")
	flag.BoolVar(&extensionsNvidia, "talos.extensions.nvidia", false, "enable tests for nvidia extensions")

	flag.StringVar(
		&talosConfig,
		"talos.config",
		defaultTalosConfigs[0].Path,
		fmt.Sprintf("The path to the Talos configuration file. Defaults to '%s' env variable if set, otherwise '%s' and '%s' in order.",
			constants.TalosConfigEnvVar,
			filepath.Join("$HOME", constants.TalosDir, constants.TalosconfigFilename),
			filepath.Join(constants.ServiceAccountMountPath, constants.TalosconfigFilename),
		),
	)
	flag.StringVar(&endpoint, "talos.endpoint", "", "endpoint to use (overrides config)")
	flag.StringVar(&k8sEndpoint, "talos.k8sendpoint", "", "Kubernetes endpoint to use (overrides kubeconfig)")
	flag.StringVar(&provisionerName, "talos.provisioner", "", "Talos cluster provisioner to use, if not set cluster state is disabled")
	flag.StringVar(&stateDir, "talos.state", defaultStateDir, "directory path to store cluster state")
	flag.StringVar(&clusterName, "talos.name", "talos-default", "the name of the cluster")
	flag.StringVar(&expectedVersion, "talos.version", version.Tag, "expected Talos version")
	flag.StringVar(&expectedGoVersion, "talos.go.version", constants.GoVersion, "expected Talos version")
	flag.StringVar(&talosctlPath, "talos.talosctlpath", "talosctl", "The path to 'talosctl' binary")
	flag.StringVar(&kubectlPath, "talos.kubectlpath", "kubectl", "The path to 'kubectl' binary")
	flag.StringVar(&helmPath, "talos.helmpath", "helm", "The path to 'helm' binary")
	flag.StringVar(&kubeStrPath, "talos.kubestrpath", "kubestr", "The path to 'kubestr' binary")
	flag.StringVar(&talosImage, "talos.image", images.DefaultTalosImageRepository, "The default 'talos' container image")
	flag.StringVar(&csiTestName, "talos.csi", "", "CSI test to run")
	flag.StringVar(&csiTestTimeout, "talos.csi.timeout", "15m", "CSI test timeout")

	flag.StringVar(&provision_test.DefaultSettings.CIDR, "talos.provision.cidr", provision_test.DefaultSettings.CIDR, "CIDR to use to provision clusters (provision tests only)")
	flag.Var(&provision_test.DefaultSettings.RegistryMirrors, "talos.provision.registry-mirror", "registry mirrors to use (provision tests only)")
	flag.IntVar(&provision_test.DefaultSettings.MTU, "talos.provision.mtu", provision_test.DefaultSettings.MTU, "MTU to use for cluster network (provision tests only)")
	flag.Int64Var(&provision_test.DefaultSettings.CPUs, "talos.provision.cpu", provision_test.DefaultSettings.CPUs, "CPU count for each VM (provision tests only)")
	flag.Int64Var(&provision_test.DefaultSettings.MemMB, "talos.provision.mem", provision_test.DefaultSettings.MemMB, "memory (in MiB) for each VM (provision tests only)")
	flag.Uint64Var(&provision_test.DefaultSettings.DiskGB, "talos.provision.disk", provision_test.DefaultSettings.DiskGB, "disk size (in GiB) for each VM (provision tests only)")
	flag.IntVar(&provision_test.DefaultSettings.ControlplaneNodes, "talos.provision.controlplanes", provision_test.DefaultSettings.ControlplaneNodes, "controlplane node count (provision tests only)")
	flag.IntVar(&provision_test.DefaultSettings.WorkerNodes, "talos.provision.workers", provision_test.DefaultSettings.WorkerNodes, "worker node count (provision tests only)")
	flag.StringVar(&provision_test.DefaultSettings.TargetInstallImageRegistry, "talos.provision.target-installer-registry",
		provision_test.DefaultSettings.TargetInstallImageRegistry, "image registry for target installer image (provision tests only)")
	flag.StringVar(&provision_test.DefaultSettings.CustomCNIURL, "talos.provision.custom-cni-url", provision_test.DefaultSettings.CustomCNIURL, "custom CNI URL for the cluster (provision tests only)")
	flag.StringVar(&provision_test.DefaultSettings.CNIBundleURL, "talos.provision.cni-bundle-url", provision_test.DefaultSettings.CNIBundleURL, "URL to download CNI bundle from")

	allSuites = slices.Concat(api.GetAllSuites(), cli.GetAllSuites(), k8s.GetAllSuites(), provision_test.GetAllSuites())
}
