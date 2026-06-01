// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	runtimectrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

func TestKernelModuleStatusSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &KernelModuleStatusSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
		},
	})
}

type KernelModuleStatusSuite struct {
	ctest.DefaultSuite
}

func (suite *KernelModuleStatusSuite) TestParseFromLiveKernel() {
	if hostname, _ := os.Hostname(); hostname == "buildkitsandbox" { //nolint:errcheck
		suite.T().Skip("test not supported under buildkit, as modules are not propagated from the host kernel into the buildkit sandbox")
	}

	suite.Require().NoError(suite.Runtime().RegisterController(&runtimectrl.KernelModuleStatusController{}))

	ctest.AssertNotEmpty[*runtime.LoadedKernelModule](suite)
	ctest.AssertNotEmpty[*runtime.KernelModuleStatus](suite)
}

func (suite *KernelModuleStatusSuite) TestParseMock() {
	suite.Require().NoError(suite.Runtime().RegisterController(
		&runtimectrl.KernelModuleStatusController{
			ProcModulesPath:        "testdata/kernel-modules/proc-modules.txt",
			ModulesBuiltinFilePath: "testdata/kernel-modules/modules-builtin.txt",
		},
	))

	ctest.AssertResources(suite, []string{
		"aes_x86_64",
		"aes_generic",
		"loopback",
		"tcp_cubic",
		"ext4",
	}, func(module *runtime.KernelModuleStatus, asrt *assert.Assertions) {
		asrt.Equal(runtime.KernelModuleTypeBuiltin, module.TypedSpec().Type)
		asrt.Equal(runtime.KernelModuleStateBuiltin, module.TypedSpec().State)
	})

	malformedEntryNames := []string{"firstmalformed", "secondmalformed"}
	ctest.AssertNoResources[*runtime.LoadedKernelModule](suite, malformedEntryNames)
	ctest.AssertNoResources[*runtime.KernelModuleStatus](suite, malformedEntryNames)
}

func (suite *KernelModuleStatusSuite) TestLoadedKernelModuleFields() {
	suite.Require().NoError(suite.Runtime().RegisterController(
		&runtimectrl.KernelModuleStatusController{
			ProcModulesPath:        "testdata/kernel-modules/proc-modules.txt",
			ModulesBuiltinFilePath: "testdata/kernel-modules/modules-builtin.txt",
		},
	))

	ctest.AssertResource(suite, "cpuid", func(res *runtime.LoadedKernelModule, asrt *assert.Assertions) {
		asrt.Equal(12288, res.TypedSpec().Size)
		asrt.Equal(0, res.TypedSpec().ReferenceCount)
		asrt.Equal([]string{}, res.TypedSpec().Dependencies)
		asrt.Equal("Live", res.TypedSpec().State)
		asrt.Equal("0x0000000000000000", res.TypedSpec().Address)
	})

	ctest.AssertResource(suite, "curve25519_x86_64", func(res *runtime.LoadedKernelModule, asrt *assert.Assertions) {
		asrt.Equal(36864, res.TypedSpec().Size)
		asrt.Equal(1, res.TypedSpec().ReferenceCount)
		asrt.Equal([]string{"wireguard"}, res.TypedSpec().Dependencies)
	})

	ctest.AssertResource(suite, "libcurve25519_generic", func(res *runtime.LoadedKernelModule, asrt *assert.Assertions) {
		asrt.Equal(45056, res.TypedSpec().Size)
		asrt.Equal(2, res.TypedSpec().ReferenceCount)
		asrt.Equal([]string{"wireguard", "curve25519_x86_64"}, res.TypedSpec().Dependencies)
	})
}

func (suite *KernelModuleStatusSuite) TestKernelModuleStatusFields() {
	suite.Require().NoError(suite.Runtime().RegisterController(
		&runtimectrl.KernelModuleStatusController{
			ProcModulesPath:        "testdata/kernel-modules/proc-modules.txt",
			ModulesBuiltinFilePath: "testdata/kernel-modules/modules-builtin.txt",
		},
	))

	ctest.AssertResource(suite, "wireguard", func(res *runtime.KernelModuleStatus, asrt *assert.Assertions) {
		asrt.Equal(runtime.KernelModuleTypeDynamic, res.TypedSpec().Type)
		asrt.Equal(114688, res.TypedSpec().Size)
		asrt.Equal(0, res.TypedSpec().ReferenceCount)
		asrt.Equal([]string{}, res.TypedSpec().Dependencies)
		asrt.Equal(runtime.KernelModuleStateLive, res.TypedSpec().State)
		asrt.Equal("0x0000000000000000", res.TypedSpec().Address)
	})
}

func (suite *KernelModuleStatusSuite) TestDynamicModuleStateVariants() {
	suite.Require().NoError(suite.Runtime().RegisterController(
		&runtimectrl.KernelModuleStatusController{
			ProcModulesPath:        "testdata/kernel-modules/proc-modules.txt",
			ModulesBuiltinFilePath: "testdata/kernel-modules/modules-builtin.txt",
		},
	))

	ctest.AssertResource(suite, "modloading", func(res *runtime.KernelModuleStatus, asrt *assert.Assertions) {
		asrt.Equal(runtime.KernelModuleTypeDynamic, res.TypedSpec().Type)
		asrt.Equal(runtime.KernelModuleStateLoading, res.TypedSpec().State)
	})

	ctest.AssertResource(suite, "modunloading", func(res *runtime.KernelModuleStatus, asrt *assert.Assertions) {
		asrt.Equal(runtime.KernelModuleTypeDynamic, res.TypedSpec().Type)
		asrt.Equal(runtime.KernelModuleStateUnloading, res.TypedSpec().State)
	})

	ctest.AssertResource(suite, "wireguard", func(res *runtime.KernelModuleStatus, asrt *assert.Assertions) {
		asrt.Equal(runtime.KernelModuleTypeDynamic, res.TypedSpec().Type)
		asrt.Equal(runtime.KernelModuleStateLive, res.TypedSpec().State)
	})
}

