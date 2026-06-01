// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package security_test

import (
	_ "embed"
	"testing"

	"github.com/siderolabs/go-pointer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/merge"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/security"
)

//go:embed testdata/imageverificationconfig.yaml
var expectedImageVerificationConfigDocument []byte

func TestImageVerificationConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := security.NewImageVerificationConfigV1Alpha1()
	cfg.ConfigRules = []security.ImageVerificationRuleV1Alpha1{
		{
			RuleImagePattern: "ghcr.io/*",
			RuleKeylessVerifier: &security.ImageKeylessVerifierV1Alpha1{
				KeylessIssuer:       "https://token.actions.githubusercontent.com",
				KeylessSubjectRegex: "https://github.com/myorg/.*",
			},
		},
		{
			RuleImagePattern: "my-registry/*",
			RulePublicKeyVerifier: &security.ImagePublicKeyVerifierV1Alpha1{
				ConfigCertificate: `-----BEGIN CERTIFICATE-----
MII--Sample Value--
-----END CERTIFICATE-----`,
			},
		},
		{
			RuleImagePattern: "no-verifier/*",
			RuleSkip:         new(true),
		},
		{
			RuleImagePattern: "deny-all/*",
			RuleDeny:         new(true),
		},
	}

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, string(expectedImageVerificationConfigDocument), string(marshaled))
}

func TestImageVerificationConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedImageVerificationConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, &security.ImageVerificationConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       security.ImageVerificationConfigKind,
		},
		ConfigRules: []security.ImageVerificationRuleV1Alpha1{
			{
				RuleImagePattern: "ghcr.io/*",
				RuleKeylessVerifier: &security.ImageKeylessVerifierV1Alpha1{
					KeylessIssuer:       "https://token.actions.githubusercontent.com",
					KeylessSubjectRegex: "https://github.com/myorg/.*",
				},
			},
			{
				RuleImagePattern: "my-registry/*",
				RulePublicKeyVerifier: &security.ImagePublicKeyVerifierV1Alpha1{
					ConfigCertificate: `-----BEGIN CERTIFICATE-----
MII--Sample Value--
-----END CERTIFICATE-----`,
				},
			},
			{
				RuleImagePattern: "no-verifier/*",
				RuleSkip:         new(true),
			},
			{
				RuleImagePattern: "deny-all/*",
				RuleDeny:         new(true),
			},
		},
	}, docs[0])
}

func TestImageVerificationConfigRules(t *testing.T) {
	t.Parallel()

	cfg := security.NewImageVerificationConfigV1Alpha1()
	cfg.ConfigRules = []security.ImageVerificationRuleV1Alpha1{
		{
			RuleImagePattern: "docker.io/library/*",
			RuleKeylessVerifier: &security.ImageKeylessVerifierV1Alpha1{
				KeylessIssuer:       "https://accounts.google.com",
				KeylessSubjectRegex: "foo@bork.gserviceaccount.com",
			},
		},
		{
			RuleImagePattern: "**",
			RuleSkip:         new(true),
		},
	}

	rules := cfg.Rules()
	require.Len(t, rules, 2)

	assert.Equal(t, "docker.io/library/*", rules[0].ImagePattern())
	assert.False(t, rules[0].Skip())
	assert.False(t, rules[0].Deny())
	assert.NotNil(t, rules[0].VerifierKeyless())
	assert.Nil(t, rules[0].VerifierPublicKey())
	assert.Equal(t, "https://accounts.google.com", rules[0].VerifierKeyless().Issuer())
	assert.Equal(t, "foo@bork.gserviceaccount.com", rules[0].VerifierKeyless().SubjectRegex())

	assert.Equal(t, "**", rules[1].ImagePattern())
	assert.True(t, rules[1].Skip())
	assert.False(t, rules[1].Deny())
	assert.Nil(t, rules[1].VerifierKeyless())
	assert.Nil(t, rules[1].VerifierPublicKey())
}

func TestImageVerificationConfigValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		cfg func() *security.ImageVerificationConfigV1Alpha1

		expectedErrors   string
		expectedWarnings []string
	}{
		{
			name: "valid config",

			cfg: func() *security.ImageVerificationConfigV1Alpha1 {
				c := security.NewImageVerificationConfigV1Alpha1()
				c.ConfigRules = []security.ImageVerificationRuleV1Alpha1{
					{
						RuleImagePattern: "ghcr.io/*",
						RuleKeylessVerifier: &security.ImageKeylessVerifierV1Alpha1{
							KeylessIssuer:       "https://token.actions.githubusercontent.com",
							KeylessSubjectRegex: "https://github.com/myorg/.*",
						},
					},
					{
						RuleImagePattern: "localhost:3000/*",
						RuleDeny:         new(true),
					},
				}

				return c
			},
		},
		{
			name: "valid config with subject instead of subjectRegex",

			cfg: func() *security.ImageVerificationConfigV1Alpha1 {
				c := security.NewImageVerificationConfigV1Alpha1()
				c.ConfigRules = []security.ImageVerificationRuleV1Alpha1{
					{
						RuleImagePattern: "docker.io/*",
						RuleKeylessVerifier: &security.ImageKeylessVerifierV1Alpha1{
							KeylessIssuer:  "https://accounts.google.com",
							KeylessSubject: "user@example.com",
						},
					},
				}

				return c
			},
		},
		{
			name: "valid config with skip and no verifier",

			cfg: func() *security.ImageVerificationConfigV1Alpha1 {
				c := security.NewImageVerificationConfigV1Alpha1()
				c.ConfigRules = []security.ImageVerificationRuleV1Alpha1{
					{
						RuleImagePattern: "registry.internal.example.com/*",
						RuleSkip:         new(true),
					},
				}

				return c
			},
		},
		{
			name: "valid config with deny and no verifier",

			cfg: func() *security.ImageVerificationConfigV1Alpha1 {
				c := security.NewImageVerificationConfigV1Alpha1()
				c.ConfigRules = []security.ImageVerificationRuleV1Alpha1{
					{
						RuleImagePattern: "registry.internal.example.com/*",
						RuleDeny:         new(true),
					},
				}

				return c
			},
		},
		{
			name: "rule missing imagePattern",

			cfg: func() *security.ImageVerificationConfigV1Alpha1 {
				c := security.NewImageVerificationConfigV1Alpha1()
				c.ConfigRules = []security.ImageVerificationRuleV1Alpha1{
					{},
				}

				return c
			},

			expectedErrors: "rule 0: imagePattern must be specified\nrule 0: at least one verifier must be configured",
		},
		{
			name: "no verifier",

			cfg: func() *security.ImageVerificationConfigV1Alpha1 {
				c := security.NewImageVerificationConfigV1Alpha1()
				c.ConfigRules = []security.ImageVerificationRuleV1Alpha1{
					{
						RuleImagePattern: "docker.io/*",
					},
				}

				return c
			},

			expectedErrors: "rule 0: at least one verifier must be configured",
		},
		{
			name: "verifier missing issuer",

			cfg: func() *security.ImageVerificationConfigV1Alpha1 {
				c := security.NewImageVerificationConfigV1Alpha1()
				c.ConfigRules = []security.ImageVerificationRuleV1Alpha1{
					{
						RuleImagePattern: "docker.io/*",
						RuleKeylessVerifier: &security.ImageKeylessVerifierV1Alpha1{
							KeylessSubject: "user@example.com",
						},
					},
				}

				return c
			},

			expectedErrors: "rule 0: verifier OIDC issuer must be specified",
		},
		{
			name: "verifier missing subject and subjectRegex",

			cfg: func() *security.ImageVerificationConfigV1Alpha1 {
				c := security.NewImageVerificationConfigV1Alpha1()
				c.ConfigRules = []security.ImageVerificationRuleV1Alpha1{
					{
						RuleImagePattern: "docker.io/*",
						RuleKeylessVerifier: &security.ImageKeylessVerifierV1Alpha1{
							KeylessIssuer: "https://accounts.google.com",
						},
					},
				}

				return c
			},

			expectedErrors: "rule 0: verifier subject or subjectRegex must be specified",
		},
		{
			name: "rule 0: verifiers cannot be configured if skip or deny is true",

			cfg: func() *security.ImageVerificationConfigV1Alpha1 {
				c := security.NewImageVerificationConfigV1Alpha1()
				c.ConfigRules = []security.ImageVerificationRuleV1Alpha1{
					{
						RuleImagePattern: "docker.io/*",
						RuleSkip:         new(true),
						RuleKeylessVerifier: &security.ImageKeylessVerifierV1Alpha1{
							KeylessIssuer:  "https://accounts.google.com",
							KeylessSubject: "user@example.com",
						},
					},
				}

				return c
			},

			expectedErrors: "rule 0: verifiers cannot be configured if skip or deny is true",
		},
		{
			name: "multiple errors",

			cfg: func() *security.ImageVerificationConfigV1Alpha1 {
				c := security.NewImageVerificationConfigV1Alpha1()
				c.ConfigRules = []security.ImageVerificationRuleV1Alpha1{
					{
						RuleSkip: new(true),
					},
					{
						RuleImagePattern: "docker.io/*",
					},
				}

				return c
			},

			expectedErrors: "rule 0: imagePattern must be specified\nrule 1: at least one verifier must be configured",
		},
		{
			name: "warnings for in imagePattern",

			cfg: func() *security.ImageVerificationConfigV1Alpha1 {
				c := security.NewImageVerificationConfigV1Alpha1()
				c.ConfigRules = []security.ImageVerificationRuleV1Alpha1{
					{
						RuleImagePattern: "localhost:3000/*/:latest",
						RuleDeny:         new(true),
					},
					{
						RuleImagePattern: "docker.io/my-image@*",
						RuleSkip:         new(true),
					},
					{
						RuleImagePattern: "nginx",
						RuleSkip:         new(true),
					},
				}

				return c
			},

			expectedWarnings: []string{
				"rule 0: imagePattern contains ':' but matching only applies to the image registry and repository, not the tag or digest",
				"rule 1: imagePattern contains '@' but matching only applies to the image registry and repository, not the tag or digest",
				"rule 2: imagePattern does not contain a '/', image references like 'nginx' are matched as 'docker.io/nginx' (normalized)",
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := test.cfg()

			warnings, err := cfg.Validate(validationMode{})
			assert.Equal(t, test.expectedWarnings, warnings)

			if test.expectedErrors == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.EqualError(t, err, test.expectedErrors)
			}
		})
	}
}

