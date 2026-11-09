// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package imager_test

import (
	"context"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/imager"
	"github.com/siderolabs/talos/pkg/imager/profile"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/meta"
	"github.com/siderolabs/talos/pkg/reporter"
)

const (
	secretToken           = "unique-token-secret"
	secretMachineToken    = "machine-token-secret"
	secretClusterSecret   = "cluster-secret-value"
	embeddedMachineConfig = `version: v1alpha1
machine:
  type: worker
  token: ` + secretMachineToken + `
cluster:
  secret: ` + secretClusterSecret + `
  controlPlane:
    endpoint: https://example.com:6443
`
)

var metaEnvRe = regexp.MustCompile(constants.MetaValuesEnvVar + `=(\S+)`)

// recordingReporter returns a reporter which records every message it is given.
func recordingReporter() (*reporter.Reporter, func() string) {
	var (
		mu    sync.Mutex
		lines []string
	)

	rep := reporter.New(
		reporter.WithOutputMode(reporter.OutputModePlain),
		reporter.WithLineFilter(func(line string) string {
			mu.Lock()
			defer mu.Unlock()

			lines = append(lines, line)

			return line
		}),
	)

	return rep, func() string {
		mu.Lock()
		defer mu.Unlock()

		return strings.Join(lines, "\n")
	}
}

// decodeMetaFromCmdline returns the META values passed on the kernel command line.
//
// They are base64-encoded (and gzipped when long), so a secret can't be found in the command line as text
// whether it is redacted or not.
func decodeMetaFromCmdline(t *testing.T, cmdline string) meta.Values {
	t.Helper()

	match := metaEnvRe.FindStringSubmatch(cmdline)
	require.NotNil(t, match, "no META values in %q", cmdline)

	values, err := meta.DecodeValues(match[1])
	require.NoError(t, err)

	return values
}

func TestImagerRedactsOutput(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		// the length of a non-sensitive META value, long enough values get the META gzipped
		padding int
		quiet   bool
	}{
		{
			name: "plain META",
		},
		{
			name:    "compressed META",
			padding: 512,
		},
		{
			name:  "quiet",
			quiet: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			t.Cleanup(cancel)

			imgr, err := imager.New(profile.Profile{
				BaseProfileName: "metal",
				Arch:            "amd64",
				Output: profile.Output{
					Kind:      profile.OutKindCmdline,
					OutFormat: profile.OutFormatRaw,
				},
				Customization: profile.CustomizationProfile{
					MetaContents: meta.Values{
						{Key: meta.UniqueMachineToken, Value: secretToken},
						{Key: meta.UserReserved1, Value: "user-data" + strings.Repeat("x", test.padding)},
					},
					EmbeddedMachineConfiguration: embeddedMachineConfig,
				},
			}, imager.WithQuiet(test.quiet))
			require.NoError(t, err)

			rep, output := recordingReporter()

			outputPath, err := imgr.Execute(ctx, t.TempDir(), rep)
			require.NoError(t, err)

			// the image carries the secrets as they are, only the output is redacted
			out, err := os.ReadFile(outputPath)
			require.NoError(t, err)

			assert.Contains(t, decodeMetaFromCmdline(t, string(out)), meta.Value{Key: meta.UniqueMachineToken, Value: secretToken})

			reported := output()

			for _, secret := range []string{secretToken, secretMachineToken, secretClusterSecret} {
				assert.NotContains(t, reported, secret)
			}

			if test.quiet {
				assert.NotContains(t, reported, "profile ready:")
				assert.NotContains(t, reported, "kernel command line:")
				assert.NotContains(t, reported, constants.MetaValuesEnvVar)

				return
			}

			assert.Contains(t, reported, "profile ready:")
			assert.Contains(t, reported, "https://example.com:6443")

			assert.Equal(t, meta.Values{
				{Key: meta.UniqueMachineToken, Value: constants.Redacted},
				{Key: meta.UserReserved1, Value: "user-data" + strings.Repeat("x", test.padding)},
			}, decodeMetaFromCmdline(t, reported))
		})
	}
}
