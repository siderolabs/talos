// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"fmt"
	"net"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/blang/semver/v4"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-retry/retry"
	"google.golang.org/grpc/codes"

	"github.com/siderolabs/talos/internal/integration/base"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/etcd"
	k8sres "github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// kubePrismEtcdIsolationScript is a state machine which blocks the connections of the kube-apiserver of the
// target control plane node to the etcd client port and waits until KubePrism serves a burst of new
// connections successfully while the target API server does not pass its /readyz check, but still accepts
// TCP connections.
//
// Only the client connections of kube-apiserver are blocked, so etcd members, quorum and leadership are not
// affected and nothing has to wait for etcd to recover. The observation is conclusive only while the TCP
// listener of the target stays open: once it closes, the old TCP-only health check would exclude the
// upstream as well. An attempt is also rejected when the upstream is excluded sooner than a fully healthy
// upstream can possibly be excluded, which means its score had not recovered from a previous failure.
// KubePrism does not expose per-upstream scores, so the settle time before every attempt is derived from
// the KubePrism score and health check constants.
//
// States: INIT -> AWAIT_READY -> SETTLE -> FAULT_ON -> AWAIT_NOT_READY -> MEASURE -> FAULT_OFF -> PASS | FAIL | AWAIT_READY.
const kubePrismEtcdIsolationScript = `
set -eu

log_step() {
        printf '[kubeprism-e2e] %%s\n' "$*"
}

bash=
curl=
table=%s
api_url=https://%s/readyz
kubeprism_url=https://127.0.0.1:%d/readyz
api_port=%d
etcd_client_port=%d
apiserver_uid=%d
settle_seconds=%d
min_exclusion_seconds=%d
response_status=
isolated=0

max_attempts=3
burst_size=10
fault_timeout=60
observe_timeout=150
recovery_timeout=300

state=INIT
attempt=0
verdict=
reason=
fault_started=0
not_ready_at=-1
excluded_by=-1

goto() {
        log_step "STATE $state -> $1: $2"
        state=$1
}

request_http_status() {
        url=$1
        log_step "requesting $url"

        if response_status=$("$curl" --silent --insecure --max-time 2 -o /dev/null -w '%%{http_code}' "$url" 2>/dev/null); then
                log_step "request to $url returned HTTP $response_status"

                return 0
        else
                status=$?
                response_status=000
                log_step "request to $url failed with curl exit code $status"

                return 1
        fi
}

request_ready() {
        request_http_status "$1" && [ "$response_status" = 200 ]
}

api_tcp_alive() {
        "$bash" -c 'exec 3<>/dev/tcp/127.0.0.1/$1' _ "$api_port" >/dev/null 2>&1
}

explain_request_failure() {
        log_step "collecting verbose failure details for $1"

        if "$curl" --verbose --insecure --max-time 5 -o /dev/null "$1" 2>&1; then
                log_step "verbose diagnostic request unexpectedly succeeded for $1"
        else
                status=$?
                log_step "verbose diagnostic request exited with code $status for $1"
        fi
}

pause() {
        read -r -t "$1" _ </dev/zero || true
}

await_ready() {
        url=$1
        limit=$2
        waited_from=$SECONDS

        while :; do
                if request_ready "$url"; then
                        return 0
                fi

                if [ $((SECONDS - waited_from)) -ge "$limit" ]; then
                        return 1
                fi

                pause 2
        done
}

settle() {
        settle_from=$SECONDS
        log_step "SETTLE: waiting ${settle_seconds}s for KubePrism upstream scores to recover after earlier failures"

        while [ $((SECONDS - settle_from)) -lt "$settle_seconds" ]; do
                pause 5
                log_step "SETTLE: $((SECONDS - settle_from))s of ${settle_seconds}s elapsed"
        done
}

run_nft() {
        log_step "running nft $*"

        if nft "$@"; then
                log_step "nft command succeeded"
        else
                status=$?
                log_step "nft command failed with exit code $status"

                return "$status"
        fi
}

restore() {
        if [ "$isolated" -eq 0 ]; then
                return 0
        fi

        log_step "removing nftables table inet $table"
        if nft delete table inet "$table"; then
                isolated=0
                log_step "removed nftables table inet $table"
        else
                status=$?
                log_step "failed to remove nftables table inet $table (exit code $status)"
        fi
}

isolate() {
        log_step "ACTION: blocking connections of kube-apiserver (uid $apiserver_uid) to the etcd client port $etcd_client_port"
        run_nft add table inet "$table"
        isolated=1
        run_nft add chain inet "$table" output '{ type filter hook output priority -200; policy accept; }'
        run_nft add rule inet "$table" output meta skuid "$apiserver_uid" tcp dport "$etcd_client_port" drop
        log_step "ACTION complete: kube-apiserver cannot reach etcd"
}

kubeprism_burst() {
        n=0

        while [ "$n" -lt "$burst_size" ]; do
                if ! request_ready "$kubeprism_url"; then
                        log_step "KubePrism request $((n + 1))/$burst_size did not return HTTP 200"

                        return 1
                fi

                n=$((n + 1))
        done

        return 0
}

st_init() {
        log_step "checking availability of nft"
        if command -v nft; then
                log_step "nft is available"
        else
                reason="nft is not available"
                goto FAIL "$reason"

                return 0
        fi

        log_step "locating curl and bash in the Nix store"
        for candidate in /nix/store/*-curl-*/bin/curl; do
                if test -x "$candidate"; then
                        curl=$candidate
                        break
                fi
        done
        for candidate in /nix/store/*-bash-*/bin/bash; do
                if test -x "$candidate"; then
                        bash=$candidate
                        break
                fi
        done

        if ! test -x "$curl" || ! test -x "$bash"; then
                reason="curl or bash is missing in the debug image"
                goto FAIL "$reason"

                return 0
        fi

        goto AWAIT_READY "tools are available"
}

st_await_ready() {
        if ! await_ready "$api_url" "$recovery_timeout"; then
                reason="the target API did not become ready within ${recovery_timeout}s"
                explain_request_failure "$api_url"
                goto FAIL "$reason"

                return 0
        fi

        if ! await_ready "$kubeprism_url" "$recovery_timeout"; then
                reason="KubePrism did not become ready within ${recovery_timeout}s"
                explain_request_failure "$kubeprism_url"
                goto FAIL "$reason"

                return 0
        fi

        goto SETTLE "the target API and KubePrism are ready"
}

st_settle() {
        settle

        if ! request_ready "$api_url" || ! api_tcp_alive || ! request_ready "$kubeprism_url"; then
                reason="the target API, its TCP listener or KubePrism is not healthy after the settle period"
                goto FAIL "$reason"

                return 0
        fi

        goto FAULT_ON "preconditions hold: the target API is ready and listening, KubePrism is ready"
}

st_fault_on() {
        attempt=$((attempt + 1))
        log_step "ATTEMPT $attempt/$max_attempts"

        isolate
        fault_started=$SECONDS
        not_ready_at=-1

        goto AWAIT_NOT_READY "the fault is installed"
}

st_await_not_ready() {
        while [ $((SECONDS - fault_started)) -lt "$fault_timeout" ]; do
                if ! api_tcp_alive; then
                        verdict=retry
                        reason="the target API TCP listener closed before the API reported not ready"
                        goto FAULT_OFF "$reason"

                        return 0
                fi

                request_http_status "$api_url" || true

                if [ "$response_status" != 200 ]; then
                        not_ready_at=$((SECONDS - fault_started))
                        goto MEASURE "the target API reports HTTP $response_status after ${not_ready_at}s while its TCP listener is open"

                        return 0
                fi

                pause 1
        done

        verdict=fail
        reason="the fault had no effect: the target API stayed ready for ${fault_timeout}s"
        goto FAULT_OFF "$reason"
}

st_measure() {
        measure_from=$SECONDS
        bursts=0
        failed_bursts=0

        while [ $((SECONDS - measure_from)) -lt "$observe_timeout" ]; do
                if ! api_tcp_alive; then
                        verdict=retry
                        reason="the target API TCP listener closed at +$((SECONDS - fault_started))s before the exclusion was observed (bursts: $bursts, failed bursts: $failed_bursts)"
                        goto FAULT_OFF "$reason"

                        return 0
                fi

                request_http_status "$api_url" || true

                if [ "$response_status" = 200 ]; then
                        verdict=retry
                        reason="the target API became ready again at +$((SECONDS - fault_started))s although the fault is still installed"
                        goto FAULT_OFF "$reason"

                        return 0
                fi

                bursts=$((bursts + 1))

                if kubeprism_burst; then
                        request_http_status "$api_url" || true

                        if [ "$response_status" != 200 ] && api_tcp_alive; then
                                excluded_by=$((SECONDS - fault_started))

                                log_step "RESULT: not_ready_at=${not_ready_at}s excluded_by=${excluded_by}s min_exclusion=${min_exclusion_seconds}s listener_alive_through_burst=yes bursts=$bursts failed_bursts=$failed_bursts burst_size=$burst_size"

                                if [ "$excluded_by" -lt "$min_exclusion_seconds" ]; then
                                        verdict=retry
                                        reason="the upstream was excluded in ${excluded_by}s, faster than the ${min_exclusion_seconds}s needed from a fully healthy state"
                                        goto FAULT_OFF "$reason"

                                        return 0
                                fi

                                verdict=pass
                                goto FAULT_OFF "the exclusion is observed"

                                return 0
                        fi

                        verdict=retry
                        reason="the target API changed state during the burst"
                        goto FAULT_OFF "$reason"

                        return 0
                fi

                failed_bursts=$((failed_bursts + 1))
                log_step "KubePrism did not complete the burst ($failed_bursts failed so far); sampling again"

                pause 1
        done

        verdict=fail
        reason="KubePrism did not serve $burst_size consecutive new connections within ${observe_timeout}s while the target API was not ready and listening (bursts: $bursts, failed bursts: $failed_bursts)"
        goto FAULT_OFF "$reason"
}

st_fault_off() {
        restore

        case "$verdict" in
        pass)
                goto PASS "the fault is removed"
                ;;
        retry)
                if [ "$attempt" -lt "$max_attempts" ]; then
                        log_step "attempt $attempt/$max_attempts is inconclusive: $reason"
                        verdict=
                        goto AWAIT_READY "retrying after the fault is removed"
                else
                        reason="no conclusive attempt in $max_attempts attempts; last: $reason"
                        goto FAIL "$reason"
                fi
                ;;
        *)
                goto FAIL "$reason"
                ;;
        esac
}

trap restore EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

log_step "starting the kube-apiserver to etcd client port isolation scenario"

while :; do
        case "$state" in
        INIT) st_init ;;
        AWAIT_READY) st_await_ready ;;
        SETTLE) st_settle ;;
        FAULT_ON) st_fault_on ;;
        AWAIT_NOT_READY) st_await_not_ready ;;
        MEASURE) st_measure ;;
        FAULT_OFF) st_fault_off ;;
        PASS)
                log_step "POSTCONDITION passed on attempt $attempt/$max_attempts: KubePrism served $burst_size new connections with HTTP 200 while the isolated target was not ready and still accepted TCP connections"
                exit 0
                ;;
        FAIL)
                log_step "POSTCONDITION failed in the state machine: $reason"
                exit 1
                ;;
        *)
                log_step "unknown state $state"
                exit 1
                ;;
        esac
done
`

