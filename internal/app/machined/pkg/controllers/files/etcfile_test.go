// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package files_test

import (
	"os"
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

	etcRoot xfs.Root
}

func (suite *EtcFileSuite) TestFiles() {
	etcFileSpec := files.NewEtcFileSpec(files.NamespaceName, "test1")
	etcFileSpec.TypedSpec().Contents = []byte("foo")
	etcFileSpec.TypedSpec().Mode = 0o644

	suite.Create(etcFileSpec)

	// controller should put a finalizer on the spec, bumping the version
	expectedVersion := etcFileSpec.Metadata().Version().Next()

	// the controller writes into the managed xfs root
	// final /etc is read-only through the composed overlay.
	ctest.AssertResource(suite, "test1", func(r *files.EtcFileStatus, asrt *assert.Assertions) {
		asrt.Equal(expectedVersion.String(), r.TypedSpec().SpecVersion)

		rwb, err := xfs.ReadFile(suite.etcRoot, "test1")
		asrt.NoError(err)
		asrt.Equal("foo", string(rwb))
	})

	rtestutils.Destroy[*files.EtcFileSpec](suite.Ctx(), suite.T(), suite.State(), []string{etcFileSpec.Metadata().ID()})
}

func (suite *EtcFileSuite) TestUpdate() {
	specs := make([]*files.EtcFileSpec, 0, 2)

	for _, id := range []string{"update1", "update2"} {
		spec := files.NewEtcFileSpec(files.NamespaceName, id)
		spec.TypedSpec().Contents = []byte(id)
		spec.TypedSpec().Mode = 0o644

		suite.Create(spec)

		specs = append(specs, spec)
	}

	assertFile := func(id, contents string) {
		ctest.AssertResource(suite, id, func(r *files.EtcFileStatus, asrt *assert.Assertions) {
			spec, err := suite.State().Get(suite.Ctx(), files.NewEtcFileSpec(files.NamespaceName, id).Metadata())
			asrt.NoError(err)

			// consumers (e.g. WaitForCARoots) wait for the status to match the latest spec version
			asrt.Equal(spec.Metadata().Version().String(), r.TypedSpec().SpecVersion)

			rwb, err := xfs.ReadFile(suite.etcRoot, id)
			asrt.NoError(err)
			asrt.Equal(contents, string(rwb))
		})
	}

	assertFile("update1", "update1")
	assertFile("update2", "update2")

	ctest.UpdateWithConflicts(suite, specs[0], func(spec *files.EtcFileSpec) error {
		spec.TypedSpec().Contents = []byte("updated")

		return nil
	})

	assertFile("update1", "updated")
	assertFile("update2", "update2")

	// destroying the spec removes the file and the status
	rtestutils.Destroy[*files.EtcFileSpec](suite.Ctx(), suite.T(), suite.State(), []string{"update1"})

	ctest.AssertNoResource[*files.EtcFileStatus](suite, "update1")

	_, err := xfs.Stat(suite.etcRoot, "update1")
	suite.Assert().ErrorIs(err, os.ErrNotExist)

	assertFile("update2", "update2")

	rtestutils.Destroy[*files.EtcFileSpec](suite.Ctx(), suite.T(), suite.State(), []string{"update2"})
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

			if ok {
				etcSuite.etcRoot = &xfs.UnixRoot{FS: opentree.NewFromPath(s.T().TempDir())}
			} else {
				etcSuite.etcRoot = &xfs.OSRoot{Shadow: s.T().TempDir()}
			}

			s.Require().NoError(etcSuite.etcRoot.OpenFS())

			s.Require().NoError(s.Runtime().RegisterQController(&filesctrl.EtcFileController{
				EtcRoot: etcSuite.etcRoot,
			}))
		},
		AfterTearDown: func(*ctest.DefaultSuite) {
			etcSuite.etcRoot.Close() //nolint:errcheck
		},
	}

	suite.Run(t, etcSuite)
}