type validationMode struct{}

func (validationMode) String() string {
	return ""
}

func (validationMode) RequiresInstall() bool {
	return false
}

func (validationMode) InContainer() bool {
	return false
}

func TestImageVerificationConfigMerge(t *testing.T) {
	t.Parallel()

	baseCfg := security.NewImageVerificationConfigV1Alpha1()
	baseCfg.ConfigRules = []security.ImageVerificationRuleV1Alpha1{
		{
			RuleImagePattern: "ghcr.io/siderolabs/talos*",
			RuleSkip:         new(true),
		},
		{
			RuleImagePattern: "docker.io/*",
			RuleKeylessVerifier: &security.ImageKeylessVerifierV1Alpha1{
				KeylessIssuer:  "https://accounts.google.com",
				KeylessSubject: "user@example.com",
			},
		},
		{
			RuleImagePattern: "**",
			RuleKeylessVerifier: &security.ImageKeylessVerifierV1Alpha1{
				KeylessIssuer:       "https://token.actions.githubusercontent.com",
				KeylessSubjectRegex: "https://github.com/fallbackorg/.*",
			},
		},
	}

	overrideCfg := security.NewImageVerificationConfigV1Alpha1()
	overrideCfg.ConfigRules = []security.ImageVerificationRuleV1Alpha1{
		{
			RuleImagePattern: "ghcr.io/siderolabs/talos*",
			RuleKeylessVerifier: &security.ImageKeylessVerifierV1Alpha1{
				KeylessIssuer:       "https://token.actions.githubusercontent.com",
				KeylessSubjectRegex: "https://github.com/differentorg/.*",
			},
		},
		{
			RuleImagePattern: "**",
			RuleSkip:         new(true),
		},
	}

	require.NoError(t, merge.Merge(baseCfg, overrideCfg))

	require.Len(t, baseCfg.ConfigRules, 3)

	assert.Equal(t, "ghcr.io/siderolabs/talos*", baseCfg.ConfigRules[0].RuleImagePattern)
	assert.False(t, pointer.SafeDeref(baseCfg.ConfigRules[0].RuleSkip))
	assert.Equal(t, "https://token.actions.githubusercontent.com", baseCfg.ConfigRules[0].RuleKeylessVerifier.KeylessIssuer)
	assert.Equal(t, "https://github.com/differentorg/.*", baseCfg.ConfigRules[0].RuleKeylessVerifier.KeylessSubjectRegex)

	assert.Equal(t, "docker.io/*", baseCfg.ConfigRules[1].RuleImagePattern)
	assert.False(t, pointer.SafeDeref(baseCfg.ConfigRules[1].RuleSkip))
	assert.Equal(t, "https://accounts.google.com", baseCfg.ConfigRules[1].RuleKeylessVerifier.KeylessIssuer)
	assert.Equal(t, "user@example.com", baseCfg.ConfigRules[1].RuleKeylessVerifier.KeylessSubject)

	assert.Equal(t, "**", baseCfg.ConfigRules[2].RuleImagePattern)
	assert.True(t, pointer.SafeDeref(baseCfg.ConfigRules[2].RuleSkip))
	assert.Nil(t, baseCfg.ConfigRules[2].RuleKeylessVerifier)
}
