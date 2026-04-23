// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/jsimonetti/rtnetlink/v2"
	"github.com/mdlayher/netlink"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

type LinkStatusSuite struct {
	ctest.DefaultSuite
}

func uniqueDummyInterface() string {
	return fmt.Sprintf("dummy%02x%02x%02x", rand.Int32()&0xff, rand.Int32()&0xff, rand.Int32()&0xff)
}

func (suite *LinkStatusSuite) TestInterfaceHwInfo() {
	errNoInterfaces := errors.New("no suitable interfaces found")

	err := retry.Constant(5*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			resources, err := safe.StateListAll[*network.LinkStatus](suite.Ctx(), suite.State())
			suite.Require().NoError(err)

			for res := range resources.All() {
				spec := res.TypedSpec()

				if !spec.Physical() {
					continue
				}

				if spec.Type != nethelpers.LinkEther {
					continue
				}

				var emptyFields []string

				for key, value := range map[string]string{
					"hw addr":   spec.HardwareAddr.String(),
					"perm addr": spec.PermanentAddr.String(),
					"driver":    spec.Driver,
					"bus path":  spec.BusPath,
					"PCI id":    spec.PCIID,
				} {
					if value == "" {
						emptyFields = append(emptyFields, key)
					}
				}

				if len(emptyFields) > 0 {
					return fmt.Errorf("the interface %s has the following fields empty: %s", res.Metadata().ID(), strings.Join(emptyFields, ", "))
				}

				return nil
			}

			return retry.ExpectedError(errNoInterfaces)
		},
	)
	if errors.Is(err, errNoInterfaces) {
		suite.T().Skip(err.Error())
	}

	suite.Require().NoError(err)
}

func (suite *LinkStatusSuite) TestLoopbackInterface() {
	ctest.AssertResource(suite, "lo", func(r *network.LinkStatus, asrt *assert.Assertions) {
		asrt.Equal("loopback", r.TypedSpec().Type.String())
		asrt.EqualValues(65536, r.TypedSpec().MTU)
	})
}

func (suite *LinkStatusSuite) TestDummyInterface() {
	if os.Geteuid() != 0 {
		suite.T().Skip("requires root")
	}

	dummyInterface := uniqueDummyInterface()

	conn, err := rtnetlink.Dial(nil)
	suite.Require().NoError(err)

	defer conn.Close() //nolint:errcheck

	suite.Require().NoError(
		conn.Link.New(
			&rtnetlink.LinkMessage{
				Type: unix.ARPHRD_ETHER,
				Attributes: &rtnetlink.LinkAttributes{
					Name: dummyInterface,
					MTU:  1400,
					Info: &rtnetlink.LinkInfo{
						Kind: "dummy",
					},
				},
			},
		),
	)

	iface, err := net.InterfaceByName(dummyInterface)
	suite.Require().NoError(err)

	defer conn.Link.Delete(uint32(iface.Index)) //nolint:errcheck

	ctest.AssertResource(suite, dummyInterface, func(r *network.LinkStatus, asrt *assert.Assertions) {
		asrt.Equal("ether", r.TypedSpec().Type.String())
		asrt.EqualValues(1400, r.TypedSpec().MTU)
		asrt.Equal(nethelpers.OperStateDown, r.TypedSpec().OperationalState)
	})

	suite.Require().NoError(
		conn.Link.Set(
			&rtnetlink.LinkMessage{
				Type:   unix.ARPHRD_ETHER,
				Index:  uint32(iface.Index),
				Flags:  unix.IFF_UP,
				Change: unix.IFF_UP,
			},
		),
	)

	ctest.AssertResource(suite, dummyInterface, func(r *network.LinkStatus, asrt *assert.Assertions) {
		asrt.Contains(
			[]nethelpers.OperationalState{nethelpers.OperStateUp, nethelpers.OperStateUnknown},
			r.TypedSpec().OperationalState,
		)
	})

	suite.Require().NoError(conn.Link.Delete(uint32(iface.Index)))

	ctest.AssertNoResource[*network.LinkStatus](suite, dummyInterface)
}

func (suite *LinkStatusSuite) TestBridgeInterface() {
	if os.Geteuid() != 0 {
		suite.T().Skip("requires root")
	}

	bridgeInterface := uniqueDummyInterface()

	conn, err := rtnetlink.Dial(nil)
	suite.Require().NoError(err)

	defer conn.Close() //nolint:errcheck

	bridgeData, err := encodeBridgeData(true)
	suite.Require().NoError(err)

	suite.Require().NoError(
		conn.Link.New(
			&rtnetlink.LinkMessage{
				Type: unix.ARPHRD_ETHER,
				Attributes: &rtnetlink.LinkAttributes{
					Name: bridgeInterface,
					Info: &rtnetlink.LinkInfo{
						Kind: "bridge",
						Data: &rtnetlink.LinkData{
							Name: "bridge",
							Data: bridgeData,
						},
					},
				},
			},
		),
	)

	bridgeIface, err := net.InterfaceByName(bridgeInterface)
	suite.Require().NoError(err)

	defer conn.Link.Delete(uint32(bridgeIface.Index)) //nolint:errcheck

	ctest.AssertResource(suite, bridgeInterface, func(r *network.LinkStatus, asrt *assert.Assertions) {
		asrt.Equal("ether", r.TypedSpec().Type.String())
		asrt.True(r.TypedSpec().BridgeMaster.STP.Enabled)
	})
}

func encodeBridgeData(stpEnabled bool) ([]byte, error) {
	encoder := netlink.NewAttributeEncoder()

	var stpState uint32
	if stpEnabled {
		stpState = 1
	}

	encoder.Uint32(unix.IFLA_BR_STP_STATE, stpState)

	return encoder.Encode()
}

func TestLinkStatusSuite(t *testing.T) {
	suite.Run(t, &LinkStatusSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 15 * time.Second,
			AfterSetup: func(s *ctest.DefaultSuite) {
				// create fake device ready status
				deviceStatus := runtimeres.NewDevicesStatus(runtimeres.NamespaceName, runtimeres.DevicesID)
				deviceStatus.TypedSpec().Ready = true
				s.Require().NoError(s.State().Create(s.Ctx(), deviceStatus))

				s.Require().NoError(s.Runtime().RegisterController(&netctrl.LinkStatusController{}))
			},
		},
	})
}
