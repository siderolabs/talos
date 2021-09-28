// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package argsbuilder_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/talos-systems/talos/pkg/argsbuilder"
)

type ArgsbuilderSuite struct {
	suite.Suite
}

func (suite *ArgsbuilderSuite) TestMergeAdditive() {
	args := argsbuilder.Args{
		"param":  "value1,value2,value3",
		"param2": "",
	}

	suite.Require().NoError(
		args.Merge(argsbuilder.Args{
			"param": "value2, value10",
		},
			argsbuilder.WithMergePolicies(argsbuilder.MergePolicies{
				"param": argsbuilder.MergeAdditive,
			}),
		),
	)

	suite.Require().Equal("value1,value2,value3,value10", args["param"])

	suite.Require().NoError(
		args.Merge(argsbuilder.Args{
			"param2": "value1, value5",
		},
			argsbuilder.WithMergePolicies(argsbuilder.MergePolicies{
				"param2": argsbuilder.MergeAdditive,
			}),
		),
	)

	suite.Require().Equal("value1,value5", args["param2"])
}

func (suite *ArgsbuilderSuite) TestMergeOverwrite() {
	args := argsbuilder.Args{
		"param": "value1,value2",
	}

	suite.Require().NoError(
		args.Merge(argsbuilder.Args{
			"param": "value10",
		},
			argsbuilder.WithMergePolicies(argsbuilder.MergePolicies{
				"param2": argsbuilder.MergeAdditive,
			}),
		),
	)

	suite.Require().Equal("value10", args["param"])
}

func (suite *ArgsbuilderSuite) TestMergeDenied() {
	args := argsbuilder.Args{
		"param": "value1,value2",
	}

	suite.Require().Error(
		args.Merge(argsbuilder.Args{
			"param": "value10",
		},
			argsbuilder.WithMergePolicies(argsbuilder.MergePolicies{
				"param": argsbuilder.MergeDenied,
			}),
		),
	)
}

func (suite *ArgsbuilderSuite) TestMergeDenyList() {
	args := argsbuilder.Args{
		"param": "value1,value2",
	}

	denyList := argsbuilder.Args{
		"param1": "",
		"param2": "",
		"param3": "",
	}

	suite.Require().Error(
		args.Merge(argsbuilder.Args{
			"param2": "value10",
		},
			argsbuilder.WithDenyList(denyList),
		),
	)
}

func TestArgsbuilderSuite(t *testing.T) {
	suite.Run(t, &ArgsbuilderSuite{})
}
