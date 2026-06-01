// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"fmt"
	"net"
	"net/netip"
	"os"
	"testing"
	"time"

	"github.com/jsimonetti/rtnetlink/v2"
	"github.com/siderolabs/go-pointer"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/suite"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type RoutingRuleSpecSuite struct {
	ctest.DefaultSuite
}

func (suite *RoutingRuleSpecSuite) assertRule(
	family nethelpers.Family,
	src, dst netip.Prefix,
	priority uint32,
	check func(rtnetlink.RuleMessage) error,
) error {
	conn, err := rtnetlink.Dial(nil)
	suite.Require().NoError(err)

	defer conn.Close() //nolint:errcheck

	rules, err := conn.Rule.List()
	suite.Require().NoError(err)

	for _, rule := range rules {
		if rule.Family != uint8(family) {
			continue
		}

		if pointer.SafeDeref(rule.Attributes.Priority) != priority {
			continue
		}

		if !matchPrefix(rule.Attributes.Src, rule.SrcLength, src) {
			continue
		}

		if !matchPrefix(rule.Attributes.Dst, rule.DstLength, dst) {
			continue
		}

		if err = check(rule); err != nil {
			return retry.ExpectedError(err)
		}

		return nil
	}

	return retry.ExpectedErrorf("rule family=%s src=%s dst=%s priority=%d not found", family, src, dst, priority)
}

func (suite *RoutingRuleSpecSuite) assertNoRule(
	family nethelpers.Family,
	src, dst netip.Prefix,
	priority uint32,
) error {
	conn, err := rtnetlink.Dial(nil)
	suite.Require().NoError(err)

	defer conn.Close() //nolint:errcheck

	rules, err := conn.Rule.List()
	suite.Require().NoError(err)

	for _, rule := range rules {
		if rule.Family != uint8(family) {
			continue
		}

		if pointer.SafeDeref(rule.Attributes.Priority) != priority {
			continue
		}

		if !matchPrefix(rule.Attributes.Src, rule.SrcLength, src) {
			continue
		}

		if !matchPrefix(rule.Attributes.Dst, rule.DstLength, dst) {
			continue
		}

		return retry.ExpectedErrorf("rule family=%s src=%q dst=%q priority=%d is still present", family, src, dst, priority)
	}

	return nil
}

func matchPrefix(ip *net.IP, length uint8, prefix netip.Prefix) bool {
	if !prefix.IsValid() || prefix.Bits() == 0 {
		return length == 0
	}

	if length != uint8(prefix.Bits()) {
		return false
	}

	if ip == nil {
		return false
	}

	addr, ok := netip.AddrFromSlice(*ip)
	if !ok {
		return false
	}

	return addr == prefix.Addr()
}

//nolint:dupl
func (suite *RoutingRuleSpecSuite) TestCreateAndDelete() {
	priority := uint32(31000)

	// Use a high priority number to avoid conflicting with default rules.
	rule := network.NewRoutingRuleSpec(network.NamespaceName, "test-rule")
	*rule.TypedSpec() = network.RoutingRuleSpecSpec{
		Family:      nethelpers.FamilyInet4,
		Src:         netip.MustParsePrefix("10.99.0.0/16"),
		Table:       nethelpers.RoutingTable(100),
		Priority:    priority,
		Action:      nethelpers.RoutingRuleActionUnicast,
		ConfigLayer: network.ConfigMachineConfiguration,
	}

	suite.Create(rule)

	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertRule(
					nethelpers.FamilyInet4,
					netip.MustParsePrefix("10.99.0.0/16"),
					netip.Prefix{},
					priority,
					func(r rtnetlink.RuleMessage) error {
						table := uint32(r.Table)
						if r.Attributes.Table != nil {
							table = *r.Attributes.Table
						}

						if table != 100 {
							return fmt.Errorf("unexpected table: got %d, want %d", table, 100)
						}

						if r.Action != unix.FR_ACT_TO_TBL {
							return fmt.Errorf("unexpected action: got %d, want %d", r.Action, unix.FR_ACT_TO_TBL)
						}

						return nil
					},
				)
			},
		),
	)

	// Teardown the rule.
	suite.Require().NoError(suite.State().TeardownAndDestroy(suite.Ctx(), rule.Metadata()))

	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertNoRule(
					nethelpers.FamilyInet4,
					netip.MustParsePrefix("10.99.0.0/16"),
					netip.Prefix{},
					priority,
				)
			},
		),
	)
}

