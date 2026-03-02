// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package security_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	securityctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/security"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	securitycfg "github.com/siderolabs/talos/pkg/machinery/config/types/security"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/security"
)

type ImageVerificationConfigSuite struct {
	ctest.DefaultSuite
}

func (suite *ImageVerificationConfigSuite) TestReconcileNoConfig() {
	ctest.AssertNoResource[*security.ImageVerificationRule](suite, "0000")
}

func (suite *ImageVerificationConfigSuite) TestReconcileNoVerificationConfig() {
	cfg := config.NewMachineConfig(container.NewV1Alpha1(&v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		MachineConfig: &v1alpha1.MachineConfig{
			MachineType: "controlplane",
		},
	}))
	suite.Create(cfg)

	ctest.AssertNoResource[*security.ImageVerificationRule](suite, "0000")
}

func (suite *ImageVerificationConfigSuite) TestReconcileWithRules() {
	verificationCfg := securitycfg.NewImageVerificationConfigV1Alpha1()
	verificationCfg.ConfigRules = []securitycfg.ImageVerificationRuleV1Alpha1{
		{
			RuleImagePattern: "docker.io/*",
			RuleVerify:       new(false),
		},
		{
			RuleImagePattern: "ghcr.io/myorg/*",
			RuleVerify:       new(true),
			RuleKeylessVerifier: &securitycfg.ImageKeylessVerifierV1Alpha1{
				KeylessIssuer:       "https://token.actions.githubusercontent.com",
				KeylessSubjectRegex: "https://github.com/myorg/.*",
				KeylessRekorURL:     "https://rekor.sigstore.dev",
			},
		},
		{
			RuleImagePattern: "quay.io/*",
			RuleVerify:       new(true),
			RulePublicKeyVerifier: &securitycfg.ImagePublicKeyVerifierV1Alpha1{
				ConfigCertificate: "TEST",
			},
		},
	}

	cont, err := container.New(verificationCfg)
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(cont)
	suite.Create(cfg)

	ctest.AssertResource(suite, "0000", func(rule *security.ImageVerificationRule, asrt *assert.Assertions) {
		asrt.Equal("docker.io/*", rule.TypedSpec().ImagePattern)
		asrt.False(rule.TypedSpec().Verify)
		asrt.Nil(rule.TypedSpec().KeylessVerifier)
		asrt.Nil(rule.TypedSpec().PublicKeyVerifier)
	})

	ctest.AssertResource(suite, "0001", func(rule *security.ImageVerificationRule, asrt *assert.Assertions) {
		asrt.Equal("ghcr.io/myorg/*", rule.TypedSpec().ImagePattern)
		asrt.True(rule.TypedSpec().Verify)
		asrt.NotNil(rule.TypedSpec().KeylessVerifier)
		asrt.Equal("https://token.actions.githubusercontent.com", rule.TypedSpec().KeylessVerifier.Issuer)
		asrt.Equal("https://github.com/myorg/.*", rule.TypedSpec().KeylessVerifier.SubjectRegex)
		asrt.Equal("https://rekor.sigstore.dev", rule.TypedSpec().KeylessVerifier.RekorURL)
		asrt.Nil(rule.TypedSpec().PublicKeyVerifier)
	})

	ctest.AssertResource(suite, "0002", func(rule *security.ImageVerificationRule, asrt *assert.Assertions) {
		asrt.Equal("quay.io/*", rule.TypedSpec().ImagePattern)
		asrt.True(rule.TypedSpec().Verify)
		asrt.Nil(rule.TypedSpec().KeylessVerifier)
		asrt.NotNil(rule.TypedSpec().PublicKeyVerifier)
		asrt.Equal("TEST", rule.TypedSpec().PublicKeyVerifier.Certificate)
	})

	suite.Destroy(cfg)

	ctest.AssertNoResource[*security.ImageVerificationRule](suite, "0000")
	ctest.AssertNoResource[*security.ImageVerificationRule](suite, "0001")
	ctest.AssertNoResource[*security.ImageVerificationRule](suite, "0002")
}

func TestImageVerificationConfigSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &ImageVerificationConfigSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(s *ctest.DefaultSuite) {
				s.Require().NoError(s.Runtime().RegisterController(&securityctrl.ImageVerificationConfigController{}))
			},
		},
	})
}
