// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package files_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	filesctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/files"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/files"
)

func (suite *UdevRuleSuite) TestUdevRule() {
	cfg := config.NewMachineConfig(&v1alpha1.Config{
		MachineConfig: &v1alpha1.MachineConfig{
			MachineUdev: &v1alpha1.UdevConfig{
				UdevRules: []string{
					`SUBSYSTEM=="block", KERNEL=="vdb*", SYMLINK+="myhdda%n"`,
					`SUBSYSTEM=="block", KERNEL=="vdb*", SYMLINK+="myhddb%n"`,
				},
			},
		},
	})

	suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))

	for _, tt := range []struct {
		// id is the first 8 characters of the base36 encoded sha256 hash of the rule
		id       string
		expected string
	}{
		{
			id:       "168vxb2k",
			expected: `SUBSYSTEM=="block", KERNEL=="vdb*", SYMLINK+="myhdda%n"`,
		},
		{
			id:       "3aaseddz",
			expected: `SUBSYSTEM=="block", KERNEL=="vdb*", SYMLINK+="myhddb%n"`,
		},
	} {
		suite.AssertWithin(3*time.Second, 100*time.Millisecond, func() error {
			udevRule, err := ctest.Get[*files.UdevRule](suite, files.NewUdevRule(tt.id).Metadata())
			if err != nil {
				if state.IsNotFoundError(err) {
					return retry.ExpectedError(err)
				}

				return err
			}

			spec := udevRule.TypedSpec()

			suite.Assert().Equal(tt.expected, spec.Rule)

			return nil
		})
	}

	// test deletion
	cfg = config.NewMachineConfig(&v1alpha1.Config{
		MachineConfig: &v1alpha1.MachineConfig{
			MachineUdev: &v1alpha1.UdevConfig{
				UdevRules: []string{
					`SUBSYSTEM=="block", KERNEL=="vdb*", SYMLINK+="myhdda%n"`,
				},
			},
		},
	})

	cfg.Metadata().SetVersion(cfg.Metadata().Version().Next())
	suite.Require().NoError(suite.State().Update(suite.Ctx(), cfg))

	suite.AssertWithin(1*time.Second, 100*time.Millisecond, func() error {
		_, err := ctest.Get[*files.UdevRule](suite, files.NewUdevRule("3aaseddz").Metadata())
		if err != nil {
			if state.IsNotFoundError(err) {
				return nil
			}

			return err
		}

		return retry.ExpectedError(fmt.Errorf("udev rule with id 3aaseddz should not exist"))
	})
}

func TestUdevRuleSuite(t *testing.T) {
	suite.Run(t, &UdevRuleSuite{
		DefaultSuite: ctest.DefaultSuite{
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&filesctrl.UdevRuleController{}))
			},
		},
	})
}

type UdevRuleSuite struct {
	ctest.DefaultSuite
}
