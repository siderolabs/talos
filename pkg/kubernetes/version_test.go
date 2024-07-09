// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubernetes_test

import (
	"testing"

	"github.com/blang/semver/v4"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/kubernetes"
)

func TestVersionGTE(t *testing.T) {
	for _, test := range []struct {
		name string

		image   string
		version semver.Version

		expected bool
	}{
		{
			name:    "tagged image",
			image:   "registry.k8s.io/kube-apiserver:v1.30.0",
			version: semver.MustParse("1.30.0"),

			expected: true,
		},
		{
			name:    "tagged image, not less",
			image:   "registry.k8s.io/kube-apiserver:v1.29.8",
			version: semver.MustParse("1.30.0"),

			expected: false,
		},
		{
			name:    "tagged image, alpha",
			image:   "registry.k8s.io/kube-apiserver:v1.30.0-alpha.3",
			version: semver.MustParse("1.30.0"),

			expected: true,
		},
		{
			name:    "tagged and digested image",
			image:   "registry.k8s.io/kube-apiserver:v1.30.0@sha256:9efd51eb47ecdd66b9426d9361edca2cbed38d57c4fe9d81213867310a1fdd99",
			version: semver.MustParse("1.30.0"),

			expected: true,
		},
		{
			name:    "invalid tag",
			image:   "registry.k8s.io/kube-apiserver:latest",
			version: semver.MustParse("1.30.0"),

			expected: false,
		},
		{
			name:    "only digest",
			image:   "registry.k8s.io/kube-apiserver@sha256:9efd51eb47ecdd66b9426d9361edca2cbed38d57c4fe9d81213867310a1fdd99",
			version: semver.MustParse("1.30.0"),

			expected: false,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.expected, kubernetes.VersionGTE(test.image, test.version))
		})
	}
}
