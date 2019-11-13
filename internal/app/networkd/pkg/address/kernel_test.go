// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package address

import (
	"context"
	"io/ioutil"
	"net"
	"os"
	"testing"

	"github.com/stretchr/testify/suite"
	"golang.org/x/sys/unix"

	"github.com/talos-systems/talos/internal/pkg/kernel"
)

type AddressSuite struct {
	suite.Suite
}

func TestAddressSuite(t *testing.T) {
	// Hide all our state transition messages
	// log.SetOutput(ioutil.Discard)
	suite.Run(t, new(AddressSuite))
}

func (suite *AddressSuite) TestFullKernelAddress() {
	tmpfile, err := ioutil.TempFile("", "example")
	suite.Assert().NoError(err)

	// nolint: errcheck
	defer os.Remove(tmpfile.Name())

	_, err = tmpfile.Write([]byte("ip=1.1.1.1:2.2.2.2:3.3.3.3:255.255.255.0:hostname:eth0:none:4.4.4.4:5.5.5.5:6.6.6.6"))
	suite.Assert().NoError(err)
	err = tmpfile.Close()
	suite.Assert().NoError(err)

	kernel.CmdLine = tmpfile.Name()

	kern := &Kernel{}
	err = kern.Discover(context.Background())
	suite.Require().NoError(err)

	suite.Assert().Equal(kern.Name(), "kernel")
	suite.Assert().Equal(kern.Family(), unix.AF_INET)
	suite.Assert().Equal(kern.Address().IP, net.ParseIP("1.1.1.1"))
	suite.Assert().Equal(kern.Mask().String(), "ffffff00")
	suite.Assert().Equal(len(kern.Routes()), 0)
	suite.Assert().Equal(kern.Hostname(), "hostname")
	suite.Assert().Equal(kern.Resolvers()[0], net.ParseIP("4.4.4.4"))
	suite.Assert().Equal(kern.Resolvers()[1], net.ParseIP("5.5.5.5"))
}

/*
func (suite *AddressSuite) TestPartialKernelAddress() {
	tmpfile, err := ioutil.TempFile("", "example")
	suite.Assert().NoError(err)

	defer os.Remove(tmpfile.Name())

	_, err = tmpfile.Write([]byte("ip=1.1.1.1:2.2.2.2:3.3.3.3:255.255.255.0"))
	suite.Assert().NoError(err)
	err = tmpfile.Close()
	suite.Assert().NoError(err)

	kernel.CmdLine = tmpfile.Name()

	kern := &Kernel{}
	err = kern.Discover(context.Background())
	suite.Require().NoError(err)

	suite.Assert().Equal(kern.Name(), "kernel")
	suite.Assert().Equal(kern.Family(), unix.AF_INET)
	suite.Assert().Equal(kern.Address().IP, net.ParseIP("1.1.1.1"))
	suite.Assert().Equal(kern.Mask().String(), "ffffff00")
}

func (suite *AddressSuite) TestIncompleteKernelAddress() {
	tmpfile, err := ioutil.TempFile("", "example")
	suite.Assert().NoError(err)

	defer os.Remove(tmpfile.Name())

	_, err = tmpfile.Write([]byte("ip=1.1.1.1"))
	suite.Assert().NoError(err)
	err = tmpfile.Close()
	suite.Assert().NoError(err)

	kernel.CmdLine = tmpfile.Name()

	kern := &Kernel{}
	err = kern.Discover(context.Background())
	suite.Require().Error(err)
}
*/
