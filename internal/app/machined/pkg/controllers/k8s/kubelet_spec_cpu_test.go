// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s_test

import (
	"net/netip"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	k8sctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/k8s"
	v1alpha1runtime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

type KubeletSpecCPUSuite struct {
	ctest.DefaultSuite

	logs *observer.ObservedLogs
}

func TestKubeletSpecCPUSuite(t *testing.T) {
	t.Parallel()

	logsCore, logs := observer.New(zapcore.ErrorLevel)

	s := &KubeletSpecCPUSuite{logs: logs}

	s.DefaultSuite = ctest.DefaultSuite{
		Timeout: 10 * time.Second,
		Logger:  zap.New(logsCore),
		AfterSetup: func(ds *ctest.DefaultSuite) {
			logs.TakeAll()

			ds.Require().NoError(ds.Runtime().RegisterController(&k8sctrl.KubeletSpecController{
				V1Alpha1Mode: v1alpha1runtime.ModeMetal,
			}))
		},
	}

	suite.Run(t, s)
}

func (suite *KubeletSpecCPUSuite) createInputs(extraConfig map[string]any) {
	cfg := k8s.NewKubeletConfig(k8s.NamespaceName, k8s.KubeletID)
	cfg.TypedSpec().Image = "kubelet:v1.38.0-alpha.1"
	cfg.TypedSpec().ClusterDNS = []string{"10.96.0.10"}
	cfg.TypedSpec().ClusterDomain = "cluster.local"
	cfg.TypedSpec().ExtraConfig = extraConfig

	suite.Create(cfg)

	nodeIP := k8s.NewNodeIP(k8s.NamespaceName, k8s.KubeletID)
	nodeIP.TypedSpec().Addresses = []netip.Addr{netip.MustParseAddr("172.20.0.2")}
	suite.Create(nodeIP)

	nodename := k8s.NewNodename(k8s.NamespaceName, k8s.NodenameID)
	nodename.TypedSpec().Nodename = "example.com"
	suite.Create(nodename)

	machineType := config.NewMachineType()
	machineType.SetMachineType(machine.TypeWorker)
	suite.Create(machineType)
}

// cpuPartitionReady publishes the status of a boot without a CPU partition policy, which the kubelet waits for.
func cpuPartitionReady(s ctest.Suite) {
	status := runtime.NewCPUPartitionStatus()
	status.TypedSpec().Ready = true
	s.Require().NoError(s.State().Create(s.Ctx(), status))
}

func (suite *KubeletSpecCPUSuite) status(spec runtime.CPUPartitionStatusSpec) {
	status := runtime.NewCPUPartitionStatus()
	*status.TypedSpec() = spec
	suite.Create(status)
}

func (suite *KubeletSpecCPUSuite) assertCPUManager(policy, reserved any) {
	ctest.AssertResource(suite, k8s.KubeletID, func(r *k8s.KubeletSpec, asrt *assert.Assertions) {
		asrt.Equal(policy, r.TypedSpec().Config["cpuManagerPolicy"])
		asrt.Equal(reserved, r.TypedSpec().Config["reservedSystemCPUs"])
	})
}

func (suite *KubeletSpecCPUSuite) assertHeld() {
	ctx, st := suite.Ctx(), suite.State()

	suite.Require().Never(func() bool {
		_, err := safe.StateGetByID[*k8s.KubeletSpec](ctx, st, k8s.KubeletID)

		return err == nil
	}, 200*time.Millisecond, 10*time.Millisecond)
}

func (suite *KubeletSpecCPUSuite) assertRejected(expected string) {
	suite.AssertWithin(5*time.Second, 10*time.Millisecond, func() error {
		for _, entry := range suite.logs.FilterMessage("controller failed").All() {
			if err, ok := entry.ContextMap()["error"].(string); ok && err == "error creating kubelet configuration: "+expected {
				return nil
			}
		}

		return retry.ExpectedErrorf("conflict not logged yet")
	})

	ctest.AssertNoResource[*k8s.KubeletSpec](suite, k8s.KubeletID)
}

func (suite *KubeletSpecCPUSuite) TestWaitsForStatus() {
	suite.createInputs(nil)
	suite.assertHeld()

	suite.status(runtime.CPUPartitionStatusSpec{})
	suite.assertHeld()
}

func (suite *KubeletSpecCPUSuite) TestStaticPolicyWhenKubepodsBounded() {
	suite.createInputs(nil)
	suite.status(runtime.CPUPartitionStatusSpec{Ready: true, KubeletReservedCPUs: "0-1"})

	suite.assertCPUManager("static", "0-1")
}

func (suite *KubeletSpecCPUSuite) TestUnchangedWithoutKubepodsBound() {
	suite.createInputs(map[string]any{"cpuManagerPolicy": "none", "reservedSystemCPUs": "0"})
	suite.status(runtime.CPUPartitionStatusSpec{Ready: true})

	suite.assertCPUManager("none", "0")
}

func (suite *KubeletSpecCPUSuite) TestUserReservationConflict() {
	suite.createInputs(map[string]any{"reservedSystemCPUs": "0"})
	suite.status(runtime.CPUPartitionStatusSpec{Ready: true, KubeletReservedCPUs: "0-1"})

	suite.assertRejected(`"reservedSystemCPUs" is derived from the CPU partition kubepods root and can't be set`)
}

func (suite *KubeletSpecCPUSuite) TestUserCPUManagerPolicyConflict() {
	suite.createInputs(map[string]any{"cpuManagerPolicy": "none"})
	suite.status(runtime.CPUPartitionStatusSpec{Ready: true, KubeletReservedCPUs: "0-1"})

	suite.assertRejected(`"cpuManagerPolicy" none conflicts with the CPU partition kubepods root, which requires "static"`)
}
