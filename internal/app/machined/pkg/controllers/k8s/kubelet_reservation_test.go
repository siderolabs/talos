// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s_test

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
	cliflag "k8s.io/component-base/cli/flag"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	k8sctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/k8s"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

func TestKubeletReservationConsumer(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := &ctest.DefaultSuite{}
		s.SetT(t)
		s.Timeout = time.Hour

		s.SetupTest()
		defer s.TearDownTest()

		require.NoError(t, s.Runtime().RegisterController(&k8sctrl.KubeletSpecController{}))

		cfg := k8s.NewKubeletConfig(k8s.NamespaceName, k8s.KubeletID)
		cfg.TypedSpec().Image = "kubelet:v1.38.0-alpha.1"
		cfg.TypedSpec().ExtraArgs = map[string]k8s.ArgValues{"node-ip": {Values: []string{"192.0.2.1"}}}
		s.Create(cfg)
		synctest.Wait()

		_, missing := safe.StateGetByID[*k8s.KubeletSpec](s.Ctx(), s.State(), k8s.KubeletID)
		require.True(t, state.IsNotFoundError(missing), "ordinary rendering prerequisites still apply")

		name := k8s.NewNodename(k8s.NamespaceName, k8s.NodenameID)
		name.TypedSpec().Nodename = "worker"
		s.Create(name)

		typ := config.NewMachineType()
		typ.SetMachineType(machine.TypeWorker)
		s.Create(typ)

		policy := runtime.NewCPUPartitionSpec()
		policy.TypedSpec().Enabled = true
		policy.TypedSpec().Roots = map[string]string{"kubepods": "2-7"}
		s.Create(policy)
		synctest.Wait()

		read := func() *k8s.KubeletSpec {
			spec, err := safe.StateGetByID[*k8s.KubeletSpec](s.Ctx(), s.State(), k8s.KubeletID)
			require.NoError(t, err)

			return spec
		}
		initial := read()
		require.Empty(t, initial.TypedSpec().Config["reservedSystemCPUs"], "enabled desired policy must not wait for its unregistered producer")
		require.Empty(t, initial.TypedSpec().Config["cpuManagerPolicy"])

		res := k8s.NewKubeletCPUReservation()
		s.Create(res)
		synctest.Wait()
		require.Equal(t, initial.Metadata().Version(), read().Metadata().Version())
		require.Equal(t, initial.TypedSpec().Args, read().TypedSpec().Args)
		require.NoError(t, safe.StateModify(s.Ctx(), s.State(), cfg, func(cfg *k8s.KubeletConfig) error {
			cfg.TypedSpec().ExtraConfig = map[string]any{"reservedSystemCPUs": "7", "cpuManagerPolicy": "none"}

			return nil
		}))
		synctest.Wait()
		require.Equal(t, "7", read().TypedSpec().Config["reservedSystemCPUs"], "unmanaged resource preserves legacy overrides")

		require.NoError(t, safe.StateModify(s.Ctx(), s.State(), cfg, func(cfg *k8s.KubeletConfig) error {
			cfg.TypedSpec().ExtraConfig = map[string]any{"cpuManagerPolicyOptions": map[string]any{"full-pcpus-only": "true", "prefer-align-cpus-by-uncorecache": "true"}}
			cfg.TypedSpec().ExtraArgs["cpu_manager-policy_options"] = k8s.ArgValues{Values: []string{"full-pcpus-only=true", "full-pcpus-only=false"}}

			return nil
		}))
		require.NoError(t, safe.StateModify(s.Ctx(), s.State(), res, func(res *k8s.KubeletCPUReservation) error {
			*res.TypedSpec() = k8s.KubeletCPUReservationSpec{Managed: true, ReservedCPUs: "0-1"}

			return nil
		}))
		synctest.Wait()

		managed := read()
		require.Equal(t, "0-1", managed.TypedSpec().Config["reservedSystemCPUs"])
		require.Equal(t, "static", managed.TypedSpec().Config["cpuManagerPolicy"])
		require.Equal(t, map[string]any{"full-pcpus-only": "false", "strict-cpu-reservation": "true"}, managed.TypedSpec().Config["cpuManagerPolicyOptions"])

		flags := pflag.NewFlagSet("kubelet", pflag.ContinueOnError)
		flags.ParseErrorsAllowlist.UnknownFlags = true
		options := map[string]string{"file-only": "true"}
		flags.Var(cliflag.NewMapStringStringNoSplit(&options), "cpu-manager-policy-options", "")
		reserved := flags.String("reserved-cpus", "", "")
		cpuPolicy := flags.String("cpu-manager-policy", "none", "")
		require.NoError(t, flags.Parse(managed.TypedSpec().Args))
		require.Equal(t, "0-1", *reserved)
		require.Equal(t, "static", *cpuPolicy)
		require.Equal(t, map[string]string{"full-pcpus-only": "false", "strict-cpu-reservation": "true"}, options)

		require.NoError(t, safe.StateModify(s.Ctx(), s.State(), res, func(res *k8s.KubeletCPUReservation) error {
			res.Metadata().Annotations().Set("test-version", "next")

			return nil
		}))
		synctest.Wait()

		next := read()
		require.Equal(t, managed.TypedSpec(), next.TypedSpec())
		require.Equal(t, managed.Metadata().Version(), next.Metadata().Version(), "unchanged effective configuration must not restart kubelet")

		s.Destroy(policy)
		synctest.Wait()
		require.Equal(t, "0-1", read().TypedSpec().Config["reservedSystemCPUs"], "desired policy removal cannot relinquish the staged reservation")

		require.NoError(t, safe.StateModify(s.Ctx(), s.State(), cfg, func(cfg *k8s.KubeletConfig) error {
			cfg.TypedSpec().ExtraConfig = map[string]any{"reservedSystemCPUs": "7", "cpuManagerPolicy": "none"}

			return nil
		}))
		require.NoError(t, safe.StateModify(s.Ctx(), s.State(), res, func(res *k8s.KubeletCPUReservation) error {
			*res.TypedSpec() = k8s.KubeletCPUReservationSpec{}

			return nil
		}))
		synctest.Wait()
		require.Equal(t, "7", read().TypedSpec().Config["reservedSystemCPUs"], "producer release restores legacy reservation")
		require.Equal(t, "none", read().TypedSpec().Config["cpuManagerPolicy"])
		require.Empty(t, read().TypedSpec().Config["cpuManagerPolicyOptions"])
		s.Destroy(res)
		synctest.Wait()
		require.Equal(t, "7", read().TypedSpec().Config["reservedSystemCPUs"])
		require.Empty(t, read().Metadata().Annotations().Raw())
	})
}
