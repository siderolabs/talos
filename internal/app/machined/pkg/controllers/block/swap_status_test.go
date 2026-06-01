// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block_test

import (
	_ "embed"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	blockctrls "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

type SwapStatusSuite struct {
	ctest.DefaultSuite
}

func TestSwapStatusSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &SwapStatusSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout:    3 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {},
		},
	})
}

//go:embed "testdata/procswaps.txt"
var procSwapsData []byte

func (suite *SwapStatusSuite) TestReconcile() {
	tmpDir := suite.T().TempDir()
	path := filepath.Join(tmpDir, "procswaps.txt")

	suite.Require().NoError(os.WriteFile(path, procSwapsData, 0o644))

	suite.Require().NoError(suite.Runtime().RegisterController(&blockctrls.SwapStatusController{
		ProcSwapsPath: path,
	}))

	ctest.AssertResources(suite, []string{"/dev/vda1", "/dev/vda2"}, func(s *block.SwapStatus, asrt *assert.Assertions) {
		asrt.Equal("partition", s.TypedSpec().Type)

		switch s.Metadata().ID() {
		case "/dev/vda1":
			asrt.Equal("/dev/vda1", s.TypedSpec().Device)
			asrt.EqualValues(524280*1024, s.TypedSpec().SizeBytes)
			asrt.Equal("512 MiB", s.TypedSpec().SizeHuman)
			asrt.EqualValues(1024*1024, s.TypedSpec().UsedBytes)
			asrt.Equal("1.0 MiB", s.TypedSpec().UsedHuman)
			asrt.EqualValues(-1, s.TypedSpec().Priority)
		case "/dev/vda2":
			asrt.Equal("/dev/vda2", s.TypedSpec().Device)
			asrt.EqualValues(2*1024*1024, s.TypedSpec().SizeBytes)
			asrt.Equal("2.0 MiB", s.TypedSpec().SizeHuman)
			asrt.EqualValues(0, s.TypedSpec().UsedBytes)
			asrt.Equal("0 B", s.TypedSpec().UsedHuman)
			asrt.EqualValues(-2, s.TypedSpec().Priority)
		}
	})
}
