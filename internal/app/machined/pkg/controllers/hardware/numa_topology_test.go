// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hardware_test

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	hardwarectrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/hardware"
	runtimetalos "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
)

type NUMATopologySuite struct {
	ctest.DefaultSuite
}

func writeNUMAFile(t *testing.T, root, path, data string) {
	t.Helper()

	path = filepath.Join(root, path)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	// Publish complete records: a controller retry can read concurrently with a repair.
	require.NoError(t, os.WriteFile(path+".tmp", []byte(data), 0o644))
	require.NoError(t, os.Rename(path+".tmp", path))
}

func numaFixture(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	for path, data := range map[string]string{
		"devices/system/cpu/present":        "0-3,8\n",
		"devices/system/cpu/online":         "0-2,8\n",
		"devices/system/node/online":        "0,2,7\n",
		"devices/system/node/node0/cpulist": "0,2\n",
		"devices/system/node/node0/meminfo": "Node 0 MemTotal:       1024 kB\nNode 0 MemFree: 16 kB\n",
		"devices/system/node/node2/cpulist": "1,3,8\n",
		"devices/system/node/node2/meminfo": "Node 2 MemTotal:       0 kB\n",
		"devices/system/node/node7/cpulist": "\n",
		"devices/system/node/node7/meminfo": "Node 7 MemTotal:       2048 kB\n",
	} {
		writeNUMAFile(t, root, path, data)
	}

	for cpu, node := range map[int]int{0: 0, 1: 2, 2: 0, 3: 2, 8: 2} {
		base := "devices/system/node/node" + strconv.Itoa(node) + "/cpu" + strconv.Itoa(cpu) + "/topology/"
		writeNUMAFile(t, root, base+"core_id", strconv.Itoa(cpu/2))
		writeNUMAFile(t, root, base+"physical_package_id", "0")
	}

	return root
}

func triggerNUMAReconcile(ctx context.Context, reconcileCh chan<- struct{}) error {
	select {
	case reconcileCh <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func TestNUMAReconcileCancellation(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		reconcileCh := make(chan struct{}, 1)
		require.NoError(t, triggerNUMAReconcile(ctx, reconcileCh))

		done := make(chan error, 1)

		go func() {
			done <- triggerNUMAReconcile(ctx, reconcileCh)
		}()

		synctest.Wait()

		select {
		case err := <-done:
			t.Fatalf("trigger returned before delivery or cancellation: %v", err)
		default:
		}

		cancel()
		require.ErrorIs(t, <-done, context.Canceled)
		require.Len(t, reconcileCh, 1)
	})
}

func (suite *NUMATopologySuite) register(root string) chan struct{} {
	reconcileCh := make(chan struct{}, 1)
	suite.Require().NoError(suite.Runtime().RegisterController(&hardwarectrl.NUMATopologyController{
		SysfsPath: root, ProcfsPath: root, ReconcileCh: reconcileCh,
	}))

	return reconcileCh
}

func (suite *NUMATopologySuite) TestInventoryAndRefresh() {
	root := numaFixture(suite.T())
	reconcileCh := suite.register(root)
	expected := hardware.NUMATopologySpec{
		PresentCPUs: []uint32{0, 1, 2, 3, 8}, OnlineCPUs: []uint32{0, 1, 2, 8},
		Nodes: []hardware.NUMANodeSpec{
			{ID: 0, CPUs: []uint32{0, 2}, MemoryTotalBytes: 1024 * 1024},
			{ID: 2, CPUs: []uint32{1, 3, 8}, MemoryTotalBytes: 0},
			{ID: 7, CPUs: []uint32{}, MemoryTotalBytes: 2048 * 1024},
		},
	}

	ctest.AssertResource(suite, hardware.NUMATopologyID, func(r *hardware.NUMATopology, asrt *assert.Assertions) { asrt.Equal(expected, *r.TypedSpec()) })
	writeNUMAFile(suite.T(), root, "devices/system/cpu/online", "0-3,8\n")

	suite.Require().NoError(triggerNUMAReconcile(suite.Ctx(), reconcileCh))

	expected.OnlineCPUs = []uint32{0, 1, 2, 3, 8}

	ctest.AssertResource(suite, hardware.NUMATopologyID, func(r *hardware.NUMATopology, asrt *assert.Assertions) { asrt.Equal(expected, *r.TypedSpec()) })
}

func (suite *NUMATopologySuite) TestWithdrawAndRecover() {
	root := numaFixture(suite.T())
	reconcileCh := suite.register(root)
	ctest.AssertResource(suite, hardware.NUMATopologyID, func(r *hardware.NUMATopology, asrt *assert.Assertions) { asrt.Len(r.TypedSpec().Nodes, 3) })
	suite.Require().NoError(os.Remove(filepath.Join(root, "devices/system/node/node2/meminfo")))

	suite.Require().NoError(triggerNUMAReconcile(suite.Ctx(), reconcileCh))

	ctest.AssertNoResource[*hardware.NUMATopology](suite, hardware.NUMATopologyID)
	writeNUMAFile(suite.T(), root, "devices/system/node/node2/meminfo", "Node 2 MemTotal: 4096 kB\n")
	// No trigger: controller restart backoff must recover a failed snapshot by itself.
	ctest.AssertResource(suite, hardware.NUMATopologyID, func(r *hardware.NUMATopology, asrt *assert.Assertions) {
		asrt.Equal(uint64(4096*1024), r.TypedSpec().Nodes[1].MemoryTotalBytes)
	})
}

