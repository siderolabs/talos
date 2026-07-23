// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package files_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/files"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	runtimeconfig "github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
	configres "github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

type UdevRulesSuite struct {
	ctest.DefaultSuite

	mu        sync.Mutex
	rulesPath string
	commands  []string
}

func TestUdevRulesSuite(t *testing.T) {
	t.Parallel()

	if os.Geteuid() != 0 {
		t.Skip("requires root")
	}

	suite.Run(t, new(UdevRulesSuite))
}

func (suite *UdevRulesSuite) SetupTest() {
	suite.mu = sync.Mutex{}
	suite.rulesPath = filepath.Join(suite.T().TempDir(), "99-talos.rules")
	suite.commands = nil

	suite.Require().NoError(os.WriteFile(suite.rulesPath, nil, 0o644))

	suite.DefaultSuite.AfterSetup = func(s *ctest.DefaultSuite) {
		s.Require().NoError(s.Runtime().RegisterController(&files.UdevRulesController{
			UdevRulesPath: suite.rulesPath,
			CommandRunner: func(_ context.Context, name string, args []string) (string, error) {
				suite.mu.Lock()
				defer suite.mu.Unlock()

				suite.commands = append(suite.commands, name+" "+strings.Join(args, " "))

				return "", nil
			},
		}))
	}

	suite.DefaultSuite.SetupTest()
}

func (suite *UdevRulesSuite) TestRulesConfigWritesReloadsAndRemovesRules() {
	suite.createUdevdService(true)
	suite.createMachineConfig([]string{"first", "second\ncontinued"})

	suite.assertRulesFile("first\nsecond\\\ncontinued\n")
	suite.assertCommands([]string{
		"/sbin/udevadm control --reload",
		"/sbin/udevadm trigger --type=devices --action=add",
		"/sbin/udevadm trigger --type=subsystems --action=add",
		"/sbin/udevadm settle --timeout=50",
	})

	ctest.UpdateWithConflicts(suite, configres.NewMachineConfig(suite.machineConfig([]string{"updated"})), func(cfg *configres.MachineConfig) error {
		udevConfig := cfg.Config().UdevRulesConfig().(*runtimeconfig.UdevRulesConfigV1Alpha1)
		udevConfig.UdevRules = []string{"updated"}

		return nil
	})

	suite.assertRulesFile("updated\n")
	suite.assertCommands([]string{
		"/sbin/udevadm control --reload",
		"/sbin/udevadm trigger --type=devices --action=add",
		"/sbin/udevadm trigger --type=subsystems --action=add",
		"/sbin/udevadm settle --timeout=50",
		"/sbin/udevadm control --reload",
		"/sbin/udevadm trigger --type=devices --action=add",
		"/sbin/udevadm trigger --type=subsystems --action=add",
		"/sbin/udevadm settle --timeout=50",
	})

	ctest.UpdateWithConflicts(suite, configres.NewMachineConfig(suite.machineConfig(nil)), func(cfg *configres.MachineConfig) error {
		udevConfig := cfg.Config().UdevRulesConfig().(*runtimeconfig.UdevRulesConfigV1Alpha1)
		udevConfig.UdevRules = nil

		return nil
	})

	suite.EventuallyWithT(func(collect *assert.CollectT) {
		contents, err := os.ReadFile(suite.rulesPath)
		assert.NoError(collect, err)
		assert.Empty(collect, contents)
	}, 10*time.Second, 100*time.Millisecond)
}

func (suite *UdevRulesSuite) TestReloadIsDeferredUntilUdevdHealthy() {
	suite.createUdevdService(false)
	suite.createMachineConfig([]string{"ACTION==\"add\""})

	suite.assertRulesFile("ACTION==\"add\"\n")
	suite.Assert().Empty(suite.commandSnapshot())

	ctest.UpdateWithConflicts(suite, v1alpha1.NewService("udevd"), func(svc *v1alpha1.Service) error {
		svc.TypedSpec().Running = true
		svc.TypedSpec().Healthy = true

		return nil
	})

	suite.assertCommands([]string{
		"/sbin/udevadm control --reload",
		"/sbin/udevadm trigger --type=devices --action=add",
		"/sbin/udevadm trigger --type=subsystems --action=add",
		"/sbin/udevadm settle --timeout=50",
	})
}

func (suite *UdevRulesSuite) createUdevdService(healthy bool) {
	service := v1alpha1.NewService("udevd")
	service.TypedSpec().Running = healthy
	service.TypedSpec().Healthy = healthy

	suite.Create(service)
}

func (suite *UdevRulesSuite) createMachineConfig(rules []string) {
	suite.Create(configres.NewMachineConfig(suite.machineConfig(rules)))
}

func (suite *UdevRulesSuite) machineConfig(rules []string) *container.Container {
	udevConfig := runtimeconfig.NewUdevRulesConfigV1Alpha1()
	udevConfig.UdevRules = rules

	cfg, err := container.New(udevConfig)
	suite.Require().NoError(err)

	return cfg
}

func (suite *UdevRulesSuite) assertRulesFile(expected string) {
	suite.EventuallyWithT(func(collect *assert.CollectT) {
		contents, err := os.ReadFile(suite.rulesPath)
		assert.NoError(collect, err)
		assert.Equal(collect, expected, string(contents))
	}, 10*time.Second, 100*time.Millisecond)
}

func (suite *UdevRulesSuite) assertCommands(expected []string) {
	suite.EventuallyWithT(func(collect *assert.CollectT) {
		assert.Equal(collect, expected, suite.commandSnapshot())
	}, 10*time.Second, 100*time.Millisecond)
}

func (suite *UdevRulesSuite) commandSnapshot() []string {
	suite.mu.Lock()
	defer suite.mu.Unlock()

	return append([]string(nil), suite.commands...)
}
