// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package argsbuilder_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/pkg/argsbuilder"
)

type ArgsbuilderSuite struct {
	suite.Suite
}

func (suite *ArgsbuilderSuite) TestMergeAdditive() {
	args := argsbuilder.Args{
		"param":  {"value1,value2,value3"},
		"param2": {""},
	}

	suite.Require().NoError(
		args.Merge(
			argsbuilder.Args{
				"param": {"value2, value10"},
			},
			argsbuilder.WithMergePolicies(argsbuilder.MergePolicies{
				"param": argsbuilder.MergeAdditive,
			}),
		),
	)

	suite.Require().Equal([]string{"value1,value2,value3,value10"}, args["param"])
	suite.Assert().Equal([]string{"--param=value1,value2,value3,value10", "--param2="}, args.Args())

	suite.Require().NoError(
		args.Merge(argsbuilder.Args{
			"param2": {"value1, value5"},
		},
			argsbuilder.WithMergePolicies(argsbuilder.MergePolicies{
				"param2": argsbuilder.MergeAdditive,
			}),
		),
	)

	suite.Require().Equal([]string{"value1,value5"}, args["param2"])
	suite.Assert().Equal([]string{"--param=value1,value2,value3,value10", "--param2=value1,value5"}, args.Args())
}

func (suite *ArgsbuilderSuite) TestMergeOverwrite() {
	args := argsbuilder.Args{
		"param": {"value1,value2"},
	}

	suite.Require().NoError(
		args.Merge(argsbuilder.Args{
			"param": {"value10"},
		}),
	)

	suite.Require().Equal([]string{"value10"}, args["param"])
	suite.Assert().Equal([]string{"--param=value10"}, args.Args())

	suite.Require().NoError(
		args.Merge(argsbuilder.Args{
			"param": {"value10", "value11"},
		}),
	)

	suite.Require().Equal([]string{"value10", "value11"}, args["param"])
	suite.Assert().Equal([]string{"--param=value10", "--param=value11"}, args.Args())
}

//nolint:dupl
func (suite *ArgsbuilderSuite) TestMergePrepend() {
	args := argsbuilder.Args{
		"param": {"value1"},
	}

	suite.Require().NoError(
		args.Merge(argsbuilder.Args{
			"param": {"value2", "value3"},
		},
			argsbuilder.WithMergePolicies(argsbuilder.MergePolicies{
				"param": argsbuilder.MergePrepend,
			}),
		),
	)

	suite.Require().Equal([]string{"value2", "value3", "value1"}, args["param"])
	suite.Assert().Equal([]string{"--param=value2", "--param=value3", "--param=value1"}, args.Args())

	suite.Require().NoError(
		args.Merge(argsbuilder.Args{
			"param": {"value4"},
		},
			argsbuilder.WithMergePolicies(argsbuilder.MergePolicies{
				"param": argsbuilder.MergePrepend,
			}),
		),
	)

	suite.Require().Equal([]string{"value4", "value2", "value3", "value1"}, args["param"])
	suite.Assert().Equal([]string{"--param=value4", "--param=value2", "--param=value3", "--param=value1"}, args.Args())
}

//nolint:dupl
func (suite *ArgsbuilderSuite) TestMergeAppend() {
	args := argsbuilder.Args{
		"param": {"value1"},
	}

	suite.Require().NoError(
		args.Merge(argsbuilder.Args{
			"param": {"value2", "value3"},
		},
			argsbuilder.WithMergePolicies(argsbuilder.MergePolicies{
				"param": argsbuilder.MergeAppend,
			}),
		),
	)

	suite.Require().Equal([]string{"value1", "value2", "value3"}, args["param"])
	suite.Assert().Equal([]string{"--param=value1", "--param=value2", "--param=value3"}, args.Args())

	suite.Require().NoError(
		args.Merge(argsbuilder.Args{
			"param": {"value4"},
		},
			argsbuilder.WithMergePolicies(argsbuilder.MergePolicies{
				"param": argsbuilder.MergeAppend,
			}),
		),
	)

	suite.Require().Equal([]string{"value1", "value2", "value3", "value4"}, args["param"])
	suite.Assert().Equal([]string{"--param=value1", "--param=value2", "--param=value3", "--param=value4"}, args.Args())
}

func (suite *ArgsbuilderSuite) TestMergeDenied() {
	args := argsbuilder.Args{
		"param": {"value1,value2"},
	}

	suite.Require().Error(
		args.Merge(argsbuilder.Args{
			"param": {"value10"},
		},
			argsbuilder.WithMergePolicies(argsbuilder.MergePolicies{
				"param": argsbuilder.MergeDenied,
			}),
		),
	)
}

func (suite *ArgsbuilderSuite) TestMergeDenyList() {
	args := argsbuilder.Args{
		"param": {"value1,value2"},
	}

	denyList := argsbuilder.Args{
		"param1": {""},
		"param2": {""},
		"param3": {""},
	}

	suite.Require().Error(
		args.Merge(argsbuilder.Args{
			"param2": {"value10"},
		},
			argsbuilder.WithDenyList(denyList),
		),
	)
}

func TestArgsbuilderSuite(t *testing.T) {
	suite.Run(t, &ArgsbuilderSuite{})
}