func (suite *NUMATopologySuite) TestFallbackAndMissingNodeInterface() {
	root := suite.T().TempDir()
	writeNUMAFile(suite.T(), root, "devices/system/cpu/present", "0-1,9")
	writeNUMAFile(suite.T(), root, "devices/system/cpu/online", "0,9")
	writeNUMAFile(suite.T(), root, "meminfo", "MemTotal: 1234 kB\n")
	reconcileCh := suite.register(root)
	ctest.AssertResource(suite, hardware.NUMATopologyID, func(r *hardware.NUMATopology, asrt *assert.Assertions) {
		asrt.Equal([]hardware.NUMANodeSpec{{ID: 0, CPUs: []uint32{0, 1, 9}, MemoryTotalBytes: 1234 * 1024}}, r.TypedSpec().Nodes)
	})
	// A present interface with a missing online file is not a non-NUMA machine.
	suite.Require().NoError(os.MkdirAll(filepath.Join(root, "devices/system/node"), 0o755))

	suite.Require().NoError(triggerNUMAReconcile(suite.Ctx(), reconcileCh))

	ctest.AssertNoResource[*hardware.NUMATopology](suite, hardware.NUMATopologyID)
}

// Each malformed record gets its own runtime and deadline, without queued triggers
// or restart backoff carried over from a previous withdrawal/recovery cycle.
func (suite *NUMATopologySuite) TestMalformedMemoryMissingTotal() {
	suite.assertMalformedMemory("Node 2 MemFree: 12 kB\n")
}

func (suite *NUMATopologySuite) TestMalformedMemoryInvalidTotal() {
	suite.assertMalformedMemory("Node 2 MemTotal: broken kB\n")
}

func (suite *NUMATopologySuite) TestMalformedMemoryInvalidUnit() {
	suite.assertMalformedMemory("Node 2 MemTotal: 12 MB\n")
}

func (suite *NUMATopologySuite) TestMalformedMemoryOverflow() {
	suite.assertMalformedMemory("Node 2 MemTotal: 18446744073709551616 kB\n")
}

func (suite *NUMATopologySuite) assertMalformedMemory(record string) {
	suite.T().Helper()

	root := numaFixture(suite.T())
	reconcileCh := suite.register(root)
	ctest.AssertResource(suite, hardware.NUMATopologyID, func(r *hardware.NUMATopology, asrt *assert.Assertions) { asrt.Len(r.TypedSpec().Nodes, 3) })
	writeNUMAFile(suite.T(), root, "devices/system/node/node2/meminfo", record)

	suite.Require().NoError(triggerNUMAReconcile(suite.Ctx(), reconcileCh))

	ctest.AssertNoResource[*hardware.NUMATopology](suite, hardware.NUMATopologyID)
	writeNUMAFile(suite.T(), root, "devices/system/node/node2/meminfo", "Node 2 MemTotal: 4096 kB\n")
	// No trigger: each malformed snapshot must recover through controller restart.
	ctest.AssertResource(suite, hardware.NUMATopologyID, func(r *hardware.NUMATopology, asrt *assert.Assertions) {
		asrt.Len(r.TypedSpec().Nodes, 3)
		asrt.Equal(uint64(4096*1024), r.TypedSpec().Nodes[1].MemoryTotalBytes)
	})
}

func (suite *NUMATopologySuite) TestInconsistentMembership() {
	root := numaFixture(suite.T())
	reconcileCh := suite.register(root)
	ctest.AssertResource(suite, hardware.NUMATopologyID, func(r *hardware.NUMATopology, asrt *assert.Assertions) { asrt.Len(r.TypedSpec().Nodes, 3) })
	writeNUMAFile(suite.T(), root, "devices/system/cpu/online", "0-3,9\n")

	suite.Require().NoError(triggerNUMAReconcile(suite.Ctx(), reconcileCh))

	ctest.AssertNoResource[*hardware.NUMATopology](suite, hardware.NUMATopologyID)
	writeNUMAFile(suite.T(), root, "devices/system/cpu/online", "0-3,8\n")
	ctest.AssertResource(suite, hardware.NUMATopologyID, func(r *hardware.NUMATopology, asrt *assert.Assertions) { asrt.Len(r.TypedSpec().Nodes, 3) })
}

func (suite *NUMATopologySuite) TestIncompleteCPUDiscovery() {
	root := numaFixture(suite.T())
	reconcileCh := suite.register(root)
	ctest.AssertResource(suite, hardware.NUMATopologyID, func(r *hardware.NUMATopology, asrt *assert.Assertions) { asrt.Len(r.TypedSpec().Nodes, 3) })
	suite.Require().NoError(os.Remove(filepath.Join(root, "devices/system/node/node2/cpu8/topology/core_id")))

	suite.Require().NoError(triggerNUMAReconcile(suite.Ctx(), reconcileCh))

	ctest.AssertNoResource[*hardware.NUMATopology](suite, hardware.NUMATopologyID)
	writeNUMAFile(suite.T(), root, "devices/system/node/node2/cpu8/topology/core_id", "4")
	ctest.AssertResource(suite, hardware.NUMATopologyID, func(r *hardware.NUMATopology, asrt *assert.Assertions) {
		asrt.Equal([]uint32{1, 3, 8}, r.TypedSpec().Nodes[1].CPUs)
	})
}

func TestNUMATopologySuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, &NUMATopologySuite{Timeout: 10 * time.Second})
}

func TestNUMATopologyContainer(t *testing.T) {
	t.Parallel()

	ctrl := &hardwarectrl.NUMATopologyController{V1Alpha1Mode: runtimetalos.ModeContainer, SysfsPath: "/nonexistent", ProcfsPath: "/nonexistent"}
	require.NoError(t, ctrl.Run(t.Context(), nil, zaptest.NewLogger(t)))
}
