// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"fmt"
	"io"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/client"
	runtimecfg "github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
	filesres "github.com/siderolabs/talos/pkg/machinery/resources/files"
)

const etcFileConfigWaitTimeout = 30 * time.Second

// EtcFileConfigSuite verifies user-managed /etc file configuration.
type EtcFileConfigSuite struct {
	base.APISuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *EtcFileConfigSuite) SuiteName() string {
	return "api.EtcFileConfigSuite"
}

// SetupTest ...
func (suite *EtcFileConfigSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), time.Minute)
}

// TearDownTest ...
func (suite *EtcFileConfigSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestEtcFileConfig verifies create, update, delete, and re-add flows.
func (suite *EtcFileConfigSuite) TestEtcFileConfig() {
	node := suite.RandomDiscoveredNodeInternalIP()
	nodeCtx := client.WithNode(suite.ctx, node)
	name := fmt.Sprintf("talos-integration-etcfile-%04x.conf", rand.Int32())
	path := "/etc/" + name

	suite.T().Logf("testing EtcFileConfig %q on node %q", name, node)

	defer suite.RemoveMachineConfigDocumentsByName(nodeCtx, runtimecfg.EtcFileConfigKind, name)

	suite.patchEtcFileConfig(nodeCtx, name, 0o644, "first")
	suite.assertEtcFile(nodeCtx, path, "first")
	suite.assertEtcFileSpec(nodeCtx, name, 0o644, "first")

	suite.patchEtcFileConfig(nodeCtx, name, 0o600, "second")
	suite.assertEtcFile(nodeCtx, path, "second")
	suite.assertEtcFileSpec(nodeCtx, name, 0o600, "second")

	suite.RemoveMachineConfigDocumentsByName(nodeCtx, runtimecfg.EtcFileConfigKind, name)
	suite.assertNoEtcFile(nodeCtx, path)

	suite.patchEtcFileConfig(nodeCtx, name, 0o644, "third")
	suite.assertEtcFile(nodeCtx, path, "third")
	suite.assertEtcFileSpec(nodeCtx, name, 0o644, "third")
}

func (suite *EtcFileConfigSuite) patchEtcFileConfig(ctx context.Context, name string, mode runtimecfg.EtcFileMode, contents string) {
	doc := runtimecfg.NewEtcFileConfigV1Alpha1(name)
	doc.FileMode = mode
	doc.Contents = contents

	suite.PatchMachineConfig(ctx, doc)
}

func (suite *EtcFileConfigSuite) assertEtcFile(ctx context.Context, path, contents string) {
	suite.EventuallyWithT(
		func(collect *assert.CollectT) {
			reader, err := suite.Client.Read(ctx, path)
			if !assert.NoError(collect, err) {
				return
			}

			defer reader.Close() //nolint:errcheck

			body, err := io.ReadAll(reader)
			if !assert.NoError(collect, err) {
				return
			}

			assert.Equal(collect, contents, strings.TrimSpace(string(body)))
		},
		etcFileConfigWaitTimeout, 100*time.Millisecond, "waiting for %s contents", path,
	)
}

func (suite *EtcFileConfigSuite) assertNoEtcFile(ctx context.Context, path string) {
	suite.EventuallyWithT(
		func(collect *assert.CollectT) {
			reader, err := suite.Client.Read(ctx, path)
			if reader != nil {
				_, readErr := io.Copy(io.Discard, reader)
				closeErr := reader.Close()

				if err == nil {
					err = readErr
				}

				if err == nil {
					err = closeErr
				}
			}

			assert.Error(collect, err)
		},
		etcFileConfigWaitTimeout, 100*time.Millisecond, "waiting for %s removal", path,
	)
}

func (suite *EtcFileConfigSuite) assertEtcFileSpec(ctx context.Context, name string, mode runtimecfg.EtcFileMode, contents string) {
	rtestutils.AssertResource(
		ctx, suite.T(), suite.Client.COSI, name,
		func(spec *filesres.EtcFileSpec, asrt *assert.Assertions) {
			asrt.EqualValues(mode, spec.TypedSpec().Mode)
			asrt.Equal(contents, string(spec.TypedSpec().Contents))
		},
	)
}

func init() {
	allSuites = append(allSuites, new(EtcFileConfigSuite))
}
