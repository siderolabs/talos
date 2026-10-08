// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	runtimecontrollers "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime"
	machineconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	runtimeconfig "github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

type SELinuxPolicySuite struct {
	ctest.DefaultSuite
}

func TestSELinuxPolicySuite(t *testing.T) {
	suite.Run(t, &SELinuxPolicySuite{
		AfterSetup: func(suite *ctest.DefaultSuite) {
			suite.Require().NoError(suite.Runtime().RegisterController(&runtimecontrollers.SELinuxPolicyController{
				// a module saying so is rejected, and so is a module needing another one absent from the set
				Compile: func(_ context.Context, modules map[string]string) ([]byte, error) {
					for name, content := range modules {
						if strings.Contains(content, "bad") {
							return nil, errors.New("secilc: " + name + " rejected")
						}

						if _, needs, ok := strings.Cut(content, "needs "); ok {
							if _, present := modules[strings.TrimSpace(needs)]; !present {
								return nil, errors.New("secilc: " + name + " needs " + needs)
							}
						}
					}

					return []byte("policy"), nil
				},
				Load: func([]byte) error { return nil },
			}))
		},
	})
}

func (suite *SELinuxPolicySuite) applyModules(modules map[string]string) {
	documents := make([]machineconfig.Document, 0, len(modules))

	for name, content := range modules {
		document := runtimeconfig.NewSELinuxPolicyConfigV1Alpha1(name)
		document.PolicyContent = content
		documents = append(documents, document)
	}

	cntr, err := container.New(documents...)
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(cntr)

	if existing, err := suite.State().Get(suite.Ctx(), cfg.Metadata()); err == nil {
		cfg.Metadata().SetVersion(existing.Metadata().Version())
		suite.Update(cfg)
	} else {
		suite.Create(cfg)
	}
}

// TestRejectedModules: a module the compile rejects is left out and the others load, whatever the order the compile
// needs them in; the status lists the modules of the config and says which were rejected and why, until they change.
func (suite *SELinuxPolicySuite) TestRejectedModules() {
	suite.applyModules(map[string]string{"good": "(type pod_good_t)\n"})

	ctest.AssertResource(suite, runtimeres.SELinuxPolicyStatusID, func(status *runtimeres.SELinuxPolicyStatus, asrt *assert.Assertions) {
		asrt.Equal([]string{"good"}, status.TypedSpec().Modules)
		asrt.Empty(status.TypedSpec().Error)
	})

	// a-extra needs z-base, which the name order compiles after it
	suite.applyModules(map[string]string{"good": "(type pod_good_t)\n", "bad": "(type bad_t)\n", "a-extra": "needs z-base\n", "z-base": "(type pod_z_t)\n"})

	ctest.AssertResource(suite, runtimeres.SELinuxPolicyStatusID, func(status *runtimeres.SELinuxPolicyStatus, asrt *assert.Assertions) {
		asrt.Equal([]string{"a-extra", "bad", "good", "z-base"}, status.TypedSpec().Modules)
		asrt.Equal("module bad rejected: secilc: bad rejected", status.TypedSpec().Error)
	})

	suite.applyModules(map[string]string{"good": "(type pod_good_t)\n", "a-extra": "needs z-base\n", "z-base": "(type pod_z_t)\n"})

	ctest.AssertResource(suite, runtimeres.SELinuxPolicyStatusID, func(status *runtimeres.SELinuxPolicyStatus, asrt *assert.Assertions) {
		asrt.Equal([]string{"a-extra", "good", "z-base"}, status.TypedSpec().Modules)
		asrt.Empty(status.TypedSpec().Error)
	})
}
