// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package files_test

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	filesctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/files"
	"github.com/siderolabs/talos/pkg/machinery/resources/files"
)

type EtcFileSuite struct {
	ctest.DefaultSuite
	etcPath    string
	shadowPath string
}

func TestEtcFileSuite(t *testing.T) {
	// skip test if we are not root
	if os.Getuid() != 0 {
		t.Skip("can't run the test as non-root")
	}

	etcTempPath := t.TempDir()
	shadowTempPath := t.TempDir()

	suite.Run(t, &EtcFileSuite{
		DefaultSuite: ctest.DefaultSuite{
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&filesctrl.EtcFileController{
					EtcPath:    etcTempPath,
					ShadowPath: shadowTempPath,
				}))
			},
		},
		etcPath:    etcTempPath,
		shadowPath: shadowTempPath,
	})
}

func (suite *EtcFileSuite) assertEtcFile(filename, contents string, expectedVersion resource.Version) error {
	b, err := os.ReadFile(filepath.Join(suite.etcPath, filename))
	if err != nil {
		return retry.ExpectedError(err)
	}

	if string(b) != contents {
		return retry.ExpectedErrorf("contents don't match %q != %q", string(b), contents)
	}

	r, err := suite.State().Get(
		suite.Ctx(),
		resource.NewMetadata(files.NamespaceName, files.EtcFileStatusType, filename, resource.VersionUndefined),
	)
	if err != nil {
		if state.IsNotFoundError(err) {
			return retry.ExpectedError(err)
		}

		return err
	}

	version := r.(*files.EtcFileStatus).TypedSpec().SpecVersion

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

	suite.Require().NoError(suite.State().Create(suite.Ctx(), etcFileSpec))

	suite.AssertWithin(5*time.Second, 100*time.Millisecond, func() error {
		return suite.assertEtcFile("test1", "foo", etcFileSpec.Metadata().Version())
	})

	for _, r := range []resource.Resource{etcFileSpec} {
		for {
			ready, err := suite.State().Teardown(suite.Ctx(), r.Metadata())
			suite.Require().NoError(err)

			if ready {
				break
			}

			time.Sleep(100 * time.Millisecond)
		}
	}
}
