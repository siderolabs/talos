// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cri_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/cri"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	criseccompresource "github.com/siderolabs/talos/pkg/machinery/resources/cri"
)

func (suite *CRISeccompProfileSuite) TestReconcileSeccompProfile() {
	cfg := config.NewMachineConfig(&v1alpha1.Config{
		MachineConfig: &v1alpha1.MachineConfig{
			MachineSeccompProfiles: []*v1alpha1.MachineSeccompProfile{
				{
					MachineSeccompProfileName: "audit.json",
					MachineSeccompProfileValue: v1alpha1.Unstructured{
						Object: map[string]interface{}{
							"defaultAction": "SCMP_ACT_LOG",
						},
					},
				},
				{
					MachineSeccompProfileName: "deny.json",
					MachineSeccompProfileValue: v1alpha1.Unstructured{
						Object: map[string]interface{}{
							"defaultAction": "SCMP_ACT_ERRNO",
						},
					},
				},
			},
		},
	})

	suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))

	for _, tt := range []struct {
		name  string
		value map[string]interface{}
	}{
		{
			name: "audit.json",
			value: map[string]interface{}{
				"defaultAction": "SCMP_ACT_LOG",
			},
		},
		{
			name: "deny.json",
			value: map[string]interface{}{
				"defaultAction": "SCMP_ACT_ERRNO",
			},
		},
	} {
		suite.AssertWithin(1*time.Second, 100*time.Millisecond, func() error {
			seccompProfile, err := ctest.Get[*criseccompresource.SeccompProfile](
				suite,
				criseccompresource.NewSeccompProfile(tt.name).Metadata(),
			)
			if err != nil {
				if state.IsNotFoundError(err) {
					return retry.ExpectedError(err)
				}

				return err
			}

			spec := seccompProfile.TypedSpec()

			suite.Assert().Equal(tt.name, spec.Name)
			suite.Assert().Equal(tt.value, spec.Value)

			return nil
		})
	}

	// test deletion
	cfg = config.NewMachineConfig(&v1alpha1.Config{
		MachineConfig: &v1alpha1.MachineConfig{
			MachineSeccompProfiles: []*v1alpha1.MachineSeccompProfile{
				{
					MachineSeccompProfileName: "audit.json",
					MachineSeccompProfileValue: v1alpha1.Unstructured{
						Object: map[string]interface{}{
							"defaultAction": "SCMP_ACT_LOG",
						},
					},
				},
			},
		},
	})

	cfg.Metadata().SetVersion(cfg.Metadata().Version().Next())
	suite.Require().NoError(suite.State().Update(suite.Ctx(), cfg))

	suite.AssertWithin(1*time.Second, 100*time.Millisecond, func() error {
		_, err := ctest.Get[*criseccompresource.SeccompProfile](
			suite,
			criseccompresource.NewSeccompProfile("deny.json").Metadata(),
		)
		if err != nil {
			if state.IsNotFoundError(err) {
				return nil
			}

			return err
		}

		return retry.ExpectedError(fmt.Errorf("seccomp profile with id deny.json should not exist"))
	})
}

func TestSeccompProfileSuite(t *testing.T) {
	suite.Run(t, &CRISeccompProfileSuite{
		DefaultSuite: ctest.DefaultSuite{
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&cri.SeccompProfileController{}))
			},
		},
	})
}

type CRISeccompProfileSuite struct {
	ctest.DefaultSuite
}
