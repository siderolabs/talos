// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/ensure"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/siderolabs/talos/internal/integration/base"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	clientconfig "github.com/siderolabs/talos/pkg/machinery/client/config"
	configmachine "github.com/siderolabs/talos/pkg/machinery/config/machine"
	configmeta "github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	confignetwork "github.com/siderolabs/talos/pkg/machinery/config/types/network"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/redactable"
	networkres "github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
	"github.com/siderolabs/talos/pkg/machinery/role"
)

// The probe is the oracle for redaction: the credentials live in the spec of the ProbeSpec resource
// it produces, while ProbeSpecSpec.ID() strips them out of the resource ID, so whether the password
// comes back is decided by redaction alone.
//
// ProbeSpec is a sensitive type registered as redactable, so whenever it is readable at all, it also
// says whether redaction ran.
//
// example.com is used because the probe has to succeed: a failing probe would flip
// NetworkStatus.ConnectivityReady on the node and upset the rest of the run.
const (
	redactionProbeUser     = "talos-redaction-probe"
	redactionProbePassword = "top-secret-probe-password" //nolint:gosec // not a credential, an oracle
	redactionProbeHost     = "example.com"
	// the ID is built from the URL with the user info stripped, so it never carries the password.
	redactionProbeID = "http:https://" + redactionProbeHost
)

// RedactionSuite verifies the resource redaction access model over the real API.
//
// It walks the decision table documented on redactable.Header: for every role and every redaction
// mode, it checks whether a resource of each sensitivity class can be read at all, and whether what
// comes back still carries its secrets.
type RedactionSuite struct {
	base.APISuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc

	node    string
	nodeCtx context.Context //nolint:containedctx
}

// SuiteName returns the name of the suite.
func (suite *RedactionSuite) SuiteName() string {
	return "api.RedactionSuite"
}

// SetupTest initializes test context.
func (suite *RedactionSuite) SetupTest() {
	if testing.Short() {
		suite.T().Skip("skipping in short mode")
	}

	if suite.Airgapped {
		suite.T().Skip("skipping in airgapped mode: the probe used as the redaction oracle has to succeed")
	}

	if !suite.Capabilities().SupportsKubernetes {
		// GenerateClientConfiguration needs the cluster name and endpoint to build a context name
		suite.T().Skip("skipping without Kubernetes: cannot mint per-role client configurations")
	}

	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 5*time.Minute)

	// the per-role client certificates are minted by a control plane node
	suite.node = suite.RandomDiscoveredNodeInternalIP(configmachine.TypeControlPlane)
	suite.nodeCtx = client.WithNode(suite.ctx, suite.node)
}