//nolint:dupl
func (suite *RoutingRuleSpecSuite) TestFwMarkUpdate() {
	priority := uint32(31002)

	rule := network.NewRoutingRuleSpec(network.NamespaceName, "fwmark-update-rule")
	*rule.TypedSpec() = network.RoutingRuleSpecSpec{
		Family:      nethelpers.FamilyInet4,
		Table:       nethelpers.RoutingTable(100),
		Priority:    priority,
		Action:      nethelpers.RoutingRuleActionUnicast,
		FwMark:      0x100,
		FwMask:      0xff00,
		ConfigLayer: network.ConfigMachineConfiguration,
	}

	suite.Create(rule)

	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertRule(
					nethelpers.FamilyInet4,
					netip.Prefix{},
					netip.Prefix{},
					priority,
					func(r rtnetlink.RuleMessage) error {
						if pointer.SafeDeref(r.Attributes.FwMark) != 0x100 {
							return fmt.Errorf("unexpected fwmark: got %x, want %x", pointer.SafeDeref(r.Attributes.FwMark), 0x100)
						}

						if pointer.SafeDeref(r.Attributes.FwMask) != 0xff00 {
							return fmt.Errorf("unexpected fwmask: got %x, want %x", pointer.SafeDeref(r.Attributes.FwMask), 0xff00)
						}

						return nil
					},
				)
			},
		),
	)

	// finalizer updates the rule, so we need to fetch the latest version before proceeding with the update.
	r, err := suite.State().Get(suite.Ctx(), rule.Metadata())
	suite.Require().NoError(err)

	rule = r.(*network.RoutingRuleSpec)
	rule.TypedSpec().FwMark = 0x200
	suite.Update(rule)

	suite.Assert().NoError(
		retry.Constant(5*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertRule(
					nethelpers.FamilyInet4,
					netip.Prefix{},
					netip.Prefix{},
					priority,
					func(r rtnetlink.RuleMessage) error {
						if pointer.SafeDeref(r.Attributes.FwMark) != 0x200 {
							return fmt.Errorf("unexpected fwmark after update: got %x, want %x", pointer.SafeDeref(r.Attributes.FwMark), 0x200)
						}

						if pointer.SafeDeref(r.Attributes.FwMask) != 0xff00 {
							return fmt.Errorf("unexpected fwmask after update: got %x, want %x", pointer.SafeDeref(r.Attributes.FwMask), 0xff00)
						}

						return nil
					},
				)
			},
		),
	)

	suite.Require().NoError(suite.State().TeardownAndDestroy(suite.Ctx(), rule.Metadata()))

	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertNoRule(
					nethelpers.FamilyInet4,
					netip.Prefix{},
					netip.Prefix{},
					priority,
				)
			},
		),
	)
}

