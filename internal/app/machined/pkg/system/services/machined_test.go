// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package services //nolint:testpackage // to test unexported variable

import (
	"fmt"
	"testing"

	cosi "github.com/cosi-project/runtime/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/siderolabs/talos/pkg/machinery/api/cluster"
	"github.com/siderolabs/talos/pkg/machinery/api/inspect"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/api/storage"
	"github.com/siderolabs/talos/pkg/machinery/api/time"
)

func collectMethods(t *testing.T) map[string]struct{} {
	methods := make(map[string]struct{})

	for _, service := range []grpc.ServiceDesc{
		cosi.State_ServiceDesc,
		cluster.ClusterService_ServiceDesc,
		inspect.InspectService_ServiceDesc,
		machine.MachineService_ServiceDesc,
		// security.SecurityService_ServiceDesc, - not in machined
		storage.StorageService_ServiceDesc,
		time.TimeService_ServiceDesc,
	} {
		for _, method := range service.Methods {
			s := fmt.Sprintf("/%s/%s", service.ServiceName, method.MethodName)
			require.NotContains(t, methods, s)
			methods[s] = struct{}{}
		}

		for _, stream := range service.Streams {
			s := fmt.Sprintf("/%s/%s", service.ServiceName, stream.StreamName)
			require.NotContains(t, methods, s)
			methods[s] = struct{}{}
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
