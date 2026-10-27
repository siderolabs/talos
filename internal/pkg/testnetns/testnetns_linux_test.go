// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package testnetns_test

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/testnetns"
)

const (
	modeEnvironment  = "TALOS_TEST_NETNS_MODE"
	childEnvironment = "TALOS_TEST_NETNS_CHILD"
)

func TestMain(m *testing.M) {
	if os.Getenv(modeEnvironment) != "" {
		os.Exit(testnetns.Run(m.Run))
	}

	os.Exit(m.Run())
}

func TestChild(t *testing.T) {
	mode := os.Getenv(modeEnvironment)
	if mode == "" {
		t.Skip("subprocess fixture")
	}

	switch mode {
	case "pass":
		input, err := io.ReadAll(os.Stdin)
		require.NoError(t, err)
		require.Equal(t, "forwarded stdin", string(input))
		fmt.Fprintln(os.Stdout, "forwarded stdout")
		fmt.Fprintln(os.Stderr, "forwarded stderr")
	case "fail":
		t.Fatal("intentional failure")
	case "skip":
		t.Skip("intentional skip")
	case "panic":
		panic("intentional panic")
	}
}

func TestRun(t *testing.T) {
	for _, tt := range []struct {
		mode   string
		code   int
		output string
	}{
		{"pass", 0, "--- PASS: TestChild"},
		{"fail", 1, "intentional failure"},
		{"skip", 0, "--- SKIP: TestChild"},
		{"panic", 2, "panic: intentional panic"},
	} {
		t.Run(tt.mode, func(t *testing.T) {
			cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestChild$", "-test.v")

			cmd.Env = append(os.Environ(), modeEnvironment+"="+tt.mode, childEnvironment+"=")
			cmd.Stdin = strings.NewReader("forwarded stdin")
			output, err := cmd.CombinedOutput()

			if tt.code == 0 {
				require.NoError(t, err, "%s", output)
			} else {
				var exitErr *exec.ExitError
				require.ErrorAs(t, err, &exitErr, "%s", output)
				require.Equal(t, tt.code, exitErr.ExitCode(), "%s", output)
			}

			require.Contains(t, string(output), tt.output)

			if tt.mode == "pass" {
				require.Contains(t, string(output), "forwarded stdout")
				require.Contains(t, string(output), "forwarded stderr")
			}
		})
	}
}

func TestStartFailure(t *testing.T) {
	t.Setenv(childEnvironment, "")

	original := os.Args
	os.Args = []string{"/nonexistent/testnetns-binary"}

	t.Cleanup(func() { os.Args = original })

	require.Equal(t, 1, testnetns.Run(func() int {
		t.Fatal("must not run without namespace isolation")

		return 0
	}))
}
