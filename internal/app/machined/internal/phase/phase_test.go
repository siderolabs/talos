/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package phase_test

import (
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/pkg/runtime"
)

type PhaseSuite struct {
	suite.Suite

	platformExists bool
	platformValue  string
}

type regularTask struct {
	errCh <-chan error
}

func (t *regularTask) RuntimeFunc(runtime.Mode) phase.RuntimeFunc {
	return func(*phase.RuntimeArgs) error {
		return <-t.errCh
	}
}

type nilTask struct{}

func (t *nilTask) RuntimeFunc(runtime.Mode) phase.RuntimeFunc {
	return nil
}

type panicTask struct{}

func (t *panicTask) RuntimeFunc(runtime.Mode) phase.RuntimeFunc {
	return func(*phase.RuntimeArgs) error {
		panic("in task")
	}
}

func (suite *PhaseSuite) SetupSuite() {
	suite.platformValue, suite.platformExists = os.LookupEnv("PLATFORM")
	suite.Require().NoError(os.Setenv("PLATFORM", "container"))
}

func (suite *PhaseSuite) TearDownSuite() {
	if !suite.platformExists {
		suite.Require().NoError(os.Unsetenv("PLATFORM"))
	} else {
		suite.Require().NoError(os.Setenv("PLATFORM", suite.platformValue))
	}
}

func (suite *PhaseSuite) TestRunSuccess() {
	r, err := phase.NewRunner(nil)
	suite.Require().NoError(err)

	taskErr := make(chan error)

	r.Add(phase.NewPhase("empty"))
	r.Add(phase.NewPhase("phase1", &regularTask{errCh: taskErr}, &regularTask{errCh: taskErr}))
	r.Add(phase.NewPhase("phase2", &regularTask{errCh: taskErr}, &nilTask{}))

	errCh := make(chan error)
	go func() {
		errCh <- r.Run()
	}()

	taskErr <- nil
	taskErr <- nil

	select {
	case <-errCh:
		suite.Require().Fail("should be still running")
	default:
	}

	taskErr <- nil

	suite.Require().NoError(<-errCh)
}

func (suite *PhaseSuite) TestRunFailures() {
	r, err := phase.NewRunner(nil)
	suite.Require().NoError(err)

	taskErr := make(chan error, 1)

	r.Add(phase.NewPhase("empty"))
	r.Add(phase.NewPhase("failphase", &panicTask{}, &regularTask{errCh: taskErr}, &nilTask{}))
	r.Add(phase.NewPhase("neverreached",
		&regularTask{}, // should never be reached
	))

	taskErr <- errors.New("test error")

	err = r.Run()
	suite.Require().Error(err)
	suite.Assert().Contains(err.Error(), "2 errors occurred")
	suite.Assert().Contains(err.Error(), "test error")
	suite.Assert().Contains(err.Error(), "panic recovered: in task")
}

func TestPhaseSuite(t *testing.T) {
	suite.Run(t, new(PhaseSuite))
}
