// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package sonobuoy provides functions to to run Kubernetes e2e tests.
package sonobuoy

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/vmware-tanzu/sonobuoy/cmd/sonobuoy/app"
	"github.com/vmware-tanzu/sonobuoy/pkg/client"
	"github.com/vmware-tanzu/sonobuoy/pkg/config"
	sonodynamic "github.com/vmware-tanzu/sonobuoy/pkg/dynamic"
	yaml "gopkg.in/yaml.v3"

	"github.com/talos-systems/talos/pkg/cluster"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// Options for the tests.
type Options struct {
	RunTests []string
	Parallel bool

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
			Parallel: true,

			RunTimeout:    time.Hour,
			DeleteTimeout: 5 * time.Minute,

			KubernetesVersion: constants.DefaultKubernetesVersion,
		},
		{
			RunTests: []string{`\[Serial\].*\[Conformance\]`},
			Parallel: false,

			RunTimeout:    time.Hour,
			DeleteTimeout: 5 * time.Minute,

			KubernetesVersion: constants.DefaultKubernetesVersion,
		},
	}

	for _, options := range optionsList {
		options := options

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
		Parallel: false,

		RunTimeout:    2 * time.Hour,
		DeleteTimeout: 5 * time.Minute,

		KubernetesVersion: constants.DefaultKubernetesVersion,
		RetrieveResults:   true,
	}

	k8sVersion, err := semver.NewVersion(options.KubernetesVersion)
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
//nolint:gocyclo,cyclop
func Run(ctx context.Context, cluster cluster.K8sProvider, options *Options) error {
	var waitOutput string

	if options.UseSpinner {
		waitOutput = string(app.SpinnerOutputMode)
	} else {
		waitOutput = string(app.SilentOutputMode)
	}

	cfg, err := cluster.K8sRestConfig(ctx)
	if err != nil {
		return fmt.Errorf("error getting kubernetes config: %w", err)
	}

	skc, err := sonodynamic.NewAPIHelperFromRESTConfig(cfg)
	if err != nil {
		return fmt.Errorf("couldn't get sonobuoy api helper: %w", err)
	}

	e2ePassed := false

	sclient, err := client.NewSonobuoyClient(cfg, skc)
	if err != nil {
		return fmt.Errorf("error building sonobuoy client: %w", err)
	}

	logReader, err := sclient.LogReader(&client.LogConfig{
		Namespace: config.DefaultNamespace,
		Follow:    true,
		Plugin:    "e2e",
	})
	if err != nil {
		return fmt.Errorf("error setting up log reader: %w", err)
	}

	logF, err := ioutil.TempFile("", "talos")
	if err != nil {
		return fmt.Errorf("error creating temporary file for logs: %w", err)
	}

	defer logF.Close() //nolint:errcheck

	go func() {
		io.Copy(logF, logReader) //nolint:errcheck
	}()

	cleanup := func() error {
		os.Remove(logF.Name()) //nolint:errcheck

		return sclient.Delete(&client.DeleteConfig{
			Namespace:  config.DefaultNamespace,
			DeleteAll:  true,
			Wait:       options.DeleteTimeout,
			WaitOutput: waitOutput,
		})
	}

	defer cleanup() //nolint:errcheck

	runConfig := client.NewRunConfig()
	runConfig.Wait = options.RunTimeout
	runConfig.WaitOutput = waitOutput

	runConfig.DynamicPlugins = []string{"e2e"}
	runConfig.PluginEnvOverrides = map[string]map[string]string{
		"e2e": {
			"E2E_FOCUS":    strings.Join(options.RunTests, "|"),
			"E2E_PARALLEL": fmt.Sprintf("%v", options.Parallel),
		},
	}
	runConfig.KubeVersion = fmt.Sprintf("v%s", options.KubernetesVersion)

	if err = sclient.Run(runConfig); err != nil {
		return fmt.Errorf("sonobuoy run failed: %w", err)
	}

	status, err := sclient.GetStatus(&client.StatusConfig{
		Namespace: config.DefaultNamespace,
	})
	if err != nil {
		return fmt.Errorf("error getting test status: %w", err)
	}

	for _, pluginStatus := range status.Plugins {
		if pluginStatus.Plugin == "e2e" {
			fmt.Print("e2e status ")

			for label, count := range pluginStatus.ResultStatusCounts {
				fmt.Printf("%s:%d ", label, count)
			}

			fmt.Println()

			if pluginStatus.ResultStatus == "passed" {
				e2ePassed = true

				break
			}

			fmt.Println("\ne2e plugin logs:")

			logF.Seek(0, io.SeekStart) //nolint:errcheck
			io.Copy(os.Stdout, logF)   //nolint:errcheck

			return fmt.Errorf("e2e plugin status: %q", pluginStatus.ResultStatus)
		}
	}

	if !e2ePassed {
		return fmt.Errorf("missing e2e plugin status")
	}

	if options.RetrieveResults {
		resultR, errCh, err := sclient.RetrieveResults(&client.RetrieveConfig{
			Namespace: config.DefaultNamespace,
			Path:      config.AggregatorResultsPath,
		})
		if err != nil {
			return fmt.Errorf("error retrieving results: %w", err)
		}

		if resultR == nil {
			return fmt.Errorf("no result reader")
		}

		gzipR, err := gzip.NewReader(resultR)
		if err != nil {
			return err
		}

		defer gzipR.Close() //nolint:errcheck

		tarR := tar.NewReader(gzipR)

		for {
			var header *tar.Header

			header, err = tarR.Next()
			if err != nil {
				if err == io.EOF {
					break
				}

				return err
			}

			matched, _ := filepath.Match("*_sonobuoy_*.tar.gz", header.Name) //nolint:errcheck

			if !matched {
				continue
			}

			var innerGzipR *gzip.Reader

			innerGzipR, err = gzip.NewReader(tarR)
			if err != nil {
				return err
			}

			defer innerGzipR.Close() //nolint:errcheck

			innnerTarR := tar.NewReader(innerGzipR)

			for {
				header, err = innnerTarR.Next()
				if err != nil {
					if err == io.EOF {
						break
					}

					return err
				}

				writeFile := func(name string) error {
					var data []byte

					data, err = io.ReadAll(innnerTarR)
					if err != nil {
						return err
					}

					return os.WriteFile(filepath.Join(options.ResultsPath, name), data, 0o644)
				}

				switch header.Name {
				case "plugins/e2e/results/global/junit_01.xml":
					if err = writeFile("junit_01.xml"); err != nil {
						return err
					}
				case "plugins/e2e/results/global/e2e.log":
					if err = writeFile("e2e.log"); err != nil {
						return err
					}
				}
			}
		}

		select {
		case err = <-errCh:
			if err != nil {
				return err
			}
		default:
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
