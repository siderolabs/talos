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
			RuleVerify:       new(true),
			RuleKeylessVerifier: &security.ImageKeylessVerifierV1Alpha1{
				KeylessIssuer:       "https://token.actions.githubusercontent.com",
				KeylessSubjectRegex: "https://github.com/myorg/.*",
			},
		},
		{
			RuleImagePattern: "my-registry/*",
			RuleVerify:       new(true),
			RulePublicKeyVerifier: &security.ImagePublicKeyVerifierV1Alpha1{
				ConfigCertificate: `-----BEGIN CERTIFICATE-----
MII--Sample Value--
-----END CERTIFICATE-----`,
			},
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
				RuleVerify:       new(true),
				RuleKeylessVerifier: &security.ImageKeylessVerifierV1Alpha1{
					KeylessIssuer:       "https://token.actions.githubusercontent.com",
					KeylessSubjectRegex: "https://github.com/myorg/.*",
				},
			},
			{
				RuleImagePattern: "my-registry/*",
				RuleVerify:       new(true),
				RulePublicKeyVerifier: &security.ImagePublicKeyVerifierV1Alpha1{
					ConfigCertificate: `-----BEGIN CERTIFICATE-----
MII--Sample Value--
-----END CERTIFICATE-----`,
				},
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
			RuleVerify:       new(true),
			RuleKeylessVerifier: &security.ImageKeylessVerifierV1Alpha1{
				KeylessIssuer:       "https://accounts.google.com",
				KeylessSubjectRegex: "foo@bork.gserviceaccount.com",
			},
		},
		{
			RuleImagePattern: "**",
			RuleVerify:       new(false),
		},
	}

	rules := cfg.Rules()
	require.Len(t, rules, 2)

	assert.Equal(t, "docker.io/library/*", rules[0].ImagePattern())
	assert.True(t, rules[0].Verify())
	assert.NotNil(t, rules[0].VerifierKeyless())
	assert.Nil(t, rules[0].VerifierPublicKey())
	assert.Equal(t, "https://accounts.google.com", rules[0].VerifierKeyless().Issuer())
	assert.Equal(t, "foo@bork.gserviceaccount.com", rules[0].VerifierKeyless().SubjectRegex())

	assert.Equal(t, "**", rules[1].ImagePattern())
	assert.False(t, rules[1].Verify())
	assert.Nil(t, rules[1].VerifierKeyless())
	assert.Nil(t, rules[1].VerifierPublicKey())
}

func TestImageVerificationConfigValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		cfg func() *security.ImageVerificationConfigV1Alpha1

		expectedErrors string
	}{
		{
			name: "valid config",

			cfg: func() *security.ImageVerificationConfigV1Alpha1 {
				c := security.NewImageVerificationConfigV1Alpha1()
				c.ConfigRules = []security.ImageVerificationRuleV1Alpha1{
					{
						RuleImagePattern: "ghcr.io/*",
						RuleVerify:       new(true),
						RuleKeylessVerifier: &security.ImageKeylessVerifierV1Alpha1{
							KeylessIssuer:       "https://token.actions.githubusercontent.com",
							KeylessSubjectRegex: "https://github.com/myorg/.*",
						},
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
						RuleVerify:       new(true),
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
			name: "valid config with verify false and no verifier",

			cfg: func() *security.ImageVerificationConfigV1Alpha1 {
				c := security.NewImageVerificationConfigV1Alpha1()
				c.ConfigRules = []security.ImageVerificationRuleV1Alpha1{
					{
						RuleImagePattern: "registry.internal.example.com/*",
						RuleVerify:       new(false),
					},
				}

				return c
			},
		},
		{
			name: "rule missing registry and imagePattern",

			cfg: func() *security.ImageVerificationConfigV1Alpha1 {
				c := security.NewImageVerificationConfigV1Alpha1()
				c.ConfigRules = []security.ImageVerificationRuleV1Alpha1{
					{
						RuleVerify: new(false),
					},
				}

				return c
			},

			expectedErrors: "rule 0: at least one of registry or imagePattern must be specified",
		},
		{
			name: "verify true but no verifier",

			cfg: func() *security.ImageVerificationConfigV1Alpha1 {
				c := security.NewImageVerificationConfigV1Alpha1()
				c.ConfigRules = []security.ImageVerificationRuleV1Alpha1{
					{
						RuleImagePattern: "docker.io/*",
						RuleVerify:       new(true),
					},
				}

				return c
			},

			expectedErrors: "rule 0: verifier must be configured if verify=true",
		},
		{
			name: "verifier missing issuer",

			cfg: func() *security.ImageVerificationConfigV1Alpha1 {
				c := security.NewImageVerificationConfigV1Alpha1()
				c.ConfigRules = []security.ImageVerificationRuleV1Alpha1{
					{
						RuleImagePattern: "docker.io/*",
						RuleVerify:       new(true),
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
						RuleVerify:       new(true),
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
			name: "verifier configured but verify is false",

			cfg: func() *security.ImageVerificationConfigV1Alpha1 {
				c := security.NewImageVerificationConfigV1Alpha1()
				c.ConfigRules = []security.ImageVerificationRuleV1Alpha1{
					{
						RuleImagePattern: "docker.io/*",
						RuleVerify:       new(false),
						RuleKeylessVerifier: &security.ImageKeylessVerifierV1Alpha1{
							KeylessIssuer:  "https://accounts.google.com",
							KeylessSubject: "user@example.com",
						},
					},
				}

				return c
			},

			expectedErrors: "rule 0: verifier configured but verify is false",
		},
		{
			name: "multiple errors",

			cfg: func() *security.ImageVerificationConfigV1Alpha1 {
				c := security.NewImageVerificationConfigV1Alpha1()
				c.ConfigRules = []security.ImageVerificationRuleV1Alpha1{
					{
						RuleVerify: new(false),
					},
					{
						RuleImagePattern: "docker.io/*",
						RuleVerify:       new(true),
					},
				}

				return c
			},

			expectedErrors: `rule 0: at least one of registry or imagePattern must be specified
rule 1: verifier must be configured if verify=true`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := test.cfg()

			warnings, err := cfg.Validate(validationMode{})

			if test.expectedErrors == "" {
				require.NoError(t, err)
				assert.Empty(t, warnings)
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
			RuleVerify:       new(false),
		},
		{
			RuleImagePattern: "docker.io/*",
			RuleVerify:       new(true),
			RuleKeylessVerifier: &security.ImageKeylessVerifierV1Alpha1{
				KeylessIssuer:  "https://accounts.google.com",
				KeylessSubject: "user@example.com",
			},
		},
		{
			RuleImagePattern: "**",
			RuleVerify:       new(true),
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
			RuleVerify:       new(true),
			RuleKeylessVerifier: &security.ImageKeylessVerifierV1Alpha1{
				KeylessIssuer:       "https://token.actions.githubusercontent.com",
				KeylessSubjectRegex: "https://github.com/differentorg/.*",
			},
		},
		{
			RuleImagePattern: "**",
			RuleVerify:       new(false),
		},
	}

	require.NoError(t, merge.Merge(baseCfg, overrideCfg))

	require.Len(t, baseCfg.ConfigRules, 3)

	assert.Equal(t, "ghcr.io/siderolabs/talos*", baseCfg.ConfigRules[0].RuleImagePattern)
	assert.True(t, pointer.SafeDeref(baseCfg.ConfigRules[0].RuleVerify))
	assert.Equal(t, "https://token.actions.githubusercontent.com", baseCfg.ConfigRules[0].RuleKeylessVerifier.KeylessIssuer)
	assert.Equal(t, "https://github.com/differentorg/.*", baseCfg.ConfigRules[0].RuleKeylessVerifier.KeylessSubjectRegex)

	assert.Equal(t, "docker.io/*", baseCfg.ConfigRules[1].RuleImagePattern)
	assert.True(t, pointer.SafeDeref(baseCfg.ConfigRules[1].RuleVerify))
	assert.Equal(t, "https://accounts.google.com", baseCfg.ConfigRules[1].RuleKeylessVerifier.KeylessIssuer)
	assert.Equal(t, "user@example.com", baseCfg.ConfigRules[1].RuleKeylessVerifier.KeylessSubject)

	assert.Equal(t, "**", baseCfg.ConfigRules[2].RuleImagePattern)
	assert.False(t, pointer.SafeDeref(baseCfg.ConfigRules[2].RuleVerify))
	assert.Nil(t, baseCfg.ConfigRules[2].RuleKeylessVerifier)
}
