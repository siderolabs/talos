/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package cmd

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type CmdSuite struct {
	suite.Suite
}

func TestCmdSuite(t *testing.T) {
	suite.Run(t, new(CmdSuite))
}

func (suite *CmdSuite) TestRun() {
	type args struct {
		name string
		args []string
	}
	tests := []struct {
		name      string
		args      args
		wantErr   bool
		errString string
	}{
		{
			"true",
			args{
				"true",
				[]string{},
			},
			false,
			"",
		},
		{
			"false",
			args{
				"badcommand",
				[]string{},
			},
			true,
			"exec: \"badcommand\": executable file not found in $PATH: ",
		},
	}
	for _, t := range tests {
		println(t.name)
		err := Run(t.args.name, t.args.args...)
		if t.wantErr {
			suite.Assert().Error(err)
			suite.Assert().Equal(err.Error(), t.errString)
		} else {
			suite.Assert().NoError(err)
		}
	}
}