//nolint:dupl
func (suite *RoutingRuleSpecSuite) TestFwMark() {
	priority := uint32(31001)

	rule := network.NewRoutingRuleSpec(network.NamespaceName, "fwmark-rule")
	*rule.TypedSpec() = network.RoutingRuleSpecSpec{
		Family:      nethelpers.FamilyInet4,
		Table:       nethelpers.RoutingTable(100),
		Priority:    priority,
		Action:      nethelpers.RoutingRuleActionUnicast,
		FwMark:      0x100,
		FwMask:      0xff00,
		ConfigLayer: network.ConfigMachineConfiguration,
	}

	suite.Create(rule)

	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertRule(
					nethelpers.FamilyInet4,
					netip.Prefix{},
					netip.Prefix{},
					priority,
					func(r rtnetlink.RuleMessage) error {
						if pointer.SafeDeref(r.Attributes.FwMark) != 0x100 {
							return fmt.Errorf("unexpected fwmark: got %x, want %x", pointer.SafeDeref(r.Attributes.FwMark), 0x100)
						}

						if pointer.SafeDeref(r.Attributes.FwMask) != 0xff00 {
							return fmt.Errorf("unexpected fwmask: got %x, want %x", pointer.SafeDeref(r.Attributes.FwMask), 0xff00)
						}

						return nil
					},
				)
			},
		),
	)

	// Teardown.
	suite.Require().NoError(suite.State().TeardownAndDestroy(suite.Ctx(), rule.Metadata()))

	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertNoRule(
					nethelpers.FamilyInet4,
					netip.Prefix{},
					netip.Prefix{},
					priority,
				)
			},
		),
	)
}

//nolint:dupl
func (suite *RoutingRuleSpecSuite) TestIPv6() {
	priority := uint32(31003)

	rule := network.NewRoutingRuleSpec(network.NamespaceName, "ipv6-rule")
	*rule.TypedSpec() = network.RoutingRuleSpecSpec{
		Family:      nethelpers.FamilyInet6,
		Src:         netip.MustParsePrefix("fd00::/8"),
		Table:       nethelpers.RoutingTable(100),
		Priority:    priority,
		Action:      nethelpers.RoutingRuleActionUnicast,
		ConfigLayer: network.ConfigMachineConfiguration,
	}

	suite.Create(rule)

	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertRule(
					nethelpers.FamilyInet6,
					netip.MustParsePrefix("fd00::/8"),
					netip.Prefix{},
					priority,
					func(r rtnetlink.RuleMessage) error {
						if r.Action != unix.FR_ACT_TO_TBL {
							return fmt.Errorf("unexpected action: got %d, want %d", r.Action, unix.FR_ACT_TO_TBL)
						}

						return nil
					},
				)
			},
		),
	)

	suite.Require().NoError(suite.State().TeardownAndDestroy(suite.Ctx(), rule.Metadata()))

	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertNoRule(
					nethelpers.FamilyInet6,
					netip.MustParsePrefix("fd00::/8"),
					netip.Prefix{},
					priority,
				)
			},
		),
	)
}

//nolint:dupl
func (suite *RoutingRuleSpecSuite) TestDstPrefix() {
	priority := uint32(31004)

	rule := network.NewRoutingRuleSpec(network.NamespaceName, "dst-prefix-rule")
	*rule.TypedSpec() = network.RoutingRuleSpecSpec{
		Family:      nethelpers.FamilyInet4,
		Dst:         netip.MustParsePrefix("192.168.0.0/16"),
		Table:       nethelpers.RoutingTable(100),
		Priority:    priority,
		Action:      nethelpers.RoutingRuleActionUnicast,
		ConfigLayer: network.ConfigMachineConfiguration,
	}

	suite.Create(rule)

	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertRule(
					nethelpers.FamilyInet4,
					netip.Prefix{},
					netip.MustParsePrefix("192.168.0.0/16"),
					priority,
					func(r rtnetlink.RuleMessage) error {
						if r.Action != unix.FR_ACT_TO_TBL {
							return fmt.Errorf("unexpected action: got %d, want %d", r.Action, unix.FR_ACT_TO_TBL)
						}

						return nil
					},
				)
			},
		),
	)

	suite.Require().NoError(suite.State().TeardownAndDestroy(suite.Ctx(), rule.Metadata()))

	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertNoRule(
					nethelpers.FamilyInet4,
					netip.Prefix{},
					netip.MustParsePrefix("192.168.0.0/16"),
					priority,
				)
			},
		),
	)
}

