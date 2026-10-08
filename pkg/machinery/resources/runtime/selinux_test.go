// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// TestSELinuxPolicyCondition: the condition holds once the status lists every module of the machine config, loaded or
// rejected; a module removed while the condition waits is not waited for.
func TestSELinuxPolicyCondition(t *testing.T) {
	t.Parallel()

	st := state.WrapCore(namespaced.NewState(inmem.Build))

	status := runtime.NewSELinuxPolicyStatus()
	status.TypedSpec().Modules = []string{"bad", "hostmon"}
	status.TypedSpec().Error = "module bad rejected: secilc"
	require.NoError(t, st.Create(t.Context(), status))

	var modules atomic.Pointer[[]string]

	modules.Store(&[]string{"hostmon", "bad"})

	condition := runtime.NewSELinuxPolicyCondition(st, func() []string { return *modules.Load() })

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	// a rejected module counts as reconciled
	require.NoError(t, condition.Wait(ctx))

	modules.Store(&[]string{"hostmon", "cni"})

	done := make(chan error, 1)

	go func() { done <- condition.Wait(ctx) }()

	select {
	case err := <-done:
		t.Fatalf("the condition does not wait for the module cni: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	// the document cni is removed before the policy controller reconciles it, which rewrites the status
	modules.Store(&[]string{"hostmon"})

	require.NoError(t, safe.StateModify(ctx, st, runtime.NewSELinuxPolicyStatus(), func(res *runtime.SELinuxPolicyStatus) error {
		res.TypedSpec().Modules = []string{"hostmon"}
		res.TypedSpec().Error = ""

		return nil
	}))

	assert.NoError(t, <-done)
}
