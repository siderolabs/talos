// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package sonobuoy provides functions to to run Kubernetes e2e tests.
package sonobuoy

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/vmware-tanzu/sonobuoy/cmd/sonobuoy/app"
	"github.com/vmware-tanzu/sonobuoy/pkg/client"
	"github.com/vmware-tanzu/sonobuoy/pkg/config"
	sonodynamic "github.com/vmware-tanzu/sonobuoy/pkg/dynamic"

	"github.com/talos-systems/talos/pkg/cluster"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// Options for the tests.
type Options struct {
	RunTests []string

	RunTimeout    time.Duration
	DeleteTimeout time.Duration

	KubernetesVersion string

	UseSpinner bool
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

// Run the e2e test against cluster with provided options.
//
//nolint: gocyclo
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

	defer logF.Close() //nolint: errcheck

	go func() {
		io.Copy(logF, logReader) //nolint: errcheck
	}()

	cleanup := func() error {
		os.Remove(logF.Name()) //nolint: errcheck

		return sclient.Delete(&client.DeleteConfig{
			Namespace:  config.DefaultNamespace,
			DeleteAll:  true,
			Wait:       options.DeleteTimeout,
			WaitOutput: waitOutput,
		})
	}

	defer cleanup() //nolint: errcheck

	runConfig := client.NewRunConfig()
	runConfig.Wait = options.RunTimeout
	runConfig.WaitOutput = waitOutput

	runConfig.E2EConfig = &client.E2EConfig{
		Focus:    strings.Join(options.RunTests, "|"),
		Parallel: "false",
	}
	runConfig.DynamicPlugins = []string{"e2e"}
	runConfig.KubeConformanceImage = fmt.Sprintf("%s:v%s", config.UpstreamKubeConformanceImageURL, options.KubernetesVersion)

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

			logF.Seek(0, io.SeekStart) //nolint: errcheck
			io.Copy(os.Stdout, logF)   //nolint: errcheck

			return fmt.Errorf("e2e plugin status: %q", pluginStatus.ResultStatus)
		}
	}

	if !e2ePassed {
		return fmt.Errorf("missing e2e plugin status")
	}

	return cleanup()
}