// TestForeignRulePreserved verifies the controller never deletes rules it
// did not install (rules with a Protocol other than RTPROT_STATIC), even
// when their priority+family collides with a Talos-managed spec.
func (suite *RoutingRuleSpecSuite) TestForeignRulePreserved() {
	priority := uint32(31100)

	conn, err := rtnetlink.Dial(nil)
	suite.Require().NoError(err)

	defer conn.Close() //nolint:errcheck

	// Pre-install a foreign rule at the same priority+family with a
	// non-Talos protocol marker (RTPROT_BOOT). The controller's match
	// logic must ignore this rule and not delete it.
	srcIP := net.ParseIP("10.88.0.0").To4()
	foreignProto := uint8(unix.RTPROT_BOOT)
	foreignTable := uint32(200)
	foreignPriority := priority

	foreignMsg := &rtnetlink.RuleMessage{
		Family:    uint8(nethelpers.FamilyInet4),
		Table:     uint8(200),
		Action:    unix.FR_ACT_TO_TBL,
		SrcLength: 16,
		Attributes: &rtnetlink.RuleAttributes{
			Priority: &foreignPriority,
			Table:    &foreignTable,
			Src:      &srcIP,
			Protocol: &foreignProto,
		},
	}

	suite.Require().NoError(conn.Rule.Add(foreignMsg))

	defer func() {
		_ = conn.Rule.Delete(foreignMsg) //nolint:errcheck
	}()

	// Create a Talos-owned spec at the same priority+family but a different src.
	// Without the protocol gate the spec controller would key-match the foreign
	// rule on (priority, family) and delete it.
	rule := network.NewRoutingRuleSpec(network.NamespaceName, "foreign-collision-rule")
	*rule.TypedSpec() = network.RoutingRuleSpecSpec{
		Family:      nethelpers.FamilyInet4,
		Src:         netip.MustParsePrefix("10.77.0.0/16"),
		Table:       nethelpers.RoutingTable(100),
		Priority:    priority,
		Action:      nethelpers.RoutingRuleActionUnicast,
		ConfigLayer: network.ConfigMachineConfiguration,
	}

	suite.Create(rule)

	// Talos-owned rule must appear.
	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertRule(
					nethelpers.FamilyInet4,
					netip.MustParsePrefix("10.77.0.0/16"),
					netip.Prefix{},
					priority,
					func(rtnetlink.RuleMessage) error { return nil },
				)
			},
		),
	)

	// Foreign rule must still be present.
	rules, err := conn.Rule.List()
	suite.Require().NoError(err)

	var foreignFound bool

	for _, r := range rules {
		if r.Family != uint8(nethelpers.FamilyInet4) {
			continue
		}

		if pointer.SafeDeref(r.Attributes.Priority) != priority {
			continue
		}

		if pointer.SafeDeref(r.Attributes.Protocol) != unix.RTPROT_BOOT {
			continue
		}

		foreignFound = true

		break
	}

	suite.Assert().True(foreignFound, "foreign rule was deleted by the spec controller")

	// Tear the Talos-owned rule down — foreign rule must still survive.
	suite.Require().NoError(suite.State().TeardownAndDestroy(suite.Ctx(), rule.Metadata()))

	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertNoRule(
					nethelpers.FamilyInet4,
					netip.MustParsePrefix("10.77.0.0/16"),
					netip.Prefix{},
					priority,
				)
			},
		),
	)

	rules, err = conn.Rule.List()
	suite.Require().NoError(err)

	foreignFound = false

	for _, r := range rules {
		if r.Family != uint8(nethelpers.FamilyInet4) {
			continue
		}

		if pointer.SafeDeref(r.Attributes.Priority) != priority {
			continue
		}

		if pointer.SafeDeref(r.Attributes.Protocol) != unix.RTPROT_BOOT {
			continue
		}

		foreignFound = true

		break
	}

	suite.Assert().True(foreignFound, "foreign rule was deleted during teardown of Talos-owned rule")
}

func TestRoutingRuleSpecSuite(t *testing.T) {
	t.Parallel()

	if os.Geteuid() != 0 {
		t.Skip("requires root")
	}

	suite.Run(t, &RoutingRuleSpecSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 15 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&netctrl.RoutingRuleSpecController{}))
			},
		},
	})
}
