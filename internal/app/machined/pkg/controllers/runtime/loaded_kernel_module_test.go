// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	runtimectrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime"
)

func TestLoadedKernelModuleSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &LoadedKernelModuleSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&runtimectrl.LoadedKernelModuleController{}))
			},
		},
	})
}

type LoadedKernelModuleSuite struct {
	ctest.DefaultSuite
}

func (suite *LoadedKernelModuleSuite) TestParseModules() {
	for _, tc := range []struct {
		name     string
		input    string
		expected []runtimectrl.Module
	}{
		{
			name:     "empty",
			input:    "",
			expected: nil,
		},
		{
			name:  "single module",
			input: `module1 12345 0 - Live 0x00000000`,
			expected: []runtimectrl.Module{
				{Name: "module1", Size: 12345, Instances: 0, Dependencies: []string{}, State: "Live", Address: "0x00000000"},
			},
		},
		{
			name: "multiple modules",
			input: `module1 12345 0 - Live 0x00000000
module2 67890 1 module1 Live 0x00000001
module3 54321 2 module1,module2 Live 0x00000002`,
			expected: []runtimectrl.Module{
				{Name: "module1", Size: 12345, Instances: 0, Dependencies: []string{}, State: "Live", Address: "0x00000000"},
				{Name: "module2", Size: 67890, Instances: 1, Dependencies: []string{"module1"}, State: "Live", Address: "0x00000001"},
				{Name: "module3", Size: 54321, Instances: 2, Dependencies: []string{"module1", "module2"}, State: "Live", Address: "0x00000002"},
			},
		},
		{
			name: "malformed lines",
			input: `module1 12345 0 - Live 0x00000000
module2 67890 1 module1 Live 0x00000001
module3 54321 2 module1,module2 Live 0x00000002
invalid_line
module4 11111 0 - Live 0x00000003`,
			expected: []runtimectrl.Module{
				{Name: "module1", Size: 12345, Instances: 0, Dependencies: []string{}, State: "Live", Address: "0x00000000"},
				{Name: "module2", Size: 67890, Instances: 1, Dependencies: []string{"module1"}, State: "Live", Address: "0x00000001"},
				{Name: "module3", Size: 54321, Instances: 2, Dependencies: []string{"module1", "module2"}, State: "Live", Address: "0x00000002"},
				{Name: "module4", Size: 11111, Instances: 0, Dependencies: []string{}, State: "Live", Address: "0x00000003"},
			},
		},
	} {
		suite.Run(tc.name, func() {
			modules, err := runtimectrl.ParseModules(strings.NewReader(tc.input))
			suite.Require().NoError(err)
			suite.Require().Equal(tc.expected, modules)
		})
	}
}