const kubePrismEtcdRecoveryScript = `
set -eu

log_step() {
        printf '[kubeprism-e2e] %%s\n' "$*"
}

bash=
curl=
table=%s
api_url=https://%s/readyz
kubeprism_url=https://127.0.0.1:%d/readyz
api_port=%d
required_successes=10
response_status=

request_http_status() {
        url=$1
        log_step "requesting $url"

        if response_status=$("$curl" --silent --insecure --max-time 2 -o /dev/null -w '%%{http_code}' "$url" 2>/dev/null); then
                log_step "request to $url returned HTTP $response_status"

                return 0
        else
                status=$?
                response_status=000
                log_step "request to $url failed with curl exit code $status"

                return 1
        fi
}

request_ready() {
        request_http_status "$1" && [ "$response_status" = 200 ]
}

api_tcp_alive() {
        "$bash" -c 'exec 3<>/dev/tcp/127.0.0.1/$1' _ "$api_port" >/dev/null 2>&1
}

pause() {
        read -r -t "$1" _ </dev/zero || true
}

log_step "PRECONDITION: confirming etcd peer isolation table inet $table was removed"
if nft list table inet "$table" >/dev/null 2>&1; then
        log_step "PRECONDITION failed: isolation table inet $table still exists"
        exit 1
fi
log_step "PRECONDITION passed: etcd peer isolation has been removed"

log_step "locating curl and bash in the Nix store"
for candidate in /nix/store/*-curl-*/bin/curl; do
        if test -x "$candidate"; then
                curl=$candidate
                break
        fi
done
for candidate in /nix/store/*-bash-*/bin/bash; do
        if test -x "$candidate"; then
                bash=$candidate
                break
        fi
done

if ! test -x "$curl" || ! test -x "$bash"; then
        log_step "PRECONDITION failed: curl or bash is unavailable"
        exit 1
fi

log_step "ACTION: waiting for the target API and KubePrism to recover after etcd became healthy"
i=0
successes=0
while [ "$i" -lt 120 ] && [ "$successes" -lt "$required_successes" ]; do
        log_step "recovery poll $((i + 1))/120; consecutive successes: $successes/$required_successes"
        if request_ready "$api_url" && api_tcp_alive && request_ready "$kubeprism_url"; then
                successes=$((successes + 1))
                log_step "both target API and KubePrism returned HTTP 200; consecutive successes: $successes/$required_successes"
        else
                successes=0
                log_step "recovery readiness is not yet stable; resetting the consecutive success count"
        fi

        i=$((i + 1))
        if [ "$successes" -lt "$required_successes" ]; then
                pause 1
        fi
done

if [ "$successes" -lt "$required_successes" ]; then
        log_step "POSTCONDITION failed: target API and KubePrism did not both return HTTP 200 for $required_successes consecutive polls"
        exit 1
fi

log_step "POSTCONDITION passed: target API and KubePrism recovered and returned HTTP 200 for $required_successes consecutive polls"
`

