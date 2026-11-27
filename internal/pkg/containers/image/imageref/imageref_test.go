// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package imageref_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/containers/image/imageref"
)

func TestParse(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		imageRef string

		expected      string
		expectedKey   string
		expectedError string
	}{
		{
			imageRef: "registry.k8s.io/pause:3.9",

			expected:    "registry.k8s.io/pause:3.9",
			expectedKey: "registry.k8s.io/pause",
		},
		{ // the registry domain is a DNS name, so its case is not significant
			imageRef: "REGISTRY.K8S.IO/pause:3.9",

			expected:    "registry.k8s.io/pause:3.9",
			expectedKey: "registry.k8s.io/pause",
		},
		{
			imageRef: "Registry.K8s.IO:5000/pause:3.9",

			expected:    "registry.k8s.io:5000/pause:3.9",
			expectedKey: "registry.k8s.io:5000/pause",
		},
		{ // the default port is not a part of the canonical reference
			imageRef: "registry.k8s.io:443/pause:3.9",

			expected:    "registry.k8s.io/pause:3.9",
			expectedKey: "registry.k8s.io/pause",
		},
		{ // leading zeros are valid in a port, and the dialer ignores them
			imageRef: "registry.k8s.io:0443/pause:3.9",

			expected:    "registry.k8s.io/pause:3.9",
			expectedKey: "registry.k8s.io/pause",
		},
		{
			imageRef: "REGISTRY.K8S.IO:00000443/pause@sha256:7031c1b283388d2c2e09b57badb803c05ebed362dc88d84b480cc47f72a21097",

			expected:    "registry.k8s.io/pause@sha256:7031c1b283388d2c2e09b57badb803c05ebed362dc88d84b480cc47f72a21097",
			expectedKey: "registry.k8s.io/pause",
		},
		{ // any other port gets its canonical spelling
			imageRef: "registry.k8s.io:05000/pause:3.9",

			expected:    "registry.k8s.io:5000/pause:3.9",
			expectedKey: "registry.k8s.io:5000/pause",
		},
		{
			imageRef: "registry.k8s.io:4430/pause:3.9",

			expected:    "registry.k8s.io:4430/pause:3.9",
			expectedKey: "registry.k8s.io:4430/pause",
		},
		{
			imageRef: "localhost:443/foo",

			expected:    "localhost/foo:latest",
			expectedKey: "localhost/foo",
		},
		{
			imageRef: "[::1]:443/foo",

			expected:    "[::1]/foo:latest",
			expectedKey: "[::1]/foo",
		},
		{
			imageRef: "[::1]/foo",

			expected:    "[::1]/foo:latest",
			expectedKey: "[::1]/foo",
		},
		{
			imageRef: "[FE80::1]:05000/foo",

			expected:    "[fe80::1]:5000/foo:latest",
			expectedKey: "[fe80::1]:5000/foo",
		},
		{ // a single-label host is a registry domain only because of its port: dropping the
			// port would turn it into a Docker Hub namespace
			imageRef: "registry:443/foo",

			expected:    "registry:443/foo:latest",
			expectedKey: "registry:443/foo",
		},
		{
			imageRef: "Registry:0443/foo",

			expected:    "registry:443/foo:latest",
			expectedKey: "registry:443/foo",
		},
		{ // the Docker Hub spellings fold once the default port is dropped
			imageRef: "registry-1.docker.io:443/library/alpine:3.19",

			expected:    "docker.io/library/alpine:3.19",
			expectedKey: "docker.io/library/alpine",
		},
		{
			imageRef: "INDEX.DOCKER.IO:0443/alpine",

			expected:    "docker.io/library/alpine:latest",
			expectedKey: "docker.io/library/alpine",
		},
		{
			imageRef: "docker.io:443/alpine:3.19",

			expected:    "docker.io/library/alpine:3.19",
			expectedKey: "docker.io/library/alpine",
		},
		{
			imageRef: "registry.hub.docker.com/library/alpine:3.19",

			expected:    "docker.io/library/alpine:3.19",
			expectedKey: "docker.io/library/alpine",
		},
		{ // on any other port it is not known to be Docker Hub
			imageRef: "index.docker.io:5000/library/alpine:3.19",

			expected:    "index.docker.io:5000/library/alpine:3.19",
			expectedKey: "index.docker.io:5000/library/alpine",
		},
		{
			imageRef: "docker.io/library/alpine:3.19",

			expected:    "docker.io/library/alpine:3.19",
			expectedKey: "docker.io/library/alpine",
		},
		{ // the legacy Docker Hub domain folds into docker.io
			imageRef: "index.docker.io/library/alpine:3.19",

			expected:    "docker.io/library/alpine:3.19",
			expectedKey: "docker.io/library/alpine",
		},
		{
			imageRef: "INDEX.DOCKER.IO/library/alpine:3.19",

			expected:    "docker.io/library/alpine:3.19",
			expectedKey: "docker.io/library/alpine",
		},
		{ // the Docker Hub registry endpoint is the same registry as docker.io, so a
			// reference written against it must not evade a rule written for docker.io
			imageRef: "registry-1.docker.io/library/alpine:3.19",

			expected:    "docker.io/library/alpine:3.19",
			expectedKey: "docker.io/library/alpine",
		},
		{
			imageRef: "Registry-1.Docker.IO/alpine",

			expected:    "docker.io/library/alpine:latest",
			expectedKey: "docker.io/library/alpine",
		},
		{ // the implicit library/ namespace is applied after the domain is normalized
			imageRef: "INDEX.DOCKER.IO/alpine",

			expected:    "docker.io/library/alpine:latest",
			expectedKey: "docker.io/library/alpine",
		},
		{
			imageRef: "DOCKER.IO/alpine:3.19",

			expected:    "docker.io/library/alpine:3.19",
			expectedKey: "docker.io/library/alpine",
		},
		{
			imageRef: "alpine",

			expected:    "docker.io/library/alpine:latest",
			expectedKey: "docker.io/library/alpine",
		},
		{
			imageRef: "GHCR.IO/siderolabs/kubelet@sha256:3fc16b37247f6f154d0ebf7428a28f89079a0a138c92c91fe975803d2e19ef2b",

			expected:    "ghcr.io/siderolabs/kubelet@sha256:3fc16b37247f6f154d0ebf7428a28f89079a0a138c92c91fe975803d2e19ef2b",
			expectedKey: "ghcr.io/siderolabs/kubelet",
		},
		{ // a reference carrying both a tag and a digest keeps only the digest
			imageRef: "GHCR.IO/siderolabs/kubelet:v1.34.1@sha256:3fc16b37247f6f154d0ebf7428a28f89079a0a138c92c91fe975803d2e19ef2b",

			expected:    "ghcr.io/siderolabs/kubelet@sha256:3fc16b37247f6f154d0ebf7428a28f89079a0a138c92c91fe975803d2e19ef2b",
			expectedKey: "ghcr.io/siderolabs/kubelet",
		},
		{
			imageRef: "LOCALHOST:5000/foo",

			expected:    "localhost:5000/foo:latest",
			expectedKey: "localhost:5000/foo",
		},
		{ // an uppercase repository path is not a valid reference, and lower-casing the
			// domain must not turn one into a valid reference
			imageRef: "registry.k8s.io/Pause:3.9",

			expectedError: "invalid reference format: repository name (Pause) must be lowercase",
		},
		{ // a single-label component is only taken for a registry domain because it is not
			// lower-case; it has no canonical spelling, so it is rejected outright
			imageRef: "Foo/bar",

			expectedError: `invalid reference format: registry domain "Foo" must be lowercase`,
		},
		{
			imageRef: "LOCALHOST/foo",

			expected:    "localhost/foo:latest",
			expectedKey: "localhost/foo",
		},
		{
			imageRef: "",

			expectedError: "invalid reference format",
		},
	} {
		t.Run(test.imageRef, func(t *testing.T) {
			t.Parallel()

			namedRef, err := imageref.Parse(test.imageRef)

			if test.expectedError != "" {
				require.Error(t, err)
				assert.EqualError(t, err, test.expectedError)

				return
			}

			require.NoError(t, err)

			assert.Equal(t, test.expected, namedRef.String())
			assert.Equal(t, test.expectedKey, imageref.RepositoryKey(namedRef))

			// normalization is idempotent: whichever spelling of the reference reaches the
			// policy check, the pull and the registry configuration lookup, it is the same one
			reparsed, err := imageref.Parse(namedRef.String())
			require.NoError(t, err)
			assert.Equal(t, test.expected, reparsed.String())
			assert.Equal(t, test.expectedKey, imageref.RepositoryKey(reparsed))
		})
	}
}

