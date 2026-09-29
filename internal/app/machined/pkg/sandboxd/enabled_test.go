// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build linux

package sandboxd_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/sandboxd"
	configcfg "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	runtimecfg "github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
)

// mockRuntime implements just enough of runtime.Runtime for the predicates.
type mockRuntime struct {
	runtime.Runtime

	state runtime.State
	cfg   configcfg.Config
}

func (m *mockRuntime) State() runtime.State     { return m.state }
func (m *mockRuntime) Config() configcfg.Config { return m.cfg }

type mockState struct {
	runtime.State

	platform runtime.Platform
}

func (m *mockState) Platform() runtime.Platform { return m.platform }

type mockPlatform struct {
	runtime.Platform

	mode runtime.Mode
}

func (m *mockPlatform) Mode() runtime.Mode { return m.mode }

func newMockRuntime(mode runtime.Mode, documents ...configcfg.Document) *mockRuntime {
	rt := &mockRuntime{
		state: &mockState{platform: &mockPlatform{mode: mode}},
	}

	if len(documents) > 0 {
		ctr, err := container.New(documents...)
		if err != nil {
			panic(err)
		}

		rt.cfg = ctr
	}

	return rt
}

func securityProfile(workloadIsolation bool) configcfg.Document {
	doc := runtimecfg.NewSecurityProfileConfigV1Alpha1()
	doc.WorkloadIsolationEnabled = &workloadIsolation

	return doc
}

func TestServiceEnabled(t *testing.T) {
	t.Parallel()

	// sandboxd is started before the machine config is loaded, so the predicate
	// must not depend on it in any mode.
	for _, mode := range []runtime.Mode{runtime.ModeMetal, runtime.ModeCloud} {
		assert.True(t, sandboxd.ServiceEnabled(newMockRuntime(mode)))
		assert.True(t, sandboxd.ServiceEnabled(newMockRuntime(mode, securityProfile(false))))
	}

	assert.False(t, sandboxd.ServiceEnabled(newMockRuntime(runtime.ModeContainer)))
	assert.False(t, sandboxd.ServiceEnabled(newMockRuntime(runtime.ModeMetalAgent)))
}

func TestIsolationEnabled(t *testing.T) {
	t.Parallel()

	// no configuration yet: indistinguishable from isolation being disabled.
	assert.False(t, sandboxd.IsolationEnabled(newMockRuntime(runtime.ModeMetal)))

	// configuration without a SecurityProfileConfig document, as on a cluster
	// upgraded from a Talos version predating workload isolation.
	assert.False(t, sandboxd.IsolationEnabled(newMockRuntime(runtime.ModeMetal, &v1alpha1.Config{ConfigVersion: "v1alpha1"})))

	assert.False(t, sandboxd.IsolationEnabled(newMockRuntime(runtime.ModeMetal, securityProfile(false))))
	assert.True(t, sandboxd.IsolationEnabled(newMockRuntime(runtime.ModeMetal, securityProfile(true))))

	// never inside a container, whatever the configuration says.
	assert.False(t, sandboxd.IsolationEnabled(newMockRuntime(runtime.ModeContainer, securityProfile(true))))
	assert.False(t, sandboxd.IsolationEnabled(newMockRuntime(runtime.ModeMetalAgent, securityProfile(true))))
}

func TestIsolationRequiresService(t *testing.T) {
	t.Parallel()

	// isolation can only ever be on where the service that provides it runs.
	for _, mode := range []runtime.Mode{runtime.ModeMetal, runtime.ModeCloud, runtime.ModeContainer, runtime.ModeMetalAgent} {
		rt := newMockRuntime(mode, securityProfile(true))

		require.False(t, sandboxd.IsolationEnabled(rt) && !sandboxd.ServiceEnabled(rt), "mode %s", mode)
	}
}
