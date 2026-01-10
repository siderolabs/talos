// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	runtimectrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

func TestLogPersistenceSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &LogPersistenceSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
		},
	})
}

type LogPersistenceSuite struct {
	ctest.DefaultSuite
}

type loggingMock struct{}

func (loggingMock) ServiceLog(service string) runtime.LogHandler { return nil }

func (loggingMock) SetSenders(senders []runtime.LogSender) []runtime.LogSender { return nil }

func (loggingMock) SetLineWriter(w runtime.LogWriter) {}

func (loggingMock) RegisteredLogs() []string { return nil }

func (suite *LogPersistenceSuite) TestDefault() {
	ctrl := &runtimectrl.LogPersistenceController{
		V1Alpha1Logging: loggingMock{},
	}

	suite.Require().NoError(suite.Runtime().RegisterController(ctrl))

	requestID := ctrl.Name() + "-" + constants.LogMountPoint

	ctest.AssertResource(suite, requestID, func(*block.VolumeMountRequest, *assert.Assertions) {})

	errCh := make(chan error, 1)

	go func() {
		errCh <- ctrl.WriteLog("service1", []byte("line1"))
	}()

	select {
	case <-errCh:
		suite.Fail("expected WriteLog to block")
	case <-time.After(10 * time.Millisecond):
		// expected
	}

	logDir := suite.T().TempDir()

	vms := block.NewVolumeMountStatus(requestID)
	vms.TypedSpec().Target = logDir
	suite.Create(vms)

	ctest.AssertResource(suite, requestID, func(vms *block.VolumeMountStatus, asrt *assert.Assertions) {
		asrt.True(vms.Metadata().Finalizers().Has(ctrl.Name()))
	})

	select {
	case err := <-errCh:
		suite.NoError(err)
	case <-time.After(500 * time.Millisecond):
		suite.Fail("expected WriteLog to complete after mount")
	}

	suite.Assert().FileExists(filepath.Join(logDir, "service1.log"))

	_, err := suite.State().Teardown(suite.Ctx(), vms.Metadata())
	suite.Require().NoError(err)

	ctest.AssertResource(suite, requestID, func(vms *block.VolumeMountStatus, asrt *assert.Assertions) {
		asrt.False(vms.Metadata().Finalizers().Has(ctrl.Name()))
	})

	st, err := os.Stat(filepath.Join(logDir, "service1.log"))
	suite.Require().NoError(err)

	suite.Assert().Equal(int64(6), st.Size())
}
