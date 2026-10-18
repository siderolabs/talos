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

// kubePrismEtcdIsolationScript isolates etcd peer traffic of the target control plane node and
// waits until KubePrism serves a burst of new connections successfully while the target API server
// does not pass its /readyz check.
//
// The target is considered unavailable on any response other than HTTP 200, including refused
// connections: once the API server stops listening, the old TCP-only health check would also
// exclude it, so the scenario does not require the TCP listener to stay open.
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
etcd_peer_port=%d
api_port=%d
response_status=
isolated=0

burst_size=10
observe_timeout=240

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
        log_step "ACTION: isolating etcd peer traffic"
        run_nft add table inet "$table"
        isolated=1
        run_nft add chain inet "$table" input '{ type filter hook input priority -200; policy accept; }'
        run_nft add chain inet "$table" output '{ type filter hook output priority -200; policy accept; }'
        run_nft add rule inet "$table" input iifname != lo tcp dport "$etcd_peer_port" drop
        run_nft add rule inet "$table" input iifname != lo tcp sport "$etcd_peer_port" drop
        run_nft add rule inet "$table" output oifname != lo tcp dport "$etcd_peer_port" drop
        run_nft add rule inet "$table" output oifname != lo tcp sport "$etcd_peer_port" drop
        log_step "ACTION complete: etcd peer traffic is isolated"
}

check_preconditions() {
        log_step "PRECONDITION: target API /readyz returns HTTP 200"
        if ! request_ready "$api_url"; then
                echo "control-plane API is not ready before isolation"
                explain_request_failure "$api_url"

                return 1
        fi

        log_step "PRECONDITION: target API TCP listener is available"
        if ! api_tcp_alive; then
                log_step "target API TCP listener is not available before isolation"

                return 1
        fi

        log_step "PRECONDITION: KubePrism /readyz returns HTTP 200"
        if ! request_ready "$kubeprism_url"; then
                echo "KubePrism is not serving ready upstreams before isolation"
                explain_request_failure "$kubeprism_url"

                return 1
        fi

        return 0
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

observe_isolation() {
        started=$SECONDS
        not_ready_at=-1
        listener_down_at=-1
        bursts=0
        failed_bursts=0

        while [ $((SECONDS - started)) -lt "$observe_timeout" ]; do
                if [ "$listener_down_at" -lt 0 ] && ! api_tcp_alive; then
                        listener_down_at=$((SECONDS - started))
                        log_step "target API TCP listener went down at +${listener_down_at}s"
                fi

                request_http_status "$api_url" || true

                if [ "$response_status" = 200 ]; then
                        log_step "target API is still ready at +$((SECONDS - started))s"
                        pause 1

                        continue
                fi

                if [ "$not_ready_at" -lt 0 ]; then
                        not_ready_at=$((SECONDS - started))
                        log_step "target API /readyz is not ready (HTTP $response_status) at +${not_ready_at}s"
                fi

                bursts=$((bursts + 1))

                if api_tcp_alive; then
                        alive_before_burst=yes
                else
                        alive_before_burst=no
                fi

                if kubeprism_burst; then
                        request_http_status "$api_url" || true

                        if [ "$response_status" != 200 ]; then
                                # "yes" means the TCP listener of the target was open before and after the burst,
                                # so only the L7 readiness check can explain the exclusion of this upstream.
                                if [ "$alive_before_burst" = yes ] && api_tcp_alive; then
                                        listener_alive_through_burst=yes
                                else
                                        listener_alive_through_burst=no
                                fi

                                log_step "RESULT: not_ready_at=${not_ready_at}s excluded_by=$((SECONDS - started))s listener_down_at=${listener_down_at}s listener_alive_through_burst=$listener_alive_through_burst bursts=$bursts failed_bursts=$failed_bursts burst_size=$burst_size"

                                return 0
                        fi

                        log_step "target API became ready during the burst; sampling again"
                else
                        failed_bursts=$((failed_bursts + 1))
                        log_step "KubePrism did not complete the burst ($failed_bursts failed so far); sampling again"
                fi

                pause 1
        done

        log_step "window not found: observation timeout after ${observe_timeout}s (not_ready_at=${not_ready_at}s, listener_down_at=${listener_down_at}s, -1 means never observed; bursts: $bursts, failed bursts: $failed_bursts)"

        return 1
}
		
trap restore EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

log_step "starting etcd peer isolation scenario on port $etcd_peer_port"
log_step "checking availability of nft"
if command -v nft; then
        log_step "nft is available"
else
        log_step "nft is not available"
        exit 1
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

if ! test -x "$curl"; then
        log_step "curl executable is missing or not executable"
        exit 1
fi

if ! test -x "$bash"; then
        log_step "bash executable is missing or not executable"
        exit 1
fi

check_preconditions || exit 1

isolate

if observe_isolation; then
        log_step "POSTCONDITION passed: KubePrism served $burst_size new connections with HTTP 200 while the isolated target was not ready"
        exit 0
fi

log_step "POSTCONDITION failed: KubePrism did not serve $burst_size consecutive new connections with HTTP 200 within ${observe_timeout}s while the isolated target was not ready"
exit 1
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
// connections to an API server which fails its readiness check after losing etcd peer
// connectivity, whether or not its TCP listener is still accepting connections.
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
	script := fmt.Sprintf(
		kubePrismEtcdIsolationScript,
		table,
		net.JoinHostPort(targetNode, strconv.Itoa(constants.DefaultControlPlanePort)),
		kubePrismPort,
		constants.EtcdPeerPort,
		constants.DefaultControlPlanePort,
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
