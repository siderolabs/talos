// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package hydrophone provides functions to run Kubernetes e2e tests.
package hydrophone

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/blang/semver/v4"
	yaml "gopkg.in/yaml.v3"
	"sigs.k8s.io/hydrophone/pkg/common"
	"sigs.k8s.io/hydrophone/pkg/conformance"
	"sigs.k8s.io/hydrophone/pkg/conformance/client"
	"sigs.k8s.io/hydrophone/pkg/types"

	"github.com/siderolabs/talos/pkg/cluster"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// Options for the tests.
type Options struct {
	RunTests []string
	Skip     string
	Parallel int

	RunTimeout    time.Duration
	DeleteTimeout time.Duration

	KubernetesVersion string

	UseSpinner      bool
	RetrieveResults bool

	ResultsPath string
}

// DefaultOptions with hand-picked tests, timeouts, etc.
func DefaultOptions() *Options {
	return &Options{
		RunTests: []string{ // list of tests to cover basic kubernetes operations
			"Pods should be submitted and removed",
			"Services should serve a basic endpoint from pods",
			"Services should be able to change the type from ExternalName to ClusterIP",
		},
		Parallel: 2,

		RunTimeout:    10 * time.Minute,
		DeleteTimeout: 3 * time.Minute,

		KubernetesVersion: constants.DefaultKubernetesVersion,
	}
}

// FastConformance runs conformance suite in two passes: parallel + serial for non parallel-safe tests.
func FastConformance(ctx context.Context, cluster cluster.K8sProvider) error {
	optionsList := []Options{
		{
			RunTests: []string{`\[Conformance\]`},
			Skip:     `\[Serial\]`,
			Parallel: 16,

			RunTimeout:    time.Hour,
			DeleteTimeout: 5 * time.Minute,

			KubernetesVersion: constants.DefaultKubernetesVersion,

			UseSpinner: true,
		},
		{
			RunTests: []string{`\[Serial\].*\[Conformance\]`},

			RunTimeout:    time.Hour,
			DeleteTimeout: 5 * time.Minute,

			KubernetesVersion: constants.DefaultKubernetesVersion,

			UseSpinner: true,
		},
	}

	for _, options := range optionsList {
		if err := Run(ctx, cluster, &options); err != nil {
			return err
		}
	}

	return nil
}

// CertifiedConformance runs conformance suite in certified mode collecting all the results.
func CertifiedConformance(ctx context.Context, cluster cluster.K8sProvider) error {
	options := Options{
		RunTests: []string{`\[Conformance\]`},

		RunTimeout:    2 * time.Hour,
		DeleteTimeout: 5 * time.Minute,

		KubernetesVersion: constants.DefaultKubernetesVersion,
		RetrieveResults:   true,

		UseSpinner: true,
	}

	k8sVersion, err := semver.ParseTolerant(options.KubernetesVersion)
	if err != nil {
		return err
	}

	options.ResultsPath = fmt.Sprintf("v%d.%d/talos", k8sVersion.Major, k8sVersion.Minor)

	if err = os.MkdirAll(options.ResultsPath, 0o755); err != nil {
		return err
	}

	return Run(ctx, cluster, &options)
}

// Run the e2e test against cluster with provided options.
//
//nolint:gocyclo
func Run(ctx context.Context, cluster cluster.K8sProvider, options *Options) error {
	cfg, err := cluster.K8sRestConfig(ctx)
	if err != nil {
		return fmt.Errorf("error getting kubernetes config: %w", err)
	}

	// reset timeout to prevent log streaming from timing out
	cfg.Timeout = 0

	config := types.NewDefaultConfiguration()
	config.ConformanceImage = fmt.Sprintf("registry.k8s.io/conformance:v%s", options.KubernetesVersion)
	config.OutputDir = options.ResultsPath
	config.Parallel = options.Parallel
	config.Skip = options.Skip

	clientset, err := cluster.K8sClient(ctx)
	if err != nil {
		return fmt.Errorf("error getting kubernetes client: %w", err)
	}

	testRunner := conformance.NewTestRunner(config, clientset)
	testClient := client.NewClient(cfg, clientset, config.Namespace)

	cleanup := func() error {
		if err := testRunner.Cleanup(ctx); err != nil {
			return fmt.Errorf("failed to cleanup: %w", err)
		}

		return nil
	}

	defer cleanup() //nolint:errcheck

	if err = cleanup(); err != nil {
		return err
	}

	verboseGinkgo := config.Verbosity >= 6
	showSpinner := !verboseGinkgo && config.Verbosity > 2 && options.UseSpinner && os.Getenv("CI") == ""

	fmt.Printf("running conformance tests version %s\n", options.KubernetesVersion)
	fmt.Printf("running tests: %s\n", strings.Join(options.RunTests, "|"))

	if err := testRunner.Deploy(ctx, strings.Join(options.RunTests, "|"), "", verboseGinkgo, config.StartupTimeout); err != nil {
		return fmt.Errorf("failed to deploy tests: %w", err)
	}

	before := time.Now()

	var spinner *common.Spinner

	if showSpinner {
		spinner = common.NewSpinner(os.Stdout)
		spinner.Start()
	}

	// PrintE2ELogs is a long running method
	if err := testClient.PrintE2ELogs(ctx); err != nil {
		return fmt.Errorf("failed to get test logs: %w", err)
	}

	if showSpinner {
		spinner.Stop()
	}

	fmt.Printf("tests finished after %v.\n", time.Since(before).Round(time.Second))

	exitCode, err := testClient.FetchExitCode(ctx)
	if err != nil {
		return fmt.Errorf("failed to determine exit code: %w", err)
	}

	if exitCode == 0 {
		fmt.Println("tests completed successfully")
	} else {
		return fmt.Errorf("tests failed: code %d", exitCode)
	}

	if options.RetrieveResults {
		if err := testClient.FetchFiles(ctx, config.OutputDir); err != nil {
			return fmt.Errorf("failed to download results: %w", err)
		}

		productInfo, err := yaml.Marshal(talos)
		if err != nil {
			return fmt.Errorf("error marshaling product info: %w", err)
		}

		if err = os.WriteFile(filepath.Join(options.ResultsPath, "PRODUCT.yaml"), productInfo, 0o644); err != nil {
			return err
		}
	}

	return cleanup()
}
