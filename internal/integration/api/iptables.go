// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"time"

	"github.com/siderolabs/talos/internal/integration/base"
)

// IPTablesSuite ...
type IPTablesSuite struct {
	base.APISuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *IPTablesSuite) SuiteName() string {
	return "api.IPTablesSuite"
}

// SetupTest ...
func (suite *IPTablesSuite) SetupTest() {
	// the timeout covers pulling the debug image on a cold node, which dominates the runtime
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 5*time.Minute)

	if !suite.Capabilities().RunsTalosKernel {
		suite.T().Skip("skipping kernel test since Talos kernel is not running")
	}

	if suite.SelinuxEnforcing {
		suite.T().Skip("skipping in SELinux enforcing mode: host namespace debug mode doesn't function")
	}
}

// TearDownTest ...
func (suite *IPTablesSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// xtablesProbeScript exercises the xt_* extensions that Cilium relies on through iptables-nft,
// for both address families.
//
// Regression test for the kernel config regression where CONFIG_IP_NF_IPTABLES and
// CONFIG_IP6_NF_IPTABLES were dropped: since Linux 6.16 those symbols no longer build the legacy
// ip_tables.ko/ip6_tables.ko (that moved to CONFIG_{IP,IP6}_NF_IPTABLES_LEGACY, gated on
// CONFIG_NETFILTER_XTABLES_LEGACY), they gate the per-family registration of the xt_* extensions
// that nft_compat resolves. Without them, ip6tables-nft fails with
// "Extension CT revision 0 not supported, missing kernel module?" even though nothing legacy is
// involved, and REJECT/rpfilter break on both families.
//
// Two details make the probes work:
//
//   - Rules go into a *user-defined* chain: nft_compat's nft_target_validate() only enforces the
//     extension's allowed hooks for base chains, so TPROXY (PREROUTING-only) can be tested without
//     hooking the node's live traffic.
//   - The table still matters: xt_check_target() compares the extension's .table against the nft
//     table name unconditionally, so CT must go into raw, TPROXY into mangle, REJECT into filter.
//
// Every probe runs even after an earlier one fails, so a partial regression names each affected
// extension rather than just the first.
const xtablesProbeScript = `
set -u

chain=TALOS-XT-PROBE
fail=0

iptables-nft --version
ip6tables-nft --version

probe() {
	bin=$1
	table=$2
	shift 2

	# drop a chain leaked by an earlier interrupted run before recreating it
	"$bin" -t "$table" -F "$chain" 2>/dev/null
	"$bin" -t "$table" -X "$chain" 2>/dev/null

	if ! "$bin" -t "$table" -N "$chain"; then
		echo "FAIL (setup): $bin -t $table -N $chain"
		fail=1

		return
	fi

	if "$bin" -t "$table" -A "$chain" "$@"; then
		echo "ok:   $bin -t $table $*"
	else
		echo "FAIL: $bin -t $table $*"
		fail=1
	fi

	"$bin" -t "$table" -F "$chain" 2>/dev/null
	"$bin" -t "$table" -X "$chain" 2>/dev/null
}

for bin in iptables-nft ip6tables-nft; do
	probe "$bin" raw    -j CT --notrack
	probe "$bin" mangle -j MARK --set-mark 1
	probe "$bin" mangle -p tcp -j TPROXY --on-port 1 --tproxy-mark 1/1
	probe "$bin" filter -j REJECT
	probe "$bin" raw    -m rpfilter -j ACCEPT
done

exit $fail
`

// TestLegacyXTables verifies that the legacy xtables extensions work via iptables in nftables mode.
func (suite *IPTablesSuite) TestLegacyXTables() {
	if !suite.Capabilities().RunsTalosKernel {
		suite.T().Skip("skipping kernel test since Talos kernel is not running")
	}

	node := suite.RandomDiscoveredNodeInternalIP()

	// this is a regression test mimicking operations done by Cilium
	out, exitCode := suite.RunDebugContainer(suite.ctx, node, "sh", "-c", xtablesProbeScript)

	suite.T().Logf("output:\n%s", out)
	suite.Assert().Zero(exitCode, "iptables-nft probes failed with exit code %d, output:\n%s", exitCode, out)
}

func init() {
	allSuites = append(allSuites, new(IPTablesSuite))
}
