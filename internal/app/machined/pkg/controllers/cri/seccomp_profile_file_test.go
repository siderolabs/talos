// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cri_test

import (
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/cri"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	criseccompresource "github.com/siderolabs/talos/pkg/machinery/resources/cri"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

func (suite *CRISeccompProfileFileSuite) TestReconcileSeccompProfileFile() {
	// need to mock mountStatus so that the controller moves ahead with the actual code
	mountStatus := runtimeres.NewMountStatus(runtimeres.NamespaceName, "EPHEMERAL")
	suite.Require().NoError(suite.State().Create(suite.Ctx(), mountStatus))

	for _, tt := range []struct {
		seccompProfileName  string
		seccompProfileValue map[string]any
	}{
		{
			seccompProfileName: "audit.json",
			seccompProfileValue: map[string]any{
				"defaultAction": "SCMP_ACT_LOG",
			},
		},
		{
			seccompProfileName: "deny.json",
			seccompProfileValue: map[string]any{
				"defaultAction": "SCMP_ACT_ERRNO",
			},
		},
	} {
		seccompProfiles := criseccompresource.NewSeccompProfile(tt.seccompProfileName)
		seccompProfiles.TypedSpec().Name = tt.seccompProfileName
		seccompProfiles.TypedSpec().Value = tt.seccompProfileValue
		suite.Require().NoError(suite.State().Create(suite.Ctx(), seccompProfiles))

		suite.AssertWithin(1*time.Second, 100*time.Millisecond, func() error {
			if _, err := os.Stat(suite.seccompProfilesDirectory + "/" + tt.seccompProfileName); err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return retry.ExpectedError(err)
				}

				return err
			}

			seccompProfileContent, err := os.ReadFile(suite.seccompProfilesDirectory + "/" + tt.seccompProfileName)
			suite.Assert().NoError(err)

			expectedSeccompProfileContent, err := json.Marshal(tt.seccompProfileValue)
			suite.Assert().NoError(err)

			suite.Assert().Equal(seccompProfileContent, expectedSeccompProfileContent)

			return nil
		})
	}

	// create a directory and file manually in the seccomp profile directory
	// ensure that the controller deletes the manually created directory/file
	// also ensure that an update doesn't update existing files timestamp
	suite.Assert().NoError(os.Mkdir(suite.seccompProfilesDirectory+"/test", 0o755))
	suite.Assert().NoError(os.WriteFile(suite.seccompProfilesDirectory+"/test.json", []byte("{}"), 0o644))

	auditJSONSeccompProfile, err := os.Stat(suite.seccompProfilesDirectory + "/audit.json")
	suite.Assert().NoError(err)

	// delete deny.json resource
	suite.Assert().NoError(suite.State().Destroy(suite.Ctx(), resource.NewMetadata(criseccompresource.NamespaceName, criseccompresource.SeccompProfileType, "deny.json", resource.VersionUndefined)))

	suite.AssertWithin(1*time.Second, 100*time.Millisecond, func() error {
		auditJSONSeccompProfileAfterUpdate, err := os.Stat(suite.seccompProfilesDirectory + "/audit.json")
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return retry.ExpectedError(err)
			}

			return err
		}

		suite.Eventually(func() bool {
			return suite.NoFileExists(suite.seccompProfilesDirectory + "/deny.json")
		}, 1*time.Second, 100*time.Millisecond)

		suite.Eventually(func() bool {
			return suite.NoFileExists(suite.seccompProfilesDirectory + "/test.json")
		}, 1*time.Second, 100*time.Millisecond)

		suite.Eventually(func() bool {
			return suite.NoDirExists(suite.seccompProfilesDirectory + "/test")
		}, 1*time.Second, 100*time.Millisecond)

		suite.Assert().Equal(auditJSONSeccompProfile.ModTime(), auditJSONSeccompProfileAfterUpdate.ModTime())

		return nil
	})
}

func TestSeccompProfileFileSuite(t *testing.T) {
	seccompProfiesDirectory := t.TempDir()

	suite.Run(t, &CRISeccompProfileFileSuite{
		DefaultSuite: ctest.DefaultSuite{
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&cri.SeccompProfileFileController{
					SeccompProfilesDirectory: seccompProfiesDirectory,
				}))
			},
		},
		seccompProfilesDirectory: seccompProfiesDirectory,
	})
}

type CRISeccompProfileFileSuite struct {
	ctest.DefaultSuite
	seccompProfilesDirectory string
}