func (suite *KernelModuleStatusSuite) TestStaleResourceCleanup() {
	tmpDir := suite.T().TempDir()
	procModulesPath := filepath.Join(tmpDir, "proc-modules.txt")

	suite.Require().NoError(os.WriteFile(procModulesPath, []byte(
		"cpuid 12288 0 - Live 0x0000000000000000\n"+
			"wireguard 114688 0 - Live 0x0000000000000000\n",
	), 0o644))

	reconcileCh := make(chan struct{}, 1)

	suite.Require().NoError(suite.Runtime().RegisterController(
		&runtimectrl.KernelModuleStatusController{
			ProcModulesPath:        procModulesPath,
			ModulesBuiltinFilePath: "testdata/kernel-modules/modules-builtin.txt",
			ReconcileCh:            reconcileCh,
		},
	))

	ctest.AssertResource(suite, "wireguard", func(res *runtime.LoadedKernelModule, asrt *assert.Assertions) {
		asrt.Equal(114688, res.TypedSpec().Size)
	})
	ctest.AssertResource(suite, "cpuid", func(res *runtime.LoadedKernelModule, asrt *assert.Assertions) {
		asrt.Equal(12288, res.TypedSpec().Size)
	})

	suite.Require().NoError(os.WriteFile(procModulesPath, []byte(
		"cpuid 12288 0 - Live 0x0000000000000000\n",
	), 0o644))

	reconcileCh <- struct{}{}

	ctest.AssertNoResource[*runtime.LoadedKernelModule](suite, "wireguard")
	ctest.AssertNoResource[*runtime.KernelModuleStatus](suite, "wireguard")

	ctest.AssertResource(suite, "cpuid", func(res *runtime.LoadedKernelModule, asrt *assert.Assertions) {
		asrt.Equal(12288, res.TypedSpec().Size)
	})
}

func TestParseLoadedDynamicModules(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		input    string
		expected []runtimectrl.DynamicModule
	}{
		{
			name:     "empty",
			input:    "",
			expected: nil,
		},
		{
			name:  "single module",
			input: `module1 12345 0 - Live 0x00000000`,
			expected: []runtimectrl.DynamicModule{
				{Name: "module1", Size: 12345, ReferenceCount: 0, Dependencies: []string{}, State: "Live", Address: "0x00000000"},
			},
		},
		{
			name: "multiple modules",
			input: `module1 12345 0 - Live 0x00000000
module2 67890 1 module1 Live 0x00000001
module3 54321 2 module1,module2 Live 0x00000002`,
			expected: []runtimectrl.DynamicModule{
				{Name: "module1", Size: 12345, ReferenceCount: 0, Dependencies: []string{}, State: "Live", Address: "0x00000000"},
				{Name: "module2", Size: 67890, ReferenceCount: 1, Dependencies: []string{"module1"}, State: "Live", Address: "0x00000001"},
				{Name: "module3", Size: 54321, ReferenceCount: 2, Dependencies: []string{"module1", "module2"}, State: "Live", Address: "0x00000002"},
			},
		},
		{
			name: "malformed lines",
			input: `module1 12345 0 - Live 0x00000000
module2 67890 1 module1 Live 0x00000001
module3 54321 2 module1,module2 Live 0x00000002
invalid_line
module4 11111 0 - Live 0x00000003`,
			expected: []runtimectrl.DynamicModule{
				{Name: "module1", Size: 12345, ReferenceCount: 0, Dependencies: []string{}, State: "Live", Address: "0x00000000"},
				{Name: "module2", Size: 67890, ReferenceCount: 1, Dependencies: []string{"module1"}, State: "Live", Address: "0x00000001"},
				{Name: "module3", Size: 54321, ReferenceCount: 2, Dependencies: []string{"module1", "module2"}, State: "Live", Address: "0x00000002"},
				{Name: "module4", Size: 11111, ReferenceCount: 0, Dependencies: []string{}, State: "Live", Address: "0x00000003"},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			modules, err := runtimectrl.ParseDynamicModules(strings.NewReader(tc.input))
			require.NoError(t, err)
			require.Equal(t, tc.expected, modules)
		})
	}
}

func TestParseBuiltinModuleNames(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty",
			input:    "",
			expected: nil,
		},
		{
			name:     "single module",
			input:    "kernel/arch/x86/crypto/aes-x86_64.ko\n",
			expected: []string{"aes_x86_64"},
		},
		{
			name: "multiple modules with hyphen normalization",
			input: `kernel/crypto/aes_generic.ko
kernel/arch/x86/crypto/aes-x86_64.ko
kernel/drivers/net/loopback.ko
`,
			expected: []string{"aes_generic", "aes_x86_64", "loopback"},
		},
		{
			name: "blank lines are skipped",
			input: `kernel/crypto/aes_generic.ko

kernel/drivers/net/loopback.ko
`,
			expected: []string{"aes_generic", "loopback"},
		},
		{
			name: "malformed lines are skipped",
			input: `kernel/crypto/aes_generic.ko
.ko
kernel/dir/.
kernel/drivers/net/loopback.ko
`,
			expected: []string{"aes_generic", "loopback"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			names, err := runtimectrl.ParseBuiltinModuleNames(strings.NewReader(tc.input))
			require.NoError(t, err)
			require.Equal(t, tc.expected, names)
		})
	}
}
