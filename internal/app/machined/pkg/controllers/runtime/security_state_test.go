// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	runtimectrls "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime"
	machineruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/fipsmode"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

type SecurityStateSuite struct {
	ctest.DefaultSuite

	lockdownPath string
}

func TestSecurityStateSuite(t *testing.T) {
	t.Parallel()

	lockdownPath := filepath.Join(t.TempDir(), "lockdown")

	s := &SecurityStateSuite{
		lockdownPath: lockdownPath,
	}

	s.DefaultSuite = ctest.DefaultSuite{
		Timeout: 10 * time.Second,
		AfterSetup: func(suite *ctest.DefaultSuite) {
			// the file is in place before the controller starts, mirroring securityfs being
			// mounted by the time `machined` is up
			suite.Require().NoError(os.WriteFile(lockdownPath, renderLockdownFile(runtime.LockdownStateNone), 0o644))

			suite.Require().NoError(suite.Runtime().RegisterController(&runtimectrls.SecurityStateController{
				V1Alpha1Mode: machineruntime.ModeMetal,
				LockdownPath: lockdownPath,
			}))
		},
	}

	suite.Run(t, s)
}

// renderLockdownFile renders the securityfs lockdown file the way the kernel does, listing
// every level with the active one in brackets.
func renderLockdownFile(active runtime.LockdownState) []byte {
	var contents strings.Builder

	for _, level := range runtime.LockdownStateValues() {
		if level == active {
			fmt.Fprintf(&contents, "[%s] ", level)
		} else {
			fmt.Fprintf(&contents, "%s ", level)
		}
	}

	contents.WriteString("\n")

	return []byte(contents.String())
}

// startMachined marks the `machined` service as started, which is what the controller waits for
// before gathering any state.
func startMachined(suite *ctest.DefaultSuite) {
	machined := v1alpha1.NewService("machined")
	machined.TypedSpec().Running = true
	machined.TypedSpec().Healthy = true

	suite.Create(machined)
}

// setLockdownLevel rewrites the lockdown file in place, without truncating it first.
//
// Every rendering lists all of the levels and differs only in where the brackets sit, so the
// write never shortens the file: the kernel likewise never presents a half-rendered file, while
// `os.WriteFile` would leave an empty one behind for the controller to read.
func (suite *SecurityStateSuite) setLockdownLevel(level runtime.LockdownState) {
	suite.T().Helper()

	f, err := os.OpenFile(suite.lockdownPath, os.O_WRONLY, 0)
	suite.Require().NoError(err)

	defer func() {
		suite.Require().NoError(f.Close())
	}()

	_, err = f.Write(renderLockdownFile(level))
	suite.Require().NoError(err)
}

func (suite *SecurityStateSuite) assertLockdownState(expected runtime.LockdownState) {
	suite.T().Helper()

	ctest.AssertResource(suite, runtime.SecurityStateID, func(res *runtime.SecurityState, asrt *assert.Assertions) {
		asrt.Equal(expected, res.TypedSpec().LockdownState)
	})
}

// TestLockdownLevelRaised covers the kernel lockdown level being raised while Talos is
// running: it is picked up from the fsnotify event, well before the poll interval elapses.
func (suite *SecurityStateSuite) TestLockdownLevelRaised() {
	startMachined(&suite.DefaultSuite)

	suite.assertLockdownState(runtime.LockdownStateNone)

	for _, level := range []runtime.LockdownState{runtime.LockdownStateIntegrity, runtime.LockdownStateConfidentiality} {
		suite.setLockdownLevel(level)

		suite.assertLockdownState(level)
	}
}

// TestNoLockdownFile covers a kernel without the lockdown LSM enabled, where the securityfs
// file is never created.
func (suite *SecurityStateSuite) TestNoLockdownFile() {
	suite.Require().NoError(os.Remove(suite.lockdownPath))

	startMachined(&suite.DefaultSuite)

	suite.assertLockdownState(runtime.LockdownStateNone)
}

// TestNoChurn covers the state being republished only when it actually changes: the controller
// is woken up by unrelated service events and by its own poll ticks, neither of which should
// bump the resource version.
func (suite *SecurityStateSuite) TestNoChurn() {
	startMachined(&suite.DefaultSuite)

	suite.assertLockdownState(runtime.LockdownStateNone)

	res, err := ctest.Get[*runtime.SecurityState](suite, runtime.NewSecurityStateSpec(runtime.NamespaceName).Metadata())
	suite.Require().NoError(err)

	version := res.Metadata().Version()

	// wake the controller up a few times via its (weak) service input
	for range 5 {
		ctest.UpdateWithConflicts(suite, v1alpha1.NewService("machined"), func(svc *v1alpha1.Service) error {
			svc.TypedSpec().Healthy = !svc.TypedSpec().Healthy

			return nil
		})
	}

	// the lockdown level is raised last, so once the new level lands the controller has processed
	// the service events as well
	suite.setLockdownLevel(runtime.LockdownStateIntegrity)
	suite.assertLockdownState(runtime.LockdownStateIntegrity)

	res, err = ctest.Get[*runtime.SecurityState](suite, runtime.NewSecurityStateSpec(runtime.NamespaceName).Metadata())
	suite.Require().NoError(err)

	suite.Assert().Equal(version.Next().String(), res.Metadata().Version().String(), "the state should have been published exactly twice")
}

// SecurityStateContainerSuite covers container mode, where the host security state, including
// its kernel lockdown level, says nothing about the container: only the FIPS mode of the running
// binary is reported.
type SecurityStateContainerSuite struct {
	ctest.DefaultSuite
}

func TestSecurityStateContainerSuite(t *testing.T) {
	t.Parallel()

	lockdownPath := filepath.Join(t.TempDir(), "lockdown")

	suite.Run(t, &SecurityStateContainerSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 10 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(os.WriteFile(lockdownPath, renderLockdownFile(runtime.LockdownStateConfidentiality), 0o644))

				suite.Require().NoError(suite.Runtime().RegisterController(&runtimectrls.SecurityStateController{
					V1Alpha1Mode: machineruntime.ModeContainer,
					LockdownPath: lockdownPath,
				}))
			},
		},
	})
}

func (suite *SecurityStateContainerSuite) TestHostStateNotReported() {
	startMachined(&suite.DefaultSuite)

	ctest.AssertResource(suite, runtime.SecurityStateID, func(res *runtime.SecurityState, asrt *assert.Assertions) {
		spec := *res.TypedSpec()

		// the FIPS mode of the running binary is reported as-is
		asrt.Equal(fipsmode.Enabled(), spec.FIPSState != runtime.FIPSStateDisabled)

		spec.FIPSState = runtime.FIPSStateDisabled

		asrt.Equal(runtime.SecurityStateSpec{}, spec, "nothing but the FIPS mode describes the container itself")
	})
}