// EtcdSuite ...
type EtcdSuite struct {
	base.K8sSuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *EtcdSuite) SuiteName() string {
	return "api.EtcdSuite"
}

// SetupTest ...
func (suite *EtcdSuite) SetupTest() {
	// make sure we abort at some point in time, but give enough room for Etcds
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 30*time.Minute)
}

// TearDownTest ...
func (suite *EtcdSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestForfeitLeadership tests moving etcd leadership to another member.
func (suite *EtcdSuite) TestForfeitLeadership() {
	if suite.Cluster == nil {
		suite.T().Skip("without full cluster state etcd test is not reliable (can't wait for cluster readiness in between resets)")
	}

	nodes := suite.DiscoverNodeInternalIPsByType(suite.ctx, machine.TypeControlPlane)

	if len(nodes) < 3 {
		suite.T().Skip("test only can be run on HA etcd clusters")
	}

	var leader string

	for _, node := range nodes {
		resp, err := suite.Client.EtcdForfeitLeadership(
			client.WithNode(suite.ctx, node),
			&machineapi.EtcdForfeitLeadershipRequest{},
		)
		suite.Require().NoError(err)

		if resp.Messages[0].GetMember() != "" {
			leader = resp.Messages[0].GetMember()

			suite.T().Log("Moved leadership to", leader)
		}
	}

	suite.Assert().NotEmpty(leader)
}

// TestLeaveCluster tests removing an etcd member.
//
//nolint:gocyclo
func (suite *EtcdSuite) TestLeaveCluster() {
	if testing.Short() {
		suite.T().Skip("skipping in short mode")
	}

	if !suite.Capabilities().SupportsReboot {
		suite.T().Skip("cluster doesn't support reboot (and reset)")
	}

	if suite.Cluster == nil {
		suite.T().Skip("without full cluster state reset test is not reliable (can't wait for cluster readiness in between resets)")
	}

	nodes := suite.DiscoverNodeInternalIPsByType(suite.ctx, machine.TypeControlPlane)

	if len(nodes) < 3 {
		suite.T().Skip("test only can be run on HA etcd clusters")
	}

	node := nodes[len(nodes)-1]

	suite.T().Log("Removing etcd member", node)

	nodeCtx := client.WithNode(suite.ctx, node)

	_, err := suite.Client.EtcdForfeitLeadership(nodeCtx, &machineapi.EtcdForfeitLeadershipRequest{})
	suite.Require().NoError(err)

	err = suite.Client.EtcdLeaveCluster(nodeCtx, &machineapi.EtcdLeaveClusterRequest{})
	suite.Require().NoError(err)

	services, err := suite.Client.ServiceInfo(nodeCtx, "etcd")
	suite.Require().NoError(err)

	for _, service := range services {
		if service.Service.Id == "etcd" {
			suite.Assert().Equal("Finished", service.Service.State)
		}
	}

	// LeaveCluster nukes the etcd data in place; /var/lib/etcd itself remains (it may be a
	// dedicated volume mount point), so assert the data (member directory) is gone rather than
	// the whole path.
	stream, err := suite.Client.MachineClient.List(nodeCtx, &machineapi.ListRequest{Root: filepath.Join(constants.EtcdDataPath, "member")})
	suite.Require().NoError(err)

	_, err = stream.Recv()
	suite.Require().Error(err)
	suite.Require().Equal(client.StatusCode(err), codes.Unknown)
	suite.Require().Contains(client.Status(err).Message(), "no such file or directory")

	// NB: Reboot the node so that it can rejoin the etcd cluster. This allows us
	// to check the cluster health and catch any issues in rejoining.
	suite.AssertRebooted(
		suite.ctx, node, func(nodeCtx context.Context) error {
			_, err = suite.Client.MachineClient.Reboot(nodeCtx, &machineapi.RebootRequest{})

			return err
		}, 10*time.Minute,
		suite.CleanupFailedPods,
	)
}

// TestMembers verifies that etcd members as resources and API response are consistent.
func (suite *EtcdSuite) TestMembers() {
	nodes := suite.DiscoverNodeInternalIPsByType(suite.ctx, machine.TypeControlPlane)

	// map member ID to hostname
	etcdMembers := map[string]string{}

	for _, node := range nodes {
		member, err := safe.StateGet[*etcd.Member](client.WithNode(suite.ctx, node), suite.Client.COSI, etcd.NewMember(etcd.NamespaceName, etcd.LocalMemberID).Metadata())
		suite.Require().NoError(err)

		hostname, err := safe.StateGet[*network.HostnameStatus](client.WithNode(suite.ctx, node), suite.Client.COSI, network.NewHostnameStatus(network.NamespaceName, network.HostnameID).Metadata())
		suite.Require().NoError(err)

		etcdMembers[member.TypedSpec().MemberID] = hostname.TypedSpec().Hostname
	}

	suite.Assert().Len(etcdMembers, len(nodes))

	resp, err := suite.Client.EtcdMemberList(suite.ctx, &machineapi.EtcdMemberListRequest{})
	suite.Require().NoError(err)

	count := 0

	for _, message := range resp.GetMessages() {
		for _, member := range message.GetMembers() {
			count++

			memberID := etcd.FormatMemberID(member.GetId())

			suite.Assert().Contains(etcdMembers, memberID)
			suite.Assert().Equal(etcdMembers[memberID], member.GetHostname())
		}
	}

	suite.Assert().Equal(len(etcdMembers), count)
}

// TestKubePrismExcludesEtcdIsolatedUpstream verifies that KubePrism stops routing new
// connections to an API server which fails its readiness check after losing its connection
// to etcd, while the TCP listener of the API server is still accepting connections.
// Only the client connections of kube-apiserver to etcd are blocked, etcd itself stays healthy.
func (suite *EtcdSuite) TestKubePrismExcludesEtcdIsolatedUpstream() {
	if testing.Short() {
		suite.T().Skip("skipping in short mode")
	}

	if suite.Cluster == nil {
		suite.T().Skip("test requires provisioned cluster state to verify recovery")
	}

	if !suite.Capabilities().RunsTalosKernel {
		suite.T().Skip("test requires the Talos kernel for nftables fault injection")
	}

	if suite.SelinuxEnforcing {
		suite.T().Skip("host namespace debug mode does not function in SELinux enforcing mode")
	}

	if !suite.SupportsEtcd() {
		suite.T().Skip("cluster does not run etcd")
	}

	nodes := suite.DiscoverNodeInternalIPsByType(suite.ctx, machine.TypeControlPlane)
	if len(nodes) < 3 {
		suite.T().Skip("test only can be run on HA etcd clusters")
	}

	// The scenario waits for the KubePrism upstream scores to settle before every attempt on every node,
	// which takes longer than the default time budget of a test.
	suite.ctxCancel()
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 60*time.Minute)

	suite.AssertClusterHealthy(suite.ctx)

	for _, node := range nodes {
		suite.Run(node, func() {
			suite.testKubePrismExcludesEtcdIsolatedUpstreamOnNode(node)
		})
	}
}

