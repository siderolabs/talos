// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package container_test

import (
	"testing"

	"github.com/siderolabs/gen/xslices"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	containercfg "github.com/siderolabs/talos/pkg/machinery/config/types/container"
)

// newContainerDoc builds a ContainerConfig document with the given dependsOn.containers list.
func newContainerDoc(name string, dependsOn ...string) *containercfg.ContainerConfigV1Alpha1 {
	doc := containercfg.NewContainerConfigV1Alpha1()
	doc.MetaName = name
	doc.ContainerImage = "docker.io/library/nginx:latest"

	if len(dependsOn) > 0 {
		doc.LifecycleConfig = &containercfg.ContainerLifecycle{
			DependsOnConfig: &containercfg.ContainerDependsOn{
				ContainersConfig: dependencies(dependsOn...),
			},
		}
	}

	return doc
}

// newDeprecatedContainerDoc is newContainerDoc, declaring the dependencies via the deprecated
// top-level dependsOn instead of lifecycle.dependsOn.
func newDeprecatedContainerDoc(name string, dependsOn ...string) *containercfg.ContainerConfigV1Alpha1 {
	doc := containercfg.NewContainerConfigV1Alpha1()
	doc.MetaName = name
	doc.ContainerImage = "docker.io/library/nginx:latest"
	doc.DependsOnConfig = &containercfg.ContainerDependsOn{ //nolint:staticcheck // testing deprecated field
		ContainersConfig: dependencies(dependsOn...),
	}

	return doc
}

// validateDocs runs container-level validation over the given documents.
func validateDocs(t *testing.T, docs ...*containercfg.ContainerConfigV1Alpha1) error {
	t.Helper()

	documents := make([]config.Document, 0, len(docs))

	for _, doc := range docs {
		documents = append(documents, doc)
	}

	ctr, err := container.New(documents...)
	require.NoError(t, err)

	_, err = ctr.ValidateAsClient(validationMode{})

	return err
}

func TestContainerDependencyCycles(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name        string
		docs        []*containercfg.ContainerConfigV1Alpha1
		expectedErr string
	}{
		{
			name: "no dependencies",
			docs: []*containercfg.ContainerConfigV1Alpha1{
				newContainerDoc("a"),
				newContainerDoc("b"),
			},
		},
		{
			name: "linear chain",
			docs: []*containercfg.ContainerConfigV1Alpha1{
				newContainerDoc("a", "b"),
				newContainerDoc("b", "c"),
				newContainerDoc("c"),
			},
		},
		{
			name: "diamond is a DAG, not a cycle",
			docs: []*containercfg.ContainerConfigV1Alpha1{
				newContainerDoc("a", "b", "c"),
				newContainerDoc("b", "d"),
				newContainerDoc("c", "d"),
				newContainerDoc("d"),
			},
		},
		{
			name: "two-node cycle",
			docs: []*containercfg.ContainerConfigV1Alpha1{
				newContainerDoc("a", "b"),
				newContainerDoc("b", "a"),
			},
			expectedErr: "container dependsOn cycle detected",
		},
		{
			name: "three-node cycle",
			docs: []*containercfg.ContainerConfigV1Alpha1{
				newContainerDoc("a", "b"),
				newContainerDoc("b", "c"),
				newContainerDoc("c", "a"),
			},
			expectedErr: "container dependsOn cycle detected",
		},
		{
			name: "dangling reference",
			docs: []*containercfg.ContainerConfigV1Alpha1{
				newContainerDoc("a", "nonexistent"),
			},
			expectedErr: `depends on container "nonexistent", which is not configured`,
		},
		{
			name: "chain through the deprecated dependsOn",
			docs: []*containercfg.ContainerConfigV1Alpha1{
				newDeprecatedContainerDoc("a", "b"),
				newContainerDoc("b", "c"),
				newContainerDoc("c"),
			},
		},
		{
			name: "dangling reference in the deprecated dependsOn",
			docs: []*containercfg.ContainerConfigV1Alpha1{
				newDeprecatedContainerDoc("a", "nonexistent"),
			},
			expectedErr: `depends on container "nonexistent", which is not configured`,
		},
		{
			name: "cycle across the deprecated and the lifecycle dependsOn",
			docs: []*containercfg.ContainerConfigV1Alpha1{
				newDeprecatedContainerDoc("a", "b"),
				newContainerDoc("b", "a"),
			},
			expectedErr: "container dependsOn cycle detected",
		},
		{
			name: "succeeded on a container which isn't restarted",
			docs: []*containercfg.ContainerConfigV1Alpha1{
				dependsOnSucceeded(newContainerDoc("a"), "init"),
				withRestartPolicy(newContainerDoc("init"), config.ContainerRestartPolicyNever),
			},
		},
		{
			name: "succeeded on a container which is always restarted",
			docs: []*containercfg.ContainerConfigV1Alpha1{
				dependsOnSucceeded(newContainerDoc("a"), "init"),
				newContainerDoc("init"),
			},
			expectedErr: `container "a" depends on container "init" having succeeded, which needs its restart policy to be on-failure or never`,
		},
		{
			name: "cycle across both conditions",
			docs: []*containercfg.ContainerConfigV1Alpha1{
				dependsOnSucceeded(newContainerDoc("a"), "init"),
				withRestartPolicy(newContainerDoc("init", "a"), config.ContainerRestartPolicyNever),
			},
			expectedErr: "container dependsOn cycle detected",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := validateDocs(t, test.docs...)

			if test.expectedErr == "" {
				require.NoError(t, err)

				return
			}

			require.Error(t, err)
			assert.Contains(t, err.Error(), test.expectedErr)
		})
	}
}

// dependencies builds dependsOn.containers entries for the given names, with the default condition.
func dependencies(names ...string) []containercfg.ContainerDependency {
	return xslices.Map(names, func(name string) containercfg.ContainerDependency {
		return containercfg.ContainerDependency{DependencyName: name}
	})
}

func lifecycle(doc *containercfg.ContainerConfigV1Alpha1) *containercfg.ContainerLifecycle {
	if doc.LifecycleConfig == nil {
		doc.LifecycleConfig = &containercfg.ContainerLifecycle{}
	}

	return doc.LifecycleConfig
}

func dependsOnSucceeded(doc *containercfg.ContainerConfigV1Alpha1, name string) *containercfg.ContainerConfigV1Alpha1 {
	lifecycle(doc).DependsOnConfig = &containercfg.ContainerDependsOn{
		ContainersConfig: []containercfg.ContainerDependency{{DependencyName: name, DependencyCondition: config.ContainerDependencyConditionSucceeded}},
	}

	return doc
}

func withRestartPolicy(doc *containercfg.ContainerConfigV1Alpha1, policy config.ContainerRestartPolicy) *containercfg.ContainerConfigV1Alpha1 {
	lifecycle(doc).RestartConfig = &containercfg.ContainerRestart{RestartPolicy: policy}

	return doc
}
