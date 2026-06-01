// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package services //nolint:testpackage // to test unexported variable

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/api"
)

func collectMethods(t *testing.T) map[string]struct{} {
	methods := make(map[string]struct{})

	for _, service := range api.TalosAPIdAllAPIs() {
		for i := range service.Services().Len() {
			svc := service.Services().Get(i)

			for j := range svc.Methods().Len() {
				method := svc.Methods().Get(j)

				s := fmt.Sprintf("/%s/%s", svc.FullName(), method.Name())
				require.NotContains(t, methods, s)
				methods[s] = struct{}{}
			}
		}
	}

	return methods
}

func TestRules(t *testing.T) {
	t.Parallel()

	methods := collectMethods(t)

	// check that there are no rules without matching methods
	t.Run("NoMethodForRule", func(t *testing.T) {
		t.Parallel()

		for rule := range rules {
			_, ok := methods[rule]
			assert.True(t, ok, "no method for rule %q", rule)
		}
	})

	// check that there are no methods without matching rules
	t.Run("NoRuleForMethod", func(t *testing.T) {
		t.Parallel()

		for method := range methods {
			_, ok := rules[method]
			assert.True(t, ok, "no rule for method %q", method)
		}
	})
}
