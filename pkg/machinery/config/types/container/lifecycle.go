// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package container

//docgen:jsonschema

import "time"

// ContainerLifecycle configures how Talos starts, runs and stops the container.
type ContainerLifecycle struct {
	//   description: |
	//     Conditions which must be satisfied before the container is started.
	DependsOnConfig *ContainerDependsOn `yaml:"dependsOn,omitempty"`
	//   description: |
	//     Probe deciding whether the running container is healthy.
	//
	//     Without a health check, a running container is healthy. With one, it is reported as
	//     pending until the first check passes, and as degraded once `failureThreshold` consecutive
	//     checks fail. A failing check never restarts the container. Health gates other containers
	//     which list this one in `lifecycle.dependsOn.containers`.
	//
	//     The probe connects to `127.0.0.1` inside the container's network namespace.
	//   examples:
	//     - value: exampleContainerHealthCheck()
	HealthCheckConfig *ContainerHealthCheck `yaml:"healthCheck,omitempty" merge:"replace"`
	//   description: |
	//     Time the container gets to exit after SIGTERM before it is killed with SIGKILL.
	//
	//     It applies whenever Talos stops the container: on shutdown, reboot and upgrade, when it is
	//     replaced after a change or restarted with `talosctl restart`, and when it is removed.
	//     Defaults to 10s. Must be greater than 0 and at most 4m.
	//     Changing it replaces the running container, which is still stopped with its previous timeout.
	//   examples:
	//     - value: >
	//         30 * time.Second
	//   schema:
	//     type: string
	//     pattern: ^[-+]?(((\d+(\.\d*)?|\d*(\.\d+)+)([nuµm]?s|m|h))|0)+$
	StopTimeoutConfig *time.Duration `yaml:"stopTimeout,omitempty"`
	//     How the container is restarted after it exits.
	RestartConfig *ContainerRestart `yaml:"restart,omitempty"`
}
