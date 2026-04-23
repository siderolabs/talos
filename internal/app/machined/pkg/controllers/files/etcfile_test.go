// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package files_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	filesctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/files"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/files"
	"github.com/siderolabs/talos/pkg/xfs"
	"github.com/siderolabs/talos/pkg/xfs/opentree"
)

type EtcFileSuite struct {
	ctest.DefaultSuite

	etcPath string
	etcRoot xfs.Root
}

func (suite *EtcFileSuite) TestFiles() {
	etcFileSpec := files.NewEtcFileSpec(files.NamespaceName, "test1")
	etcFileSpec.TypedSpec().Contents = []byte("foo")
	etcFileSpec.TypedSpec().Mode = 0o644

	// create "read-only" mock (in Talos it's part of rootfs)
	suite.T().Logf("mock created %q", filepath.Join(suite.etcPath, etcFileSpec.Metadata().ID()))
	suite.Require().NoError(os.WriteFile(filepath.Join(suite.etcPath, etcFileSpec.Metadata().ID()), nil, 0o644))

	suite.Create(etcFileSpec)

	// controller should put a finalizer on the spec, bumping the version
	expectedVersion := etcFileSpec.Metadata().Version().Next()

	ctest.AssertResource(suite, "test1", func(r *files.EtcFileStatus, asrt *assert.Assertions) {
		asrt.Equal(expectedVersion.String(), r.TypedSpec().SpecVersion)

		rwb, err := xfs.ReadFile(suite.etcRoot, "test1")
		asrt.NoError(err)
		asrt.Equal("foo", string(rwb))

		rob, err := os.ReadFile(filepath.Join(suite.etcPath, "test1"))
		asrt.NoError(err)
		asrt.Equal("foo", string(rob))
	})

	rtestutils.Destroy[*files.EtcFileSpec](suite.Ctx(), suite.T(), suite.State(), []string{etcFileSpec.Metadata().ID()})
}

func TestEtcFileSuite(t *testing.T) {
	t.Parallel()

	if os.Geteuid() != 0 {
		t.Skip("requires root")
	}

	etcSuite := &EtcFileSuite{}
	etcSuite.DefaultSuite = ctest.DefaultSuite{
		Timeout: 10 * time.Second,
		AfterSetup: func(s *ctest.DefaultSuite) {
			ok, err := runtime.KernelCapabilities().OpentreeOnAnonymousFS()
			s.Require().NoError(err)

			etcSuite.etcPath = s.T().TempDir()

			if ok {
				etcSuite.etcRoot = &xfs.UnixRoot{FS: opentree.NewFromPath(s.T().TempDir())}
			} else {
				etcSuite.etcRoot = &xfs.OSRoot{Shadow: s.T().TempDir()}
			}

			s.Require().NoError(etcSuite.etcRoot.OpenFS())

			s.Require().NoError(s.Runtime().RegisterController(&filesctrl.EtcFileController{
				EtcPath: etcSuite.etcPath,
				EtcRoot: etcSuite.etcRoot,
			}))
		},
		AfterTearDown: func(*ctest.DefaultSuite) {
			etcSuite.etcRoot.Close() //nolint:errcheck
		},
	}

	suite.Run(t, etcSuite)
}