// TearDownTest cleans up test context.
func (suite *RedactionSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// clientForRole mints a client certificate carrying exactly the given role and builds a client on it.
func (suite *RedactionSuite) clientForRole(r role.Role) *client.Client {
	suite.T().Helper()

	resp, err := suite.Client.GenerateClientConfiguration(suite.nodeCtx, &machineapi.GenerateClientConfigurationRequest{
		Roles:  role.MakeSet(r).Strings(),
		CrtTtl: durationpb.New(time.Hour),
	})
	suite.Require().NoError(err)
	suite.Require().Len(resp.Messages, 1)

	cfg, err := clientconfig.FromBytes(resp.Messages[0].Talosconfig)
	suite.Require().NoError(err)

	// the generated configuration carries no endpoints, so reuse the ones the suite is talking to
	cfg.Contexts[cfg.Context].Endpoints = suite.Client.GetEndpoints()

	cli, err := client.New(suite.ctx, client.WithConfig(cfg))
	suite.Require().NoError(err)

	suite.T().Cleanup(func() {
		suite.Assert().NoError(cli.Close())
	})

	return cli
}

// plantProbe applies the probe carrying the credentials, waits for the resource to show up, and
// returns the function which takes it back out again.
//
// The caller defers the returned function rather than registering it with t.Cleanup: testify runs
// TearDownTest, which cancels the suite context, before any cleanup registered on the test.
func (suite *RedactionSuite) plantProbe() func() {
	probeURL := fmt.Sprintf("https://%s:%s@%s", redactionProbeUser, redactionProbePassword, redactionProbeHost)

	probeConfig := confignetwork.NewHTTPProbeConfigV1Alpha1("redaction-probe")
	probeConfig.ProbeInterval = time.Minute
	probeConfig.ProbeFailureThreshold = 5
	probeConfig.HTTPEndpoint = configmeta.URL{URL: ensure.Value(url.Parse(probeURL))}
	probeConfig.HTTPTimeout = 10 * time.Second

	suite.PatchMachineConfig(suite.nodeCtx, probeConfig)

	// the credentials are in the spec, and only in the spec: the ID has them stripped
	rtestutils.AssertResource(
		suite.nodeCtx, suite.T(), suite.Client.COSI, redactionProbeID,
		func(spec *networkres.ProbeSpec, asrt *assert.Assertions) {
			asrt.NotNil(spec.TypedSpec().HTTP.URL)
		},
	)

	return func() {
		suite.RemoveMachineConfigDocuments(suite.nodeCtx, confignetwork.HTTPProbeKind)
		rtestutils.AssertNoResource[*networkres.ProbeSpec](suite.nodeCtx, suite.T(), suite.Client.COSI, redactionProbeID)
	}
}

// assertReadable checks whether a resource kind can be listed at all with the given client and mode.
func (suite *RedactionSuite) assertReadable(cli *client.Client, mode redactable.Mode, kind resource.Kind, allowed bool, what string) {
	suite.T().Helper()

	ctx := redactable.WithMode(client.WithNode(suite.ctx, suite.node), mode)

	_, err := cli.COSI.List(ctx, kind, state.WithListUnmarshalOptions(state.WithSkipProtobufUnmarshal()))

	if allowed {
		suite.Assert().NoError(err, "%s: expected the read to be allowed", what)
	} else {
		suite.Assert().Equal(codes.PermissionDenied, client.StatusCode(err), "%s: expected permission denied, got %v", what, err)
	}
}

// assertProbeRedacted reads the probe and checks whether its password survived the trip.
//
// It is only called where the probe is readable: the credentials are in the spec, and nothing but
// redaction can take them out.
func (suite *RedactionSuite) assertProbeRedacted(cli *client.Client, mode redactable.Mode, redacted bool, what string) {
	suite.T().Helper()

	ctx := redactable.WithMode(client.WithNode(suite.ctx, suite.node), mode)

	probe, err := safe.StateGetByID[*networkres.ProbeSpec](ctx, cli.COSI, redactionProbeID)
	if !suite.Assert().NoError(err, "%s: expected the probe to be readable", what) {
		return
	}

	u := probe.TypedSpec().HTTP.URL
	suite.Require().NotNil(u)
	suite.Require().NotNil(u.User, "%s: the user info should never be dropped, only replaced", what)

	password, _ := u.User.Password()
	_, annotated := probe.Metadata().Annotations().Get(constants.AnnotationRedacted)

	// the username is not a secret and survives either way, which is what makes the password a
	// meaningful oracle rather than the whole URL going missing
	suite.Assert().Equal(redactionProbeUser, u.User.Username(), "%s", what)

	if redacted {
		suite.Assert().Equal(constants.Redacted, password, "%s: expected the password to be redacted", what)
		suite.Assert().True(annotated, "%s: expected the resource to be annotated as redacted", what)
	} else {
		suite.Assert().Equal(redactionProbePassword, password, "%s: expected the password to come through", what)
		suite.Assert().False(annotated, "%s: expected no redaction annotation", what)
	}
}

// TestRedactionMatrix walks the whole decision table of redactable.Header.
func (suite *RedactionSuite) TestRedactionMatrix() {
	defer suite.plantProbe()()

	clients := map[role.Role]*client.Client{
		role.Admin:    suite.clientForRole(role.Admin),
		role.Operator: suite.clientForRole(role.Operator),
		role.Reader:   suite.clientForRole(role.Reader),
	}

	// readable with any role, in any mode, and carrying nothing to redact
	nonSensitive := resource.NewMetadata(
		networkres.NamespaceName, networkres.HostnameStatusType, "", resource.VersionUndefined,
	)
	// sensitive and reviewed as safe to hand out redacted
	sensitiveRedactable := resource.NewMetadata(
		networkres.NamespaceName, networkres.ProbeSpecType, "", resource.VersionUndefined,
	)
	// sensitive with nothing meaningful left once redacted, so never served in a redacted form
	sensitiveOpaque := resource.NewMetadata(
		secrets.NamespaceName, secrets.OSRootType, "", resource.VersionUndefined,
	)

	for _, mode := range []redactable.Mode{
		redactable.ModeDisabled,
		redactable.ModeAccept,
		redactable.ModeEnforce,
	} {
		for _, r := range []role.Role{role.Admin, role.Operator, role.Reader} {
			var (
				admin = r == role.Admin
				cli   = clients[r]
				what  = fmt.Sprintf("mode %q, role %q", mode, r)
			)

			// a non-sensitive type is readable with any role in any mode
			suite.assertReadable(cli, mode, nonSensitive, true, what+", non-sensitive")

			// a sensitive, redactable type: admin-only until the client says it can take a redacted
			// answer, then readable by everyone
			readable := admin || mode != redactable.ModeDisabled

			suite.assertReadable(cli, mode, sensitiveRedactable, readable, what+", sensitive redactable")

			// where it is readable, the probe says whether redaction actually happened, rather than
			// just whether the read was allowed
			if readable {
				suite.assertProbeRedacted(cli, mode,
					mode == redactable.ModeEnforce || (mode == redactable.ModeAccept && !admin),
					what)
			}

			// a sensitive type which cannot be redacted: admin-only, and denied even to an admin
			// once the client demands that everything it reads be redacted
			suite.assertReadable(cli, mode, sensitiveOpaque,
				admin && mode != redactable.ModeEnforce,
				what+", sensitive opaque")
		}
	}
}

func init() {
	allSuites = append(allSuites, new(RedactionSuite))
}
