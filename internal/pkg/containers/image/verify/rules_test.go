// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package verify_test

import (
	"fmt"
	"testing"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/containers/image/verify"
	"github.com/siderolabs/talos/pkg/machinery/resources/security"
)

func TestRuleMatcher(t *testing.T) {
	t.Parallel()

	patterns := []string{
		"registry.k8s.io/*",
		"docker.io/library/busybox*",
		"index.docker.io/library/alpine*",
		"ghcr.io/siderolabs/*",
		"index.docker.io*",
	}

	st := state.WrapCore(namespaced.NewState(inmem.Build))

	for idx, pattern := range patterns {
		rule := security.NewImageVerificationRule(fmt.Sprintf("%04d", idx))
		rule.TypedSpec().ImagePattern = pattern
		rule.TypedSpec().Deny = true

		require.NoError(t, st.Create(t.Context(), rule))
	}

	matcher, err := verify.NewRuleMatcher(t.Context(), st)
	require.NoError(t, err)

	for _, test := range []struct {
		imageRef string

		expectedRuleID string
		expectedError  string
	}{
		{
			imageRef:       "registry.k8s.io/pause:3.9",
			expectedRuleID: "0000",
		},
		{ // the registry domain is a DNS name, so upper-casing it must not evade the rule
			imageRef:       "REGISTRY.K8S.IO/pause:3.9",
			expectedRuleID: "0000",
		},
		{
			imageRef:       "registry.k8s.io/pause@sha256:7031c1b283388d2c2e09b57badb803c05ebed362dc88d84b480cc47f72a21097",
			expectedRuleID: "0000",
		},
		{ // a rule written the way the configuration reference shows it matches Docker Hub
			imageRef:       "docker.io/library/busybox:1.36",
			expectedRuleID: "0001",
		},
		{
			imageRef:       "busybox:1.36",
			expectedRuleID: "0001",
		},
		{
			imageRef:       "index.docker.io/library/busybox:1.36",
			expectedRuleID: "0001",
		},
		{ // registry-1.docker.io is the Docker Hub endpoint docker.io itself resolves to, so a
			// reference written that way must not evade a rule written for docker.io
			imageRef:       "registry-1.docker.io/library/busybox:1.36",
			expectedRuleID: "0001",
		},
		{
			imageRef:       "REGISTRY-1.DOCKER.IO/busybox:1.36",
			expectedRuleID: "0001",
		},
		{ // and so does a rule written against the legacy Docker Hub domain
			imageRef:       "docker.io/library/alpine:3.19",
			expectedRuleID: "0002",
		},
		{
			imageRef:       "DOCKER.IO/library/alpine:3.19",
			expectedRuleID: "0002",
		},
		{
			imageRef:       "ghcr.io/siderolabs/kubelet:v1.34.1",
			expectedRuleID: "0003",
		},
		{ // a rule pattern carrying no `/` is anchored at the registry domain and is folded
			// just like one written as `index.docker.io/*`
			imageRef:       "nginx:latest",
			expectedRuleID: "0004",
		},
		{
			imageRef: "quay.io/some/image:v1.0.0",
		},
		{ // the tag is never part of the match
			imageRef: "quay.io/registry.k8s.io:latest",
		},
		{
			imageRef:      "registry.k8s.io/Pause:3.9",
			expectedError: `failed to parse image reference "registry.k8s.io/Pause:3.9": invalid reference format: repository name (Pause) must be lowercase`,
		},
	} {
		t.Run(test.imageRef, func(t *testing.T) {
			t.Parallel()

			rule, err := matcher(test.imageRef)

			if test.expectedError != "" {
				require.Error(t, err)
				assert.EqualError(t, err, test.expectedError)

				return
			}

			require.NoError(t, err)

			if test.expectedRuleID == "" {
				assert.Nil(t, rule)

				return
			}

			require.NotNil(t, rule)
			assert.Equal(t, test.expectedRuleID, rule.Metadata().ID())
		})
	}
}
