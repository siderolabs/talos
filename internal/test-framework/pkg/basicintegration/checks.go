// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package basicintegration

import (
	"context"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"github.com/talos-systems/talos/internal/test-framework/pkg/checker"
)

// retryChecks creates a list of commands that should pass their
// associated Checks.
func (b *BasicIntegration) retryChecks(ctx context.Context) []checker.Check {
	bootkubeStatus := checker.Check{
		Command: exec.CommandContext(ctx,
			"/bin/osctl",
			"--talosconfig",
			b.talosConfig,
			"service",
			"bootkube"),
		Check: func(data string) bool {
			re := regexp.MustCompile(`STATE\s+Finished`)
			return re.MatchString(data)
		},
		Name: "Wait for bootkube to complete successfully",
		Wait: 300 * time.Second,
	}

	getKubeconfig := checker.Check{
		Command: exec.CommandContext(ctx,
			"/bin/osctl",
			"--talosconfig",
			b.talosConfig,
			"kubeconfig",
			filepath.Dir(b.kubeConfig),
			"-f"),
		Check: func(data string) bool {
			// No output for kubeconfig
			// so if command exit code is 0,
			// then success
			return true
		},
		Name: "Wait for kubeconfig to be available",
		Wait: 300 * time.Second,
	}

	setKubeconfigTarget := checker.Check{
		Command: exec.CommandContext(ctx,
			"/usr/local/bin/kubectl",
			"--kubeconfig",
			b.kubeConfig,
			"config",
			"set-cluster",
			"local",
			"--server",
			"https://10.5.0.2:6443"),
		Check: func(data string) bool {
			// No output for kubectl
			// so if command exit code is 0,
			// then success
			return true
		},
		Name: "Set 10.5.0.2 for kubeconfig target",
		Wait: 2 * time.Second,
	}

	waitForAllNodes := checker.Check{
		Command: exec.CommandContext(ctx,
			"/usr/local/bin/kubectl",
			"--kubeconfig",
			b.kubeConfig,
			"get",
			"nodes",
			"-o",
			"go-template='{{ len .items}}'"),
		Check: func(data string) bool {
			numNodes, err := strconv.Atoi(data)
			if err != nil {
				return false
			}

			if numNodes != 4 {
				return false
			}

			return true
		},
		Name: "Wait for all nodes to join",
		Wait: 300 * time.Second,
	}

	waitForAllNodesReady := checker.Check{
		Command: exec.CommandContext(ctx,
			"/usr/local/bin/kubectl",
			"--kubeconfig",
			b.kubeConfig,
			"get",
			"nodes",
			"-o",
			"wide"),
		Check: func(data string) bool {
			re := regexp.MustCompile(`NotReady`)
			return !re.MatchString(data)
		},
		Name: "Wait for all nodes to be ready",
		Wait: 300 * time.Second,
	}

	waitForAllMasters := checker.Check{
		Command: exec.CommandContext(ctx,
			"/usr/local/bin/kubectl",
			"--kubeconfig",
			b.kubeConfig,
			"get",
			"nodes",
			"-l",
			"node-role.kubernetes.io/master=''",
			"-o",
			"go-template='{{ len .items}}'"),
		Check: func(data string) bool {
			numNodes, err := strconv.Atoi(data)
			if err != nil {
				return false
			}

			if numNodes != 3 {
				return false
			}

			return true
		},
		Name: "Wait for healthy control plane",
		Wait: 300 * time.Second,
	}

	waitForEtcdRunning := checker.Check{
		Command: exec.CommandContext(ctx,
			"/bin/osctl",
			"--talosconfig",
			b.talosConfig,
			"service",
			"etcd",
			"--nodes",
			"10.5.0.2,10.5.0.3,10.5.0.4"),
		Check: func(data string) bool {
			re := regexp.MustCompile(`STATE\s+Running`)
			return len(re.FindAllStringIndex(data, -1)) == 3
		},
		Name: "Wait for etcd to be running",
		Wait: 300 * time.Second,
	}

	return []checker.Check{bootkubeStatus, getKubeconfig, setKubeconfigTarget, waitForAllNodes, waitForAllNodesReady, waitForAllMasters, waitForEtcdRunning}
}

// oneShotChecks define checks that should not be retried upon failure.
func (b *BasicIntegration) oneShotChecks(ctx context.Context) []checker.Check {
	runIntegrationTest := checker.Check{
		Command: exec.CommandContext(ctx,
			"/bin/integration-test",
			"-test.v",
			"--talos.config",
			b.talosConfig),
		Check: func(data string) bool {
			return true
		},
		Name: "Run integration-test",
	}

	return []checker.Check{runIntegrationTest}
}
