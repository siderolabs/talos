// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package network_test

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/jsimonetti/rtnetlink"
	"github.com/stretchr/testify/suite"
	"github.com/talos-systems/go-retry/retry"
	"golang.org/x/sys/unix"

	netctrl "github.com/talos-systems/talos/internal/app/machined/pkg/controllers/network"
	"github.com/talos-systems/talos/pkg/logging"
	"github.com/talos-systems/talos/pkg/machinery/nethelpers"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
)

type LinkStatusSuite struct {
	suite.Suite

	state state.State

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

func (suite *LinkStatusSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	suite.runtime, err = runtime.NewRuntime(suite.state, logging.Wrap(log.Writer()))
	suite.Require().NoError(err)

	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.LinkStatusController{}))

	suite.startRuntime()
}

func (suite *LinkStatusSuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
}

func (suite *LinkStatusSuite) uniqueDummyInterface() string {
	return fmt.Sprintf("dummy%02x%02x%02x", rand.Int31()&0xff, rand.Int31()&0xff, rand.Int31()&0xff)
}

func (suite *LinkStatusSuite) assertInterfaces(requiredIDs []string, check func(*network.LinkStatus) error) error {
	missingIDs := make(map[string]struct{}, len(requiredIDs))

	for _, id := range requiredIDs {
		missingIDs[id] = struct{}{}
	}

	resources, err := suite.state.List(
		suite.ctx,
		resource.NewMetadata(network.NamespaceName, network.LinkStatusType, "", resource.VersionUndefined),
	)
	if err != nil {
		return err
	}

	for _, res := range resources.Items {
		_, required := missingIDs[res.Metadata().ID()]
		if !required {
			continue
		}

		delete(missingIDs, res.Metadata().ID())

		if err = check(res.(*network.LinkStatus)); err != nil {
			return retry.ExpectedError(err)
		}
	}

	if len(missingIDs) > 0 {
		return retry.ExpectedError(fmt.Errorf("some resources are missing: %q", missingIDs))
	}

	return nil
}

func (suite *LinkStatusSuite) assertNoInterface(id string) error {
	resources, err := suite.state.List(
		suite.ctx,
		resource.NewMetadata(network.NamespaceName, network.LinkStatusType, "", resource.VersionUndefined),
	)
	if err != nil {
		return err
	}

	for _, res := range resources.Items {
		if res.Metadata().ID() == id {
			return retry.ExpectedError(fmt.Errorf("interface %q is still there", id))
		}
	}

	return nil
}

func (suite *LinkStatusSuite) TestInterfaceHwInfo() {
	errNoInterfaces := fmt.Errorf("no suitable interfaces found")

	err := retry.Constant(5*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			resources, err := suite.state.List(
				suite.ctx,
				resource.NewMetadata(network.NamespaceName, network.LinkStatusType, "", resource.VersionUndefined),
			)
			suite.Require().NoError(err)

			for _, res := range resources.Items {
				spec := res.(*network.LinkStatus).TypedSpec() //nolint:errcheck,forcetypeassert

				if !spec.Physical() {
					continue
				}

				if spec.Type != nethelpers.LinkEther {
					continue
				}

				emptyFields := []string{}

				for key, value := range map[string]string{
					"hw addr":  spec.HardwareAddr.String(),
					"driver":   spec.Driver,
					"bus path": spec.BusPath,
					"PCI id":   spec.PCIID,
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
	suite.Assert().NoError(
		retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertInterfaces(
					[]string{"lo"}, func(r *network.LinkStatus) error {
						suite.Assert().Equal("loopback", r.TypedSpec().Type.String())
						suite.Assert().EqualValues(65536, r.TypedSpec().MTU)

						return nil
					},
				)
			},
		),
	)
}

func (suite *LinkStatusSuite) TestDummyInterface() {
	dummyInterface := suite.uniqueDummyInterface()

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

	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertInterfaces(
					[]string{dummyInterface}, func(r *network.LinkStatus) error {
						suite.Assert().Equal("ether", r.TypedSpec().Type.String())
						suite.Assert().EqualValues(1400, r.TypedSpec().MTU)
						suite.Assert().Equal(nethelpers.OperStateDown, r.TypedSpec().OperationalState)

						return nil
					},
				)
			},
		),
	)

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

	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertInterfaces(
					[]string{dummyInterface}, func(r *network.LinkStatus) error {
						if r.TypedSpec().OperationalState != nethelpers.OperStateUp && r.TypedSpec().OperationalState != nethelpers.OperStateUnknown {
							return retry.ExpectedError(
								fmt.Errorf(
									"operational state is not up: %s",
									r.TypedSpec().OperationalState,
								),
							)
						}

						return nil
					},
				)
			},
		),
	)

	suite.Require().NoError(conn.Link.Delete(uint32(iface.Index)))

	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertNoInterface(dummyInterface)
			},
		),
	)
}

func (suite *LinkStatusSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.wg.Wait()

	// trigger updates in resources to stop watch loops
	suite.Assert().NoError(
		suite.state.Create(
			context.Background(),
			network.NewLinkRefresh(network.NamespaceName, "bar"),
		),
	)
}

func TestLinkStatusSuite(t *testing.T) {
	suite.Run(t, new(LinkStatusSuite))
}
