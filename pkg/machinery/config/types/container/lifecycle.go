// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package container

//docgen:jsonschema

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
}