func (suite *EtcdSuite) testKubePrismExcludesEtcdIsolatedUpstreamOnNode(targetNode string) {
	nodeCtx := client.WithNode(suite.ctx, targetNode)

	status, err := safe.StateGetByID[*k8sres.KubePrismStatuses](nodeCtx, suite.Client.COSI, k8sres.KubePrismStatusesID)
	if state.IsNotFoundError(err) {
		suite.T().Skip("KubePrism is not enabled on this control-plane node")
	}
	suite.Require().NoError(err)
	if !status.TypedSpec().Healthy {
		suite.T().Skip("KubePrism is not healthy on this control-plane node")
	}

	_, port, err := net.SplitHostPort(status.TypedSpec().Host)
	suite.Require().NoError(err, "invalid KubePrism endpoint %q", status.TypedSpec().Host)

	kubePrismPort, err := strconv.Atoi(port)
	suite.Require().NoError(err)

	table := fmt.Sprintf("talos_kubeprism_e2e_%x", time.Now().UnixNano())

	// An upstream with the highest score is excluded after HighScore+1 failed health checks, so it cannot be
	// excluded sooner than HighScore health check intervals after its first failed check. After an earlier
	// isolation the score has to climb from the lowest to the highest value again, one step per health check,
	// before the exclusion time can be measured from a fully healthy state.
	settle := time.Duration(constants.KubePrismHighScore-constants.KubePrismLowScore+1) * constants.KubePrismHealthCheckInterval
	minExclusion := time.Duration(constants.KubePrismHighScore) * constants.KubePrismHealthCheckInterval

	script := fmt.Sprintf(
		kubePrismEtcdIsolationScript,
		table,
		net.JoinHostPort(targetNode, strconv.Itoa(constants.DefaultControlPlanePort)),
		kubePrismPort,
		constants.DefaultControlPlanePort,
		constants.EtcdClientPort,
		constants.KubernetesAPIServerRunUser,
		int(settle/time.Second),
		int(minExclusion/time.Second),
	)

	out, exitCode := suite.RunDebugContainerWithImageAndLiveOutput(
		suite.ctx,
		targetNode,
		"nixery.dev/shell/curl",
		"/nix/store/2ndah67h0z5m31v2wkdmg2md4380ggr5-bash-interactive-5.3p15/bin/bash",
		"-c",
		script,
	)

	suite.T().Log("ACTION: isolation was removed; waiting for etcd and cluster health before recovery checks")
	suite.AssertClusterHealthy(suite.ctx)

	recoveryScript := fmt.Sprintf(
		kubePrismEtcdRecoveryScript,
		table,
		net.JoinHostPort(targetNode, strconv.Itoa(constants.DefaultControlPlanePort)),
		kubePrismPort,
		constants.DefaultControlPlanePort,
	)

	recoveryOut, recoveryExitCode := suite.RunDebugContainerWithImageAndLiveOutput(
		suite.ctx,
		targetNode,
		"nixery.dev/shell/curl",
		"/nix/store/2ndah67h0z5m31v2wkdmg2md4380ggr5-bash-interactive-5.3p15/bin/bash",
		"-c",
		recoveryScript,
	)

	suite.Assert().Zero(exitCode, "KubePrism did not exclude the etcd-isolated upstream:\n%s", out)
	suite.Assert().Zero(recoveryExitCode, "KubePrism upstream did not recover after etcd isolation was removed:\n%s", recoveryOut)
}