func TestNormalizePattern(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		pattern string

		expected string
	}{
		{
			pattern:  "registry.k8s.io/*",
			expected: "registry.k8s.io/*",
		},
		{
			pattern:  "REGISTRY.K8S.IO/*",
			expected: "registry.k8s.io/*",
		},
		{ // the pattern shape given as the first example in the config reference
			pattern:  "docker.io/library/nginx",
			expected: "docker.io/library/nginx",
		},
		{
			pattern:  "index.docker.io/library/alpine*",
			expected: "docker.io/library/alpine*",
		},
		{
			pattern:  "Index.Docker.IO/library/alpine*",
			expected: "docker.io/library/alpine*",
		},
		{
			pattern:  "*",
			expected: "*",
		},
		{ // a glob in the domain is matched as written
			pattern:  "*.docker.io/library/*",
			expected: "*.docker.io/library/*",
		},
		{ // the whole pattern is lower-cased, as the repository key always is
			pattern:  "*.Example.COM/Team/*",
			expected: "*.example.com/team/*",
		},
		{ // the default port is dropped, just as it is from a reference
			pattern:  "registry.k8s.io:443/*",
			expected: "registry.k8s.io/*",
		},
		{
			pattern:  "REGISTRY.K8S.IO:0443/*",
			expected: "registry.k8s.io/*",
		},
		{
			pattern:  "registry.k8s.io:443",
			expected: "registry.k8s.io",
		},
		{ // including the port behind a glob
			pattern:  "*.example.com:443/*",
			expected: "*.example.com/*",
		},
		{
			pattern:  "*.example.com:0443/team/*",
			expected: "*.example.com/team/*",
		},
		{
			pattern:  "*.example.com:05000/*",
			expected: "*.example.com:5000/*",
		},
		{
			pattern:  "*:443/*",
			expected: "*/*",
		},
		{ // the port is under the glob, so it is matched as written
			pattern:  "registry.k8s.io:44*",
			expected: "registry.k8s.io:44*",
		},
		{
			pattern:  "registry.k8s.io:*/pause",
			expected: "registry.k8s.io:*/pause",
		},
		{
			pattern:  "registry-1.docker.io:443/library/*",
			expected: "docker.io/library/*",
		},
		{
			pattern:  "registry.hub.docker.com/library/*",
			expected: "docker.io/library/*",
		},
		{ // a single-label host keeps its port in a reference, and so it does in a pattern
			pattern:  "registry:0443/*",
			expected: "registry:443/*",
		},
		{
			pattern:  "[::1]:443/*",
			expected: "[::1]/*",
		},
		{
			pattern:  "*/library/*",
			expected: "*/library/*",
		},
		{
			pattern:  "nginx*",
			expected: "nginx*",
		},
		{ // a pattern carrying no `/` is still anchored at the registry domain, so the
			// domain literal in front of the glob is folded just as it is with a `/`
			pattern:  "index.docker.io*",
			expected: "docker.io*",
		},
		{
			pattern:  "registry-1.docker.io/library/alpine*",
			expected: "docker.io/library/alpine*",
		},
		{
			pattern:  "REGISTRY-1.DOCKER.IO*",
			expected: "docker.io*",
		},
		{
			pattern:  "docker.io*",
			expected: "docker.io*",
		},
		{ // the literal in front of the glob is not a complete domain, so there is nothing
			// to fold, only to lower-case
			pattern:  "INDEX.DOCKER.I*",
			expected: "index.docker.i*",
		},
		{
			pattern:  "REGISTRY.K8S.IO",
			expected: "registry.k8s.io",
		},
		{
			pattern:  "localhost:5000/*",
			expected: "localhost:5000/*",
		},
	} {
		t.Run(test.pattern, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.expected, imageref.NormalizePattern(test.pattern))
			assert.Equal(t, test.expected, imageref.NormalizePattern(test.expected), "normalization should be idempotent")
		})
	}
}
