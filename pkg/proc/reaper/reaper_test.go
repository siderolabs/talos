// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package reaper_test

import (
	"os/exec"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/talos-systems/talos/pkg/proc/reaper"
)

type ReaperSuite struct {
	suite.Suite
}

func (suite *ReaperSuite) SetupSuite() {
	reaper.Run()
}

func (suite *ReaperSuite) TearDownSuite() {
	reaper.Shutdown()
}

func (suite *ReaperSuite) TestNoActivity() {
}

func (suite *ReaperSuite) TestNoNotify() {
	const N = 5
	commands := make([]*exec.Cmd, N)

	for i := range commands {
		commands[i] = exec.Command("/bin/sh", "-c", ":")
		suite.Assert().NoError(commands[i].Start())
	}

	// let the reaper do the work
	time.Sleep(time.Second)

	// zombies should have been reaped, so `Wait()` should fail
	for i := range commands {
		suite.Assert().EqualError(commands[i].Wait(), "waitid: no child processes")
	}
}

//nolint: gocyclo
func (suite *ReaperSuite) TestNotifyStop() {
	const N = 5
	commands := make([]*exec.Cmd, N)

	notifyCh := make([]chan reaper.ProcessInfo, 2)
	for j := range notifyCh {
		notifyCh[j] = make(chan reaper.ProcessInfo, N)
		reaper.Notify(notifyCh[j])
	}

	expectedPids := make([]int, N)

	for i := range commands {
		commands[i] = exec.Command("/bin/sh", "-c", ":")
		suite.Require().NoError(commands[i].Start())
		expectedPids[i] = commands[i].Process.Pid
	}

	sort.Ints(expectedPids)

	for _, ch := range notifyCh {
		gotPids := make([]int, N)
		for i := range gotPids {
			info := <-ch
			suite.T().Log(info)
			gotPids[i] = info.Pid

			suite.Assert().True(info.Status.Exited())
			suite.Assert().Equal(0, info.Status.ExitStatus())
		}

		sort.Ints(gotPids)

		suite.Assert().Equal(expectedPids, gotPids)
	}

	// zombies should have been reaped, so `Wait()` should fail
	for i := range commands {
		suite.Assert().EqualError(commands[i].Wait(), "waitid: no child processes")
	}

	for _, ch := range notifyCh {
		select {
		case <-ch:
			suite.Require().Fail("there should be no more notifications in the channel")
		default:
		}
	}

	reaper.Stop(notifyCh[0])

	command := exec.Command("/bin/sh", "-c", ":")
	suite.Require().NoError(command.Start())

	// notification should come on still active notify channel
	<-notifyCh[1]

	select {
	case <-notifyCh[0]:
		suite.Require().Fail("there should be no notifications after Stop")
	default:
	}

	for j := range notifyCh {
		reaper.Stop(notifyCh[j])
	}
}

func (suite *ReaperSuite) TestFailedProcess() {
	notifyCh := make(chan reaper.ProcessInfo, 1)

	reaper.Notify(notifyCh)
	defer reaper.Stop(notifyCh)

	command := exec.Command("/bin/sh", "-c", "exit 3")
	suite.Require().NoError(command.Start())

	info := <-notifyCh

	suite.Assert().Equal(command.Process.Pid, info.Pid)
	suite.Assert().True(info.Status.Exited())
	suite.Assert().Equal(3, info.Status.ExitStatus())
	suite.Assert().EqualError(command.Wait(), "waitid: no child processes")
}

func (suite *ReaperSuite) TestWait() {
	type args struct {
		name string
		args []string
	}

	tests := []struct {
		args      args
		errString string
	}{
		{
			args{
				"true",
				[]string{},
			},
			"",
		},
		{
			args{
				"false",
				[]string{},
			},
			"exit status 1",
		},
		{
			args{
				"/bin/sh",
				[]string{
					"-c",
					"kill -2 $$",
				},
			},
			"signal: interrupt",
		},
	}

	notifyCh := make(chan reaper.ProcessInfo, 1)

	suite.Require().True(reaper.Notify(notifyCh))
	defer reaper.Stop(notifyCh)

	for _, t := range tests {
		cmd := exec.Command(t.args.name, t.args.args...)
		suite.Require().NoError(cmd.Start())

		err := reaper.WaitWrapper(true, notifyCh, cmd)

		if t.errString == "" {
			suite.Assert().NoError(err)
		} else {
			suite.Assert().EqualError(err, t.errString)
		}
	}
}

func TestReaperSuite(t *testing.T) {
	suite.Run(t, new(ReaperSuite))
}