// TestRemoveMember tests removing an etcd member forcefully.
func (suite *EtcdSuite) TestRemoveMember() {
	if testing.Short() {
		suite.T().Skip("skipping in short mode")
	}

	if !suite.Capabilities().SupportsReboot {
		suite.T().Skip("cluster doesn't support reboot (and reset)")
	}

	if suite.Cluster == nil {
		suite.T().Skip("without full cluster state reset test is not reliable (can't wait for cluster readiness in between resets)")
	}

	nodes := suite.DiscoverNodeInternalIPsByType(suite.ctx, machine.TypeControlPlane)

	if len(nodes) < 3 {
		suite.T().Skip("test only can be run on HA etcd clusters")
	}

	controlNode, nodeToRemove := nodes[len(nodes)-1], nodes[0]

	suite.T().Log("Removing etcd member", nodeToRemove)

	removeCtx := client.WithNode(suite.ctx, nodeToRemove)
	controlCtx := client.WithNode(suite.ctx, controlNode)

	_, err := suite.Client.EtcdForfeitLeadership(removeCtx, &machineapi.EtcdForfeitLeadershipRequest{})
	suite.Require().NoError(err)

	member, err := safe.StateGet[*etcd.Member](removeCtx, suite.Client.COSI, etcd.NewMember(etcd.NamespaceName, etcd.LocalMemberID).Metadata())
	suite.Require().NoError(err)

	memberID, err := etcd.ParseMemberID(member.TypedSpec().MemberID)
	suite.Require().NoError(err)

	err = suite.Client.EtcdRemoveMemberByID(controlCtx, &machineapi.EtcdRemoveMemberByIDRequest{
		MemberId: memberID,
	})
	suite.Require().NoError(err)

	// Verify that memberID disappeared from the etcd member list.
	//
	// This is retried because the control node builds an etcd client across all control plane
	// IPs and health-checks each by dialing: right after the removed member is forfeited/removed,
	// its etcd can reset the connection, surfacing as a transient "connection reset by peer"
	// (gRPC Unavailable) while building the client. Re-list until the member is gone.
	suite.Require().NoError(
		retry.Constant(time.Second*5, retry.WithUnits(200*time.Millisecond)).RetryWithContext(
			controlCtx,
			func(ctx context.Context) error {
				resp, err := suite.Client.EtcdMemberList(ctx, &machineapi.EtcdMemberListRequest{})
				if err != nil {
					return retry.ExpectedError(err)
				}

				for _, message := range resp.GetMessages() {
					for _, member := range message.GetMembers() {
						if member.GetId() == memberID {
							return retry.ExpectedErrorf("member %d still present in etcd member list", memberID)
						}
					}
				}

				return nil
			},
		),
	)

	// Wipe etcd data so the removed node rejoins the cluster fresh. If ETCD is directory-backed it
	// lives under EPHEMERAL (wipe EPHEMERAL); if it is a dedicated partition, wipe that partition
	// directly and leave EPHEMERAL intact.
	etcdVolumeStatus, err := safe.StateGetByID[*block.VolumeStatus](removeCtx, suite.Client.COSI, constants.EtcdDataVolumeID)
	suite.Require().NoError(err)

	partitionsToWipe := []*machineapi.ResetPartitionSpec{{Label: constants.EtcdDataVolumeID, Wipe: true}}
	if etcdVolumeStatus.TypedSpec().Type == block.VolumeTypeDirectory {
		partitionsToWipe = []*machineapi.ResetPartitionSpec{{Label: constants.EphemeralPartitionLabel, Wipe: true}}
	}

	// NB: Reset the node so that it can rejoin the etcd cluster. This allows us
	// to check the cluster health and catch any issues in rejoining.
	suite.AssertRebooted(
		suite.ctx, nodeToRemove, func(nodeCtx context.Context) error {
			_, err = suite.Client.MachineClient.Reset(nodeCtx, &machineapi.ResetRequest{
				Reboot:                 true,
				SystemPartitionsToWipe: partitionsToWipe,
			})

			return err
		}, 10*time.Minute,
		suite.CleanupFailedPods,
	)
}

