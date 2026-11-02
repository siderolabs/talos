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
}
