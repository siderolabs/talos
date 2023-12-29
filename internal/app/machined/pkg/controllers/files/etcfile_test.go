// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package files_test

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/suite"

	filesctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/files"
	"github.com/siderolabs/talos/pkg/logging"
	"github.com/siderolabs/talos/pkg/machinery/resources/files"
)

type EtcFileSuite struct {
	suite.Suite

	state state.State

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc

	etcPath    string
	shadowPath string
}

func (suite *EtcFileSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	suite.runtime, err = runtime.NewRuntime(suite.state, logging.Wrap(log.Writer()))
	suite.Require().NoError(err)

	suite.startRuntime()

	suite.etcPath = suite.T().TempDir()
	suite.shadowPath = suite.T().TempDir()

	suite.Require().NoError(
		suite.runtime.RegisterController(
			&filesctrl.EtcFileController{
				EtcPath:    suite.etcPath,
				ShadowPath: suite.shadowPath,
			},
		),
	)
}

func (suite *EtcFileSuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
}

func (suite *EtcFileSuite) assertEtcFile(filename, contents string, expectedVersion resource.Version) error {
	b, err := os.ReadFile(filepath.Join(suite.etcPath, filename))
	if err != nil {
		return retry.ExpectedError(err)
	}

	if string(b) != contents {
		return retry.ExpectedErrorf("contents don't match %q != %q", string(b), contents)
	}

	r, err := safe.ReaderGet[*files.EtcFileStatus](suite.ctx, suite.state, resource.NewMetadata(files.NamespaceName, files.EtcFileStatusType, filename, resource.VersionUndefined))
	if err != nil {
		if state.IsNotFoundError(err) {
			return retry.ExpectedError(err)
		}

		return err
	}

	version := r.TypedSpec().SpecVersion

	expected, err := strconv.Atoi(expectedVersion.String())
	suite.Require().NoError(err)

	ver, err := strconv.Atoi(version)
	suite.Require().NoError(err)

	if ver < expected {
		return retry.ExpectedErrorf("version mismatch %s > %s", expectedVersion, version)
	}

	return nil
}

func (suite *EtcFileSuite) TestFiles() {
	etcFileSpec := files.NewEtcFileSpec(files.NamespaceName, "test1")
	etcFileSpec.TypedSpec().Contents = []byte("foo")
	etcFileSpec.TypedSpec().Mode = 0o644

	// create "read-only" mock (in Talos it's part of rootfs)
	suite.T().Logf("mock created %q", filepath.Join(suite.etcPath, etcFileSpec.Metadata().ID()))
	suite.Require().NoError(os.WriteFile(filepath.Join(suite.etcPath, etcFileSpec.Metadata().ID()), nil, 0o644))

	suite.Require().NoError(suite.state.Create(suite.ctx, etcFileSpec))

	suite.Assert().NoError(
		retry.Constant(5*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertEtcFile("test1", "foo", etcFileSpec.Metadata().Version())
			},
		),
	)

	for _, r := range []resource.Resource{etcFileSpec} {
		for {
			ready, err := suite.state.Teardown(suite.ctx, r.Metadata())
			suite.Require().NoError(err)

			if ready {
				break
			}

			time.Sleep(100 * time.Millisecond)
		}
	}
}

func (suite *EtcFileSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.wg.Wait()
}

func TestEtcFileSuite(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("requires root")
	}

	suite.Run(t, new(EtcFileSuite))
}