func (suite *EtcdSuite) downgradesSupported() bool {
	suite.T().Helper()

	downgradeFromAtLeast := semver.Version{Major: 3, Minor: 6}

	status, err := suite.Client.EtcdStatus(suite.ctx)
	suite.Require().NoError(err)

	for _, message := range status.Messages {
		etcdVersion := message.GetMemberStatus().GetProtocolVersion()

		etcdVersionSemver, err := semver.Parse(etcdVersion)
		if err != nil {
			suite.T().Logf("ETCD version parsing failed: %s", err)

			continue
		}

		if etcdVersionSemver.LT(downgradeFromAtLeast) {
			return false
		}
	}

	return true
}

// TestDowngrade tests downgrade API of etcd.
func (suite *EtcdSuite) TestDowngrade() {
	if testing.Short() {
		suite.T().Skip("skipping in short mode")
	}

	if !suite.Capabilities().SupportsReboot {
		suite.T().Skip("cluster doesn't support reboot (and reset)")
	}

	if !suite.downgradesSupported() {
		suite.T().Skip("cluster needs to run ETCD 3.6.x to support downgrades")
	}

	nodes := suite.DiscoverNodeInternalIPsByType(suite.ctx, machine.TypeControlPlane)
	node := nodes[0]
	downgradeTo := "3.6"

	// test the downgrade validate API
	validateResp, err := suite.Client.EtcdDowngradeValidate(
		client.WithNode(suite.ctx, node),
		&machineapi.EtcdDowngradeValidateRequest{Version: downgradeTo},
	)
	suite.Require().NoError(err)

	for _, message := range validateResp.GetMessages() {
		suite.Assert().NotEqual(downgradeTo, message.GetClusterDowngrade().GetClusterVersion())
	}

	// test the downgrade enable API
	enableResp, err := suite.Client.EtcdDowngradeEnable(
		client.WithNode(suite.ctx, node),
		&machineapi.EtcdDowngradeEnableRequest{Version: downgradeTo},
	)
	suite.Require().NoError(err)

	for _, message := range enableResp.GetMessages() {
		suite.Assert().NotEqual(downgradeTo, message.GetClusterDowngrade().GetClusterVersion())
	}

	// test the downgrade cancel API
	cancelResp, err := suite.Client.EtcdDowngradeCancel(
		client.WithNode(suite.ctx, node),
	)
	suite.Require().NoError(err)

	for _, message := range cancelResp.GetMessages() {
		suite.Assert().NotEqual(downgradeTo, message.GetClusterDowngrade().GetClusterVersion())
	}
}

func init() {
	allSuites = append(allSuites, new(EtcdSuite))
}
